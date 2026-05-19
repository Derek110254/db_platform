package sql

// checker.go 是 SQL 风险检查模块的核心实现。
// 它完成的流程包括：
// 1. 按语句切分原始 SQL，并保留起始行号，方便定位问题；
// 2. 进行基础语法层面的静态检查，例如括号、引号、语句结尾等；
// 3. 根据预设风险规则识别高危 SQL，用于提示全表更新、全表删除、危险 DDL/DML 等风险。
// 这里的检查是“静态文本分析”，不会真正连接数据库执行 SQL。

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DBType 表示当前支持的数据库方言类型。
// 目前仅支持 MySQL 与 Oracle，两者会在部分语法规则和风险规则上有所区别。
type DBType string

const (
	MySQL  DBType = "mysql"
	Oracle DBType = "oracle"
)

// Severity 表示风险严重程度，同时也会参与最终风险分值与风险等级计算。
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// SQLStatement 表示切分后的单条 SQL 语句。
// 除了原始文本外，还保留了归一化文本、起始行号、是否以分号结束、前置注释等上下文信息。
type SQLStatement struct {
	No          int      `json:"no"`
	StartLine   int      `json:"startLine"`
	Raw         string   `json:"raw"`
	Normalized  string   `json:"normalized"`
	Terminated  bool     `json:"terminated"`
	LeadingNote []string `json:"leadingNote,omitempty"`
}

// SyntaxIssue 表示一条语法类问题。
// line 用于前端定位到原始 SQL 的大致位置，sql 字段用于展示问题片段。
type SyntaxIssue struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
	SQL     string `json:"sql"`
}

type SyntaxCheckResult struct {
	Valid  bool          `json:"valid"`
	Errors []SyntaxIssue `json:"errors"`
}

// RiskRule 描述一条高风险 SQL 规则。
// 规则既支持纯正则匹配，也支持 CustomCheck 这种更灵活的自定义判定函数。
type RiskRule struct {
	ID          string
	Name        string
	Description string
	Severity    Severity
	Databases   []DBType
	Pattern     *regexp.Regexp
	CustomCheck func(sql string) bool
	Suggestion  string
}

type MatchResult struct {
	StatementNo int      `json:"statementNo"`
	Line        int      `json:"line"`
	RuleID      string   `json:"ruleID"`
	Name        string   `json:"name"`
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
	Suggestion  string   `json:"suggestion"`
	SQL         string   `json:"sql"`
}

type CheckResponse struct {
	OK            bool          `json:"ok"`
	DBType        string        `json:"dbType"`
	SyntaxMessage string        `json:"syntaxMessage"`
	SyntaxErrors  []SyntaxIssue `json:"syntaxErrors"`
	RiskLevel     Severity      `json:"riskLevel"`
	RiskScore     int           `json:"riskScore"`
	RiskMessage   string        `json:"riskMessage"`
	RiskItems     []MatchResult `json:"riskItems"`
}

// CheckSQL 是 SQL 风险检查模块的统一入口。
// 它先校验数据库类型，再依次完成语句切分、语法检查、风险识别，
// 最后把前端所需的汇总信息一次性返回。
func CheckSQL(dbType string, rawSQL string) CheckResponse {
	db := DBType(strings.ToLower(strings.TrimSpace(dbType)))

	if db != MySQL && db != Oracle {
		return CheckResponse{
			OK:            false,
			DBType:        dbType,
			SyntaxMessage: "暂时只支持 mysql 和 oracle",
			SyntaxErrors: []SyntaxIssue{
				{
					Line:    1,
					Message: "暂时只支持 mysql 和 oracle",
				},
			},
			RiskLevel:   SeverityLow,
			RiskScore:   0,
			RiskMessage: "未执行风险检测",
			RiskItems:   []MatchResult{},
		}
	}

	stmts := splitStatementsWithLines(rawSQL)
	syntaxRes := checkSQLSyntaxWithLines(db, rawSQL, stmts)

	rules := buildRules()
	riskItems, riskScore, riskLevel := detectRisksByStatement(db, stmts, rules)

	ok := syntaxRes.Valid

	syntaxMsg := "SQL语法检查通过"
	if !syntaxRes.Valid {
		syntaxMsg = "发现 " + strconv.Itoa(len(syntaxRes.Errors)) + " 条语法问题"
	}

	riskMsg := "未发现高风险SQL"
	if len(riskItems) > 0 {
		riskMsg = "检测到 " + strconv.Itoa(len(riskItems)) + " 个高风险项"
	}

	return CheckResponse{
		OK:            ok,
		DBType:        string(db),
		SyntaxMessage: syntaxMsg,
		SyntaxErrors:  syntaxRes.Errors,
		RiskLevel:     riskLevel,
		RiskScore:     riskScore,
		RiskMessage:   riskMsg,
		RiskItems:     riskItems,
	}
}

// splitStatementsWithLines 将整段 SQL 按“语句”切分，同时尽量保留原始行号信息。
// 这里专门处理了单引号、行注释、空行等情况，避免简单按分号切分导致误判。
func splitStatementsWithLines(raw string) []SQLStatement {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")

	stmts := make([]SQLStatement, 0)
	var current []string
	var notes []string

	inQuote := false
	startLine := 0
	no := 1

	flushCurrent := func(terminated bool) {
		if len(current) == 0 {
			return
		}
		rawStmt := strings.TrimSpace(strings.Join(current, "\n"))
		if rawStmt == "" {
			current = nil
			startLine = 0
			return
		}
		stmts = append(stmts, SQLStatement{
			No:          no,
			StartLine:   startLine,
			Raw:         rawStmt,
			Normalized:  normalizeSQL(rawStmt),
			Terminated:  terminated,
			LeadingNote: append([]string{}, notes...),
		})
		no++
		current = nil
		notes = nil
		startLine = 0
		inQuote = false
	}

	for i, line := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)

		if len(current) == 0 && trimmed == "" {
			continue
		}

		if len(current) == 0 && isWholeLineComment(trimmed) {
			notes = append(notes, line)
			continue
		}

		// 关键修复：
		// 如果当前语句因为单引号未闭合进入了异常状态，
		// 但下一行看起来已经是一个全新的 SQL 起始，则强制切分恢复。
		if len(current) > 0 && inQuote && looksLikeStatementStart(trimmed) {
			flushCurrent(false)
		}

		if len(current) == 0 {
			startLine = lineNo
		}

		current = append(current, line)

		runes := []rune(line)
		terminated := false

		for j := 0; j < len(runes); j++ {
			r := runes[j]

			if !inQuote {
				if j+1 < len(runes) && runes[j] == '-' && runes[j+1] == '-' {
					break
				}
				if runes[j] == '#' {
					break
				}
			}

			if r == '\'' {
				if j+1 < len(runes) && runes[j+1] == '\'' {
					j++
					continue
				}
				inQuote = !inQuote
				continue
			}

			if !inQuote && r == ';' {
				terminated = true
			}
		}

		if terminated {
			flushCurrent(true)
		}
	}

	if len(current) > 0 {
		flushCurrent(false)
	}

	return stmts
}

// normalizeSQL 对 SQL 文本做归一化处理。
// 主要目标是去掉注释、压缩多余空白、统一大小写分析基础，以便后续规则匹配更稳定。
func normalizeSQL(sql string) string {
	s := strings.ReplaceAll(sql, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\t", " ")

	lines := strings.Split(s, "\n")
	cleaned := make([]string, 0, len(lines))
	inBlockComment := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlockComment && isWholeLineComment(trimmed) {
			continue
		}

		runes := []rune(line)
		var out []rune
		inQuote := false

		for i := 0; i < len(runes); i++ {
			r := runes[i]

			if !inQuote && !inBlockComment && i+1 < len(runes) && runes[i] == '/' && runes[i+1] == '*' {
				inBlockComment = true
				i++
				continue
			}
			if inBlockComment {
				if i+1 < len(runes) && runes[i] == '*' && runes[i+1] == '/' {
					inBlockComment = false
					i++
				}
				continue
			}

			if r == '\'' {
				if i+1 < len(runes) && runes[i+1] == '\'' {
					out = append(out, r, runes[i+1])
					i++
					continue
				}
				inQuote = !inQuote
				out = append(out, r)
				continue
			}

			if !inQuote {
				if i+1 < len(runes) && runes[i] == '-' && runes[i+1] == '-' {
					break
				}
				if runes[i] == '#' {
					break
				}
			}

			out = append(out, r)
		}

		cleaned = append(cleaned, string(out))
	}

	s = strings.Join(cleaned, " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(strings.ToUpper(s))
}

// checkSQLSyntaxWithLines 执行语法层面的静态检查。
// 它不会做数据库级解析，而是基于文本特征快速识别常见问题，适合作为上线前预检。
func checkSQLSyntaxWithLines(db DBType, rawSQL string, stmts []SQLStatement) SyntaxCheckResult {
	issues := make([]SyntaxIssue, 0)

	if strings.TrimSpace(rawSQL) == "" {
		return SyntaxCheckResult{
			Valid: false,
			Errors: []SyntaxIssue{
				{
					Line:    1,
					Message: "SQL不能为空",
				},
			},
		}
	}

	issues = append(issues, findQuoteIssues(rawSQL)...)

	for _, stmt := range stmts {
		sql := stmt.Normalized
		if sql == "" {
			continue
		}

		rawTrimmed := strings.TrimSpace(stmt.Raw)

		if !isBalanced(stmt.Raw, '(', ')') {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "括号不匹配",
				SQL:     rawTrimmed,
			})
			continue
		}

		if !hasAnyPrefix(sql,
			"SELECT", "UPDATE", "DELETE", "INSERT", "ALTER", "DROP", "TRUNCATE",
			"CREATE", "WITH", "MERGE", "BEGIN", "DECLARE", "USE", "SET", "COMMIT",
			"START", "SHOW", "DESC", "DESCRIBE", "COMMENT",
			"LOAD", "CALL", "EXPLAIN", "GRANT", "REVOKE", "RENAME",
			"ANALYZE", "OPTIMIZE", "REPLACE",
		) {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "未识别的SQL起始关键字",
				SQL:     rawTrimmed,
			})
			continue
		}

		if regexp.MustCompile(`^SELECT\s+FROM\b`).MatchString(sql) {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "SELECT语句缺少查询列",
				SQL:     rawTrimmed,
			})
			continue
		}

		if strings.HasPrefix(sql, "SELECT") && !strings.Contains(sql, " FROM ") {
			if !(db == MySQL && isMySQLSelectWithoutFromAllowed(sql)) {
				issues = append(issues, SyntaxIssue{
					Line:    stmt.StartLine,
					Message: "SELECT语句缺少FROM",
					SQL:     rawTrimmed,
				})
			}
		}

		if strings.HasPrefix(sql, "DELETE") && !regexp.MustCompile(`\bDELETE\s+FROM\b`).MatchString(sql) {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "DELETE语句缺少FROM",
				SQL:     rawTrimmed,
			})
			continue
		}

		if regexp.MustCompile(`^DELETE\s+FROM\s*;?$`).MatchString(sql) {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "DELETE语句缺少表名",
				SQL:     rawTrimmed,
			})
			continue
		}

		if regexp.MustCompile(`^UPDATE\s+.+\s+WHERE\b.+\bSET\b`).MatchString(sql) {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "UPDATE语句中WHERE位置错误，应先SET后WHERE",
				SQL:     rawTrimmed,
			})
			continue
		}

		if strings.HasPrefix(sql, "UPDATE") && !strings.Contains(sql, " SET ") {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "UPDATE语句缺少SET",
				SQL:     rawTrimmed,
			})
			continue
		}

		if strings.HasPrefix(sql, "INSERT") && !regexp.MustCompile(`\bINSERT\s+INTO\b`).MatchString(sql) {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "INSERT语句缺少INTO",
				SQL:     rawTrimmed,
			})
			continue
		}

		if regexp.MustCompile(`^INSERT\s+INTO\s+[A-Z0-9_.$"]+\s+[A-Z0-9_.$"]+\s*,`).MatchString(sql) {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "INSERT字段列表缺少括号，应写为 INSERT INTO table (col1, col2) VALUES ...",
				SQL:     rawTrimmed,
			})
			continue
		}

		if db == Oracle && strings.Contains(sql, " LIMIT ") {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "Oracle不支持LIMIT语法",
				SQL:     rawTrimmed,
			})
			continue
		}

		if db == MySQL && regexp.MustCompile(`\bEXECUTE\s+IMMEDIATE\b`).MatchString(sql) {
			issues = append(issues, SyntaxIssue{
				Line:    stmt.StartLine,
				Message: "MySQL中通常不使用EXECUTE IMMEDIATE",
				SQL:     rawTrimmed,
			})
			continue
		}
	}

	merged := mergeSyntaxIssues(issues)
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Line == merged[j].Line {
			if merged[i].Message == merged[j].Message {
				return merged[i].SQL < merged[j].SQL
			}
			return merged[i].Message < merged[j].Message
		}
		return merged[i].Line < merged[j].Line
	})

	return SyntaxCheckResult{
		Valid:  len(merged) == 0,
		Errors: merged,
	}
}

func isMySQLSelectWithoutFromAllowed(sql string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^SELECT\s+SLEEP\s*\(`),
		regexp.MustCompile(`^SELECT\s+BENCHMARK\s*\(`),
		regexp.MustCompile(`^SELECT\s+NOW\s*\(`),
		regexp.MustCompile(`^SELECT\s+CURDATE\s*\(`),
		regexp.MustCompile(`^SELECT\s+CURTIME\s*\(`),
		regexp.MustCompile(`^SELECT\s+VERSION\s*\(`),
		regexp.MustCompile(`^SELECT\s+UUID\s*\(`),
		regexp.MustCompile(`^SELECT\s+\d+(\s*,\s*\d+)*\s*;?$`),
		regexp.MustCompile(`^SELECT\s+'.*'\s*;?$`),
	}
	for _, p := range patterns {
		if p.MatchString(sql) {
			return true
		}
	}
	return false
}

func findQuoteIssues(raw string) []SyntaxIssue {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	issues := make([]SyntaxIssue, 0)

	inQuote := false
	startLine := 0
	startSQL := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 关键修复：
		// 如果引号还没闭合，但下一行看起来是新语句起始，
		// 则认为上一条语句单引号不匹配，到此结束。
		if inQuote && looksLikeStatementStart(trimmed) {
			issues = append(issues, SyntaxIssue{
				Line:    startLine,
				Message: "单引号不匹配",
				SQL:     startSQL,
			})
			inQuote = false
			startLine = 0
			startSQL = ""
		}

		if !inQuote && isWholeLineComment(trimmed) {
			continue
		}

		runes := []rune(line)
		for j := 0; j < len(runes); j++ {
			r := runes[j]

			if !inQuote {
				if j+1 < len(runes) && runes[j] == '-' && runes[j+1] == '-' {
					break
				}
				if runes[j] == '#' {
					break
				}
			}

			if r == '\'' {
				if j+1 < len(runes) && runes[j+1] == '\'' {
					j++
					continue
				}
				if !inQuote {
					inQuote = true
					startLine = i + 1
					startSQL = trimmed
				} else {
					inQuote = false
					startLine = 0
					startSQL = ""
				}
			}
		}
	}

	if inQuote {
		issues = append(issues, SyntaxIssue{
			Line:    startLine,
			Message: "单引号不匹配",
			SQL:     startSQL,
		})
	}

	return issues
}

func looksLikeStatementStart(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	u := strings.ToUpper(trimmed)
	keywords := []string{
		"SELECT", "UPDATE", "DELETE", "INSERT", "CREATE", "ALTER", "DROP",
		"TRUNCATE", "USE", "SET", "COMMIT", "BEGIN", "DECLARE", "MERGE",
		"WITH", "SHOW", "DESC", "DESCRIBE", "COMMENT",
		"LOAD", "CALL", "EXPLAIN", "GRANT", "REVOKE", "RENAME",
		"ANALYZE", "OPTIMIZE", "REPLACE",
	}
	for _, kw := range keywords {
		if u == kw || strings.HasPrefix(u, kw+" ") || strings.HasPrefix(u, kw+"\t") || strings.HasPrefix(u, kw+"(") {
			return true
		}
	}
	return false
}

func isWholeLineComment(trimmed string) bool {
	return strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "#")
}

func mergeSyntaxIssues(in []SyntaxIssue) []SyntaxIssue {
	seen := make(map[string]bool)
	out := make([]SyntaxIssue, 0, len(in))
	for _, item := range in {
		key := strconv.Itoa(item.Line) + "|" + item.Message + "|" + item.SQL
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func isBalanced(s string, left, right rune) bool {
	count := 0
	inQuote := false
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if r == '\'' {
			if i+1 < len(runes) && runes[i+1] == '\'' {
				i++
				continue
			}
			inQuote = !inQuote
			continue
		}

		if inQuote {
			continue
		}

		if r == left {
			count++
		}
		if r == right {
			count--
			if count < 0 {
				return false
			}
		}
	}

	return count == 0
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// detectRisksByStatement 逐条语句应用风险规则，产出命中项、累计风险分值和综合风险等级。
func detectRisksByStatement(db DBType, stmts []SQLStatement, rules []RiskRule) ([]MatchResult, int, Severity) {
	results := make([]MatchResult, 0)
	score := 0

	for _, stmt := range stmts {
		sql := stmt.Normalized
		if sql == "" {
			continue
		}

		// 单引号不平衡的语句不参与风险判断
		if !isQuoteBalancedText(stmt.Raw) {
			continue
		}

		for _, rule := range rules {
			if !supportsDB(rule.Databases, db) {
				continue
			}

			matched := false
			if rule.CustomCheck != nil {
				matched = rule.CustomCheck(sql)
			} else if rule.Pattern != nil {
				matched = rule.Pattern.MatchString(sql)
			}

			if matched {
				results = append(results, MatchResult{
					StatementNo: stmt.No,
					Line:        stmt.StartLine,
					RuleID:      rule.ID,
					Name:        rule.Name,
					Severity:    rule.Severity,
					Description: rule.Description,
					Suggestion:  rule.Suggestion,
					SQL:         strings.TrimSpace(stmt.Raw),
				})
				score += severityScore(rule.Severity)
			}
		}
	}

	results = deduplicateMatches(results)

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Line == results[j].Line {
			if results[i].Severity == results[j].Severity {
				return results[i].RuleID < results[j].RuleID
			}
			return severityScore(results[i].Severity) > severityScore(results[j].Severity)
		}
		return results[i].Line < results[j].Line
	})

	return results, score, calculateRiskLevel(score)
}

func deduplicateMatches(in []MatchResult) []MatchResult {
	seen := map[string]bool{}
	out := make([]MatchResult, 0, len(in))
	for _, item := range in {
		key := strconv.Itoa(item.Line) + "|" + item.RuleID + "|" + item.SQL
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func severityScore(s Severity) int {
	switch s {
	case SeverityCritical:
		return 40
	case SeverityHigh:
		return 25
	case SeverityMedium:
		return 12
	default:
		return 5
	}
}

func calculateRiskLevel(score int) Severity {
	switch {
	case score >= 60:
		return SeverityCritical
	case score >= 30:
		return SeverityHigh
	case score >= 12:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func supportsDB(list []DBType, db DBType) bool {
	for _, item := range list {
		if item == db {
			return true
		}
	}
	return false
}

func isQuoteBalancedText(s string) bool {
	runes := []rune(s)
	inQuote := false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\'' {
			if i+1 < len(runes) && runes[i+1] == '\'' {
				i++
				continue
			}
			inQuote = !inQuote
		}
	}
	return !inQuote
}

// buildRules 定义全部高风险规则。
// 新增风险规则时，通常只需要在这里追加配置，而不用改动外层处理流程。
func buildRules() []RiskRule {
	return []RiskRule{
		{
			ID:          "R001",
			Name:        "DELETE without WHERE",
			Description: "检测到DELETE语句未带WHERE条件,可能导致整表数据被删除",
			Severity:    SeverityCritical,
			Databases:   []DBType{MySQL, Oracle},
			CustomCheck: func(sql string) bool {
				return regexp.MustCompile(`\bDELETE\s+FROM\s+[\w.$"]+\b`).MatchString(sql) &&
					!regexp.MustCompile(`\bWHERE\b`).MatchString(sql)
			},
			Suggestion: "请补充WHERE条件,或先SELECT验证影响范围",
		},
		{
			ID:          "R002",
			Name:        "UPDATE without WHERE",
			Description: "检测到UPDATE语句未带WHERE条件,可能更新全表",
			Severity:    SeverityCritical,
			Databases:   []DBType{MySQL, Oracle},
			Suggestion:  "请增加WHERE条件,并优先在事务中执行",
			CustomCheck: func(sql string) bool {
				return regexp.MustCompile(`\bUPDATE\s+[\w.$"]+\s+SET\b`).MatchString(sql) &&
					!regexp.MustCompile(`\bWHERE\b`).MatchString(sql)
			},
		},
		{
			ID:          "R003",
			Name:        "TRUNCATE TABLE",
			Description: "检测到TRUNCATE TABLE,属于高风险清表操作",
			Severity:    SeverityCritical,
			Databases:   []DBType{MySQL, Oracle},
			Pattern:     regexp.MustCompile(`\bTRUNCATE\s+TABLE\b`),
			Suggestion:  "确认目标环境和表名,必要时增加人工审批",
		},
		{
			ID:          "R004",
			Name:        "DROP TABLE or VIEW",
			Description: "检测到DROP TABLE或VIEW,属于破坏性DDL",
			Severity:    SeverityCritical,
			Databases:   []DBType{MySQL, Oracle},
			Pattern:     regexp.MustCompile(`\bDROP\s+(TABLE|VIEW)\b`),
			Suggestion:  "生产环境建议禁用或走审批",
		},
		{
			ID:          "R005",
			Name:        "DROP DATABASE/SCHEMA/USER",
			Description: "检测到DROP DATABASE或SCHEMA或USER,属于极高危操作",
			Severity:    SeverityCritical,
			Databases:   []DBType{MySQL, Oracle},
			Pattern:     regexp.MustCompile(`\bDROP\s+(DATABASE|SCHEMA|USER)\b`),
			Suggestion:  "建议直接阻断并要求DBA二次确认",
		},
		{
			ID:          "R006",
			Name:        "ALTER TABLE DROP COLUMN",
			Description: "检测到ALTER TABLE DROP COLUMN,可能导致结构破坏和兼容问题",
			Severity:    SeverityHigh,
			Databases:   []DBType{MySQL, Oracle},
			Pattern:     regexp.MustCompile(`\bALTER\s+TABLE\b.*\bDROP\s+(COLUMN\s+)?`),
			Suggestion:  "请先确认依赖关系并在变更窗口执行",
		},
		{
			ID:          "R007",
			Name:        "INSERT INTO SELECT",
			Description: "检测到INSERT INTO SELECT,可能造成大量数据写入",
			Severity:    SeverityMedium,
			Databases:   []DBType{MySQL, Oracle},
			Pattern:     regexp.MustCompile(`\bINSERT\s+INTO\b.*\bSELECT\b`),
			Suggestion:  "建议增加数据量评估与执行前预估",
		},
		{
			ID:          "R008",
			Name:        "UNION SELECT",
			Description: "检测到UNION SELECT,可能与注入探测或高成本查询有关",
			Severity:    SeverityMedium,
			Databases:   []DBType{MySQL, Oracle},
			Pattern:     regexp.MustCompile(`\bUNION\s+(ALL\s+)?SELECT\b`),
			Suggestion:  "请确认SQL来源可信,并检查执行计划",
		},
		{
			ID:          "R009",
			Name:        "SELECT *",
			Description: "检测到SELECT *,可能带来性能和字段泄露风险",
			Severity:    SeverityLow,
			Databases:   []DBType{MySQL, Oracle},
			Pattern:     regexp.MustCompile(`\bSELECT\s+\*\s+FROM\b`),
			Suggestion:  "建议改为显式字段列表",
		},
		{
			ID:          "R010",
			Name:        "JOIN without ON",
			Description: "检测到JOIN但可能缺少ON条件,存在笛卡尔积风险",
			Severity:    SeverityHigh,
			Databases:   []DBType{MySQL, Oracle},
			Suggestion:  "请确认JOIN条件完整,并检查结果集规模",
			CustomCheck: func(sql string) bool {
				return regexp.MustCompile(`\bJOIN\b`).MatchString(sql) &&
					!regexp.MustCompile(`\bON\b`).MatchString(sql)
			},
		},
		{
			ID:          "R011",
			Name:        "Oracle EXECUTE IMMEDIATE",
			Description: "检测到Oracle动态SQL,存在注入和运行时风险",
			Severity:    SeverityHigh,
			Databases:   []DBType{Oracle},
			Pattern:     regexp.MustCompile(`\bEXECUTE\s+IMMEDIATE\b`),
			Suggestion:  "建议配合绑定变量并限制动态拼接来源",
		},
		{
			ID:          "R012",
			Name:        "Oracle DBMS_SQL",
			Description: "检测到Oracle DBMS_SQL包调用,需重点审查动态执行逻辑",
			Severity:    SeverityHigh,
			Databases:   []DBType{Oracle},
			Pattern:     regexp.MustCompile(`\bDBMS_SQL\b`),
			Suggestion:  "请增加白名单校验和参数绑定",
		},
		{
			ID:          "R013",
			Name:        "MySQL INTO OUTFILE",
			Description: "检测到MySQL INTO OUTFILE,可能涉及数据导出",
			Severity:    SeverityCritical,
			Databases:   []DBType{MySQL},
			Pattern:     regexp.MustCompile(`\bINTO\s+OUTFILE\b`),
			Suggestion:  "建议默认拦截,防止数据泄露",
		},
		{
			ID:          "R014",
			Name:        "MySQL LOAD DATA",
			Description: "检测到MySQL LOAD DATA,可能造成批量导入和脏数据风险",
			Severity:    SeverityHigh,
			Databases:   []DBType{MySQL},
			Pattern:     regexp.MustCompile(`\bLOAD\s+DATA\b`),
			Suggestion:  "请校验来源文件,字符集和目标表",
		},
		{
			ID:          "R015",
			Name:        "Sleep/Benchmark function",
			Description: "检测到延时函数,可能与注入探测,压测或恶意SQL相关",
			Severity:    SeverityMedium,
			Databases:   []DBType{MySQL, Oracle},
			Pattern:     regexp.MustCompile(`\b(SLEEP|BENCHMARK|DBMS_LOCK\.SLEEP)\b`),
			Suggestion:  "建议限制此类函数在生产库执行",
		},
	}
}
