package sql

import (
	"fmt"
	"strings"
	"unicode"
)

/*
query_scope.go
----------------------------------------------------------------------
为只读查询增加应用层命名空间限制。

词法扫描会跳过字符串和注释，并识别 FROM、JOIN、子查询及逗号连接中的表引用：
MySQL 限制在配置数据库，PostgreSQL/Oracle 限制在配置 schema，MSSQL 限制在配置数据库并禁止 linked server。
该校验属于纵深防护，不能替代目标数据库账号的最小权限配置。
*/

// sqlScopeToken 是范围校验使用的轻量词法单元。
// 字符串和注释会在词法阶段被跳过，避免把其中的示例表名误判为真实引用。
type sqlScopeToken struct {
	text       string
	identifier bool
}

var fromClauseEndKeywords = map[string]struct{}{
	"where": {}, "group": {}, "having": {}, "order": {}, "limit": {},
	"offset": {}, "fetch": {}, "union": {}, "except": {}, "intersect": {},
	"for": {}, "connect": {}, "start": {}, "model": {}, "window": {},
}

// validateQueryScope 将连接配置中的库/schema 作为查询范围，拒绝显式跨范围表引用。
// 这是一层纵深防护；数据库账号自身仍必须遵循最小权限原则。
func validateQueryScope(record DBConnectionRecord, sqlText string) error {
	tokens, err := tokenizeSQLScope(sqlText)
	if err != nil {
		return err
	}

	fromActive := make(map[int]bool)
	expectRelation := make(map[int]bool)
	depth := 0

	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		lower := strings.ToLower(token.text)

		if token.text == "(" {
			if expectRelation[depth] {
				expectRelation[depth] = false
			}
			depth++
			continue
		}
		if token.text == ")" {
			delete(fromActive, depth)
			delete(expectRelation, depth)
			if depth > 0 {
				depth--
			}
			continue
		}

		if token.identifier {
			if _, closesFrom := fromClauseEndKeywords[lower]; closesFrom {
				fromActive[depth] = false
				expectRelation[depth] = false
				continue
			}
			if lower == "from" {
				fromActive[depth] = true
				expectRelation[depth] = true
				continue
			}
			if lower == "join" {
				expectRelation[depth] = true
				continue
			}
		}

		if token.text == "," && fromActive[depth] {
			expectRelation[depth] = true
			continue
		}
		if !expectRelation[depth] {
			continue
		}

		if token.identifier && (lower == "only" || lower == "lateral") {
			continue
		}
		if !token.identifier {
			expectRelation[depth] = false
			continue
		}

		parts, nextIndex := readQualifiedRelation(tokens, index)
		if err := validateRelationScope(record, parts); err != nil {
			return err
		}
		if nextIndex < len(tokens) && tokens[nextIndex].text == "@" {
			return fmt.Errorf("不允许通过数据库链接访问其他范围: %s", strings.Join(parts, "."))
		}

		expectRelation[depth] = false
		index = nextIndex - 1
	}

	return nil
}

func readQualifiedRelation(tokens []sqlScopeToken, start int) ([]string, int) {
	parts := []string{tokens[start].text}
	index := start + 1

	for index < len(tokens) && tokens[index].text == "." {
		index++
		if index < len(tokens) && tokens[index].text == "." {
			parts = append(parts, "")
			continue
		}
		if index >= len(tokens) || !tokens[index].identifier {
			break
		}
		parts = append(parts, tokens[index].text)
		index++
	}

	return parts, index
}

func validateRelationScope(record DBConnectionRecord, parts []string) error {
	if len(parts) <= 1 {
		return nil
	}

	dbType := strings.ToLower(strings.TrimSpace(record.DBType))
	relation := strings.Join(parts, ".")

	switch dbType {
	case "mysql":
		if len(parts) != 2 || !equalSQLName(parts[0], record.DatabaseName) {
			return fmt.Errorf("当前连接仅允许查询 MySQL 数据库 %q，不允许访问 %q", record.DatabaseName, relation)
		}
	case "postgres":
		schemaName := strings.TrimSpace(record.ServiceName)
		if schemaName == "" {
			schemaName = "public"
		}
		if len(parts) != 2 || !equalSQLName(parts[0], schemaName) {
			return fmt.Errorf("当前连接仅允许查询 PostgreSQL schema %q，不允许访问 %q", schemaName, relation)
		}
	case "oracle":
		schemaName := strings.TrimSpace(record.DatabaseName)
		if schemaName == "" {
			schemaName = strings.TrimSpace(record.Username)
		}
		if len(parts) != 2 || !equalSQLName(parts[0], schemaName) {
			return fmt.Errorf("当前连接仅允许查询 Oracle schema %q，不允许访问 %q", schemaName, relation)
		}
	case "mssql":
		// 两段名称是 schema.table，仍位于当前数据库；三段名称必须指向配置数据库。
		if len(parts) == 2 {
			return nil
		}
		if len(parts) != 3 || !equalSQLName(parts[0], record.DatabaseName) {
			return fmt.Errorf("当前连接仅允许查询 MSSQL 数据库 %q，不允许访问 %q", record.DatabaseName, relation)
		}
	}

	return nil
}

func equalSQLName(left string, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func tokenizeSQLScope(sqlText string) ([]sqlScopeToken, error) {
	tokens := make([]sqlScopeToken, 0)

	for index := 0; index < len(sqlText); {
		current := sqlText[index]

		if unicode.IsSpace(rune(current)) {
			index++
			continue
		}
		if current == '-' && index+1 < len(sqlText) && sqlText[index+1] == '-' {
			index += 2
			for index < len(sqlText) && sqlText[index] != '\n' {
				index++
			}
			continue
		}
		if current == '/' && index+1 < len(sqlText) && sqlText[index+1] == '*' {
			end := strings.Index(sqlText[index+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("SQL 块注释未闭合")
			}
			index += end + 4
			continue
		}
		if current == '\'' {
			next, err := skipQuotedSQL(sqlText, index, '\'', '\'')
			if err != nil {
				return nil, err
			}
			index = next
			continue
		}
		if current == '"' || current == '`' {
			next, value, err := readQuotedIdentifier(sqlText, index, current, current)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, sqlScopeToken{text: value, identifier: true})
			index = next
			continue
		}
		if current == '[' {
			next, value, err := readQuotedIdentifier(sqlText, index, '[', ']')
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, sqlScopeToken{text: value, identifier: true})
			index = next
			continue
		}
		if isSQLIdentifierStart(current) {
			start := index
			index++
			for index < len(sqlText) && isSQLIdentifierPart(sqlText[index]) {
				index++
			}
			tokens = append(tokens, sqlScopeToken{text: sqlText[start:index], identifier: true})
			continue
		}

		tokens = append(tokens, sqlScopeToken{text: string(current)})
		index++
	}

	return tokens, nil
}

func skipQuotedSQL(sqlText string, start int, quote byte, escapeQuote byte) (int, error) {
	index := start + 1
	for index < len(sqlText) {
		if sqlText[index] != quote {
			index++
			continue
		}
		if index+1 < len(sqlText) && sqlText[index+1] == escapeQuote {
			index += 2
			continue
		}
		return index + 1, nil
	}
	return 0, fmt.Errorf("SQL 字符串未闭合")
}

func readQuotedIdentifier(sqlText string, start int, open byte, close byte) (int, string, error) {
	var value strings.Builder
	index := start + 1

	for index < len(sqlText) {
		if sqlText[index] != close {
			value.WriteByte(sqlText[index])
			index++
			continue
		}
		if index+1 < len(sqlText) && sqlText[index+1] == close {
			value.WriteByte(close)
			index += 2
			continue
		}
		return index + 1, value.String(), nil
	}

	return 0, "", fmt.Errorf("SQL 标识符未闭合: %c", open)
}

func isSQLIdentifierStart(value byte) bool {
	return value == '_' || value == '$' || value == '#' ||
		value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
		value >= 0x80
}

func isSQLIdentifierPart(value byte) bool {
	return isSQLIdentifierStart(value) || value >= '0' && value <= '9'
}
