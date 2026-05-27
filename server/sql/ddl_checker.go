package sql

// ddl_checker.go 是 DDL 合规性检查模块。
// 它的核心思路不是做完整 SQL 解析器，而是针对项目关心的建表/建索引规范，
// 通过文本拆分、正则匹配与结构化抽取的方式，给出可读性较强的规范问题列表。

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DDLRuleSeverity 表示 DDL 规范问题的重要程度。
// 与 SQL 风险检查不同，这里更多用于表达“建议 / 警告 / 错误”三个层次。
type DDLRuleSeverity string

const (
	DDLSeveritySuggestion DDLRuleSeverity = "suggestion"
	DDLSeverityWarning    DDLRuleSeverity = "warning"
	DDLSeverityError      DDLRuleSeverity = "error"
)

// DDLIssue 表示一条 DDL 规范问题。
// 返回前端时只保留对象类型、对象名称、说明、建议四个核心展示字段。
type DDLIssue struct {
	Line        int             `json:"-"`
	Severity    DDLRuleSeverity `json:"-"`
	ObjectType  string          `json:"objectType"`
	ObjectName  string          `json:"objectName"`
	Description string          `json:"description"`
	Suggestion  string          `json:"suggestion"`
	RuleKey     string          `json:"-"`
}

type DDLCheckResponse struct {
	OK         bool       `json:"ok"`
	DBType     string     `json:"dbType"`
	DDLMessage string     `json:"ddlMessage"`
	IssueCount int        `json:"issueCount"`
	Issues     []DDLIssue `json:"issues"`
}

type ddlColumn struct {
	Name string
	Raw  string
}

type ddlIndex struct {
	Name      string
	TableName string
	Columns   []string
	IsUnique  bool
}

// ddlTable 是内部使用的建表抽象结构。
// 它把 CREATE TABLE 语句提取成结构化信息，便于后续统一检查主键、索引、注释、字段命名等规则。
type ddlTable struct {
	Name            string
	Columns         []ddlColumn
	Indexes         []ddlIndex
	HasPrimaryKey   bool
	HasCreateTime   bool
	HasUpdateTime   bool
	HasTableComment bool
	HasForeignKey   bool
	Engine          string
	Charset         string
	Raw             string
	IsPartial       bool // [新增] 标记是否为仅通过 ALTER 提取的局部表信息
}

// 这一组正则表达式用于识别项目当前支持的核心 DDL 形式。
// 由于目标是静态规范检查而非完整语法解析，所以这里重点覆盖 CREATE TABLE、CREATE INDEX、COMMENT ON、ALTER TABLE ADD PRIMARY KEY 等场景。
var (
	//reCreateTable = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s*\((.*)\)\s*(.*)$`)
	reCreateTable = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_.$"` + "`" + `]+)\s*\((.*)\)\s*(.*)$`)

	//reCreateIndex = regexp.MustCompile(`(?is)^\s*CREATE\s+(UNIQUE\s+)?INDEX\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s+ON\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s*\(([^)]*)\)\s*;?\s*$`)
	reCreateIndex = regexp.MustCompile(`(?is)^\s*CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_.$"` + "`" + `]+)\s+ON\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s*\(([^)]*)\)\s*;?\s*$`)

	reCreateTableAs = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_.$"` + "`" + `]+)\s+AS\s+SELECT\s+.*$`)

	reCommentTable = regexp.MustCompile(`(?is)^\s*COMMENT\s+ON\s+TABLE\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s+IS\s+'[\s\S]*?'\s*;?\s*$`)

	//reCommentColumn = regexp.MustCompile(`(?is)^\s*COMMENT\s+ON\s+COLUMN\s+([a-zA-Z0-9_.$"` + "`" + `]+)\.([a-zA-Z0-9_.$"` + "`" + `]+)\s+IS\s+'[\s\S]*?'\s*;?\s*$`)
	// [修改] 给 '[\s\S]*?' 加上括号变成 '([\s\S]*?)'，用于提取具体的注释内容
	reCommentColumn = regexp.MustCompile(`(?is)^\s*COMMENT\s+ON\s+COLUMN\s+([a-zA-Z0-9_.$"` + "`" + `]+)\.([a-zA-Z0-9_.$"` + "`" + `]+)\s+IS\s+'([\s\S]*?)'\s*;?\s*$`)

	//reAlterTablePrimaryKey = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s+ADD\s+CONSTRAINT\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s+PRIMARY\s+KEY\s*\(([^)]*)\)(?:[\s\S]*?)?;?\s*$`)
	reAlterTablePrimaryKey = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s+ADD\s+(?:CONSTRAINT\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s+)?PRIMARY\s+KEY\s*\(([^)]*)\)(?:[\s\S]*?)?;?\s*$`)

	reAlterTableColumn = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([a-zA-Z0-9_.$"` + "`" + `]+)\s+(ADD|MODIFY|CHANGE)\s+(?:COLUMN\s+)?(.*)$`)

	reReserved = regexp.MustCompile(`^(ACCESS|ADD|ALL|ALTER|AND|ANY|AS|ASC|AUDIT|BETWEEN|BY|CHAR|CHECK|CLUSTER|COLUMN|COMMENT|COMPRESS|CONNECT|CREATE|CURRENT|DATE|DECIMAL|DEFAULT|DELETE|DESC|DISTINCT|DROP|ELSE|EXCLUSIVE|EXISTS|FILE|FLOAT|FOR|FROM|GRANT|GROUP|HAVING|IDENTIFIED|IMMEDIATE|IN|INCREMENT|INDEX|INITIAL|INSERT|INTEGER|INTERSECT|INTO|IS|LEVEL|LIKE|LOCK|LONG|MAXEXTENTS|MINUS|MLSLABEL|MODE|MODIFY|NOAUDIT|NOCOMPRESS|NOT|NOWAIT|NULL|NUMBER|OF|OFFLINE|ON|ONLINE|OPTION|OR|ORDER|PCTFREE|PRIOR|PRIVILEGES|PUBLIC|RAW|RENAME|RESOURCE|REVOKE|ROW|ROWID|ROWNUM|ROWS|SELECT|SESSION|SET|SHARE|SIZE|SMALLINT|START|SUCCESSFUL|SYNONYM|SYSDATE|TABLE|THEN|TO|TRIGGER|UID|UNION|UNIQUE|UPDATE|USER|VALIDATE|VALUES|VARCHAR|VARCHAR2|VIEW|WHENEVER|WHERE|WITH)$`)

	reBlockComment = regexp.MustCompile(`(?is)/\*.*?\*/`)
)

// CheckDDL 是 DDL 合规性检查统一入口。
// 它会先完成数据库类型与空内容校验，再拆分语句、抽取表与索引结构，最后聚合所有规范问题。
func CheckDDL(dbType string, rawSQL string) DDLCheckResponse {
	db := DBType(strings.ToLower(strings.TrimSpace(dbType)))
	if db != MySQL && db != Oracle {
		return DDLCheckResponse{
			OK:         false,
			DBType:     dbType,
			DDLMessage: "暂时只支持 mysql 和 oracle",
			IssueCount: 1,
			Issues: []DDLIssue{{
				ObjectType:  "database",
				ObjectName:  strings.ToUpper(strings.TrimSpace(dbType)),
				Description: "暂时只支持 mysql 和 oracle",
				Suggestion:  "请将数据库类型切换为 mysql 或 oracle 后重新检测",
			}},
		}
	}

	if strings.TrimSpace(rawSQL) == "" {
		return DDLCheckResponse{
			OK:         false,
			DBType:     string(db),
			DDLMessage: "DDL 不能为空",
			IssueCount: 1,
			Issues: []DDLIssue{{
				ObjectType:  "statement",
				ObjectName:  "DDL",
				Description: "请输入 CREATE TABLE、ALTER TABLE、CREATE INDEX、COMMENT ON 等 DDL 语句",
				Suggestion:  "请粘贴待检查的 DDL 内容",
			}},
		}
	}

	stmts := splitStatementsWithLines(rawSQL)
	issues := make([]DDLIssue, 0)

	commentedTables := map[string]bool{}
	commentedColumns := map[string]bool{}
	tableMap := map[string]*ddlTable{}

	for _, stmt := range stmts {
		raw := stripLineComments(stmt.Raw)
		if raw == "" {
			continue
		}

		if m := reCommentTable.FindStringSubmatch(raw); len(m) > 0 {
			tableName := strings.ToUpper(cleanIdentifier(m[1]))
			commentedTables[tableName] = true
			if t, ok := tableMap[tableName]; ok {
				t.HasTableComment = true
			}
			continue
		}

		if m := reCommentColumn.FindStringSubmatch(raw); len(m) > 0 {
			tableName := strings.ToUpper(cleanIdentifier(m[1]))
			colName := strings.ToUpper(cleanIdentifier(m[2]))
			commentText := m[3] // [新增] 提取出注释文本
			commentedColumns[tableName+"."+colName] = true
			// [新增] 根据提取出来的单独注释内容，反向修正表的 HasCreateTime / HasUpdateTime 状态
			if t, ok := tableMap[tableName]; ok {
				if strings.Contains(commentText, "创建时间") || strings.Contains(commentText, "插入时间") {
					t.HasCreateTime = true
				}
				if strings.Contains(commentText, "更新时间") {
					t.HasUpdateTime = true
				}
			}
			continue
		}

		if m := reAlterTablePrimaryKey.FindStringSubmatch(raw); len(m) > 0 {
			tableName := strings.ToUpper(cleanIdentifier(m[1]))
			if t, ok := tableMap[tableName]; ok {
				t.HasPrimaryKey = true
			} else {
				tableMap[tableName] = &ddlTable{
					Name:          cleanIdentifier(m[1]),
					HasPrimaryKey: true,
					IsPartial:     true, // [新增] 标记这是一个局部表，跳过全表检查
				}
			}
			continue
		}

		// [新增] 提取 ALTER TABLE ADD / MODIFY / CHANGE 的字段
		if m := reAlterTableColumn.FindStringSubmatch(raw); len(m) > 0 {
			tableName := strings.ToUpper(cleanIdentifier(m[1]))
			action := strings.ToUpper(m[2]) // ADD, MODIFY, CHANGE
			body := strings.TrimSpace(m[3])
			// if strings.HasSuffix(body, ";") {
			// 	body = body[:len(body)-1]
			// }
			body = strings.TrimSuffix(body, ";")

			// 排除约束、索引等非纯字段的操作
			upperBody := strings.ToUpper(body)
			if strings.HasPrefix(upperBody, "CONSTRAINT") || strings.HasPrefix(upperBody, "INDEX") || strings.HasPrefix(upperBody, "UNIQUE") || strings.HasPrefix(upperBody, "PRIMARY") {
				continue
			}

			var items []string
			// 处理带括号的多个字段：ALTER TABLE t ADD (col1 INT, col2 INT)
			if strings.HasPrefix(body, "(") && strings.HasSuffix(body, ")") {
				items = splitDDLItems(body[1 : len(body)-1])
			} else {
				items = []string{body}
			}

			var newCols []ddlColumn
			for _, item := range items {
				trimmed := strings.TrimSpace(item)
				if trimmed == "" {
					continue
				}

				colName := extractLeadingIdentifier(trimmed)
				// CHANGE 语法比较特殊: CHANGE old_col new_col type
				if action == "CHANGE" {
					fields := strings.Fields(trimmed)
					if len(fields) >= 2 {
						colName = fields[1]
					}
				}

				if colName != "" {
					newCols = append(newCols, ddlColumn{Name: cleanIdentifier(colName), Raw: trimmed})
				}
			}

			if len(newCols) > 0 {
				if t, ok := tableMap[tableName]; ok {
					t.Columns = append(t.Columns, newCols...)
				} else {
					// 创建局部表结构
					tableMap[tableName] = &ddlTable{
						Name:      cleanIdentifier(m[1]),
						Columns:   newCols,
						IsPartial: true,
					}
				}
			}
			continue
		}

		if idx, ok := parseStandaloneCreateIndex(raw); ok {
			tableName := strings.ToUpper(idx.TableName)
			if t, exists := tableMap[tableName]; exists {
				t.Indexes = append(t.Indexes, idx)
			} else {
				tableMap[tableName] = &ddlTable{
					Name:      idx.TableName,
					Indexes:   []ddlIndex{idx},
					IsPartial: true, // [新增] 标记这是一个局部表，跳过全表检查
				}
			}
			continue
		}

		cleanStmt := stmt
		cleanStmt.Raw = raw

		if table, ok := parseCreateTable(cleanStmt); ok {
			upperTable := strings.ToUpper(table.Name)
			if existed, ok := tableMap[upperTable]; ok {
				mergeDDLTable(existed, table)
			} else {
				t := table
				tableMap[upperTable] = &t
			}
		}

		if issuesForStmt := checkStandaloneStatementRules(db, cleanStmt); len(issuesForStmt) > 0 {
			issues = append(issues, issuesForStmt...)
		}
	}

	for tableName, table := range tableMap {
		if commentedTables[tableName] {
			table.HasTableComment = true
		}
		issues = append(issues, checkTableRules(db, *table, commentedColumns)...)
	}

	issues = dedupeAndSortDDLIssues(issues)

	msg := "DDL 规范检查通过"
	if len(issues) > 0 {
		msg = fmt.Sprintf("检测到 %d 个 DDL 规范问题", len(issues))
	}

	ok := true
	for _, item := range issues {
		if item.Severity == DDLSeverityError {
			ok = false
			break
		}
	}

	for i := range issues {
		issues[i].Line = 0
		issues[i].Severity = ""
		issues[i].RuleKey = ""
	}

	return DDLCheckResponse{
		OK:         ok,
		DBType:     string(db),
		DDLMessage: msg,
		IssueCount: len(issues),
		Issues:     issues,
	}
}

// mergeDDLTable 用于合并同一张表在多条 DDL 中提取到的信息。
// 例如先 CREATE TABLE，后 COMMENT ON TABLE / ALTER TABLE ADD PRIMARY KEY 时，需要把这些信息汇总到同一张表对象上。
func mergeDDLTable(dst *ddlTable, src ddlTable) {
	if dst.Name == "" {
		dst.Name = src.Name
	}
	dst.Columns = append(dst.Columns, src.Columns...)
	dst.Indexes = append(dst.Indexes, src.Indexes...)
	dst.HasPrimaryKey = dst.HasPrimaryKey || src.HasPrimaryKey
	dst.HasCreateTime = dst.HasCreateTime || src.HasCreateTime
	dst.HasUpdateTime = dst.HasUpdateTime || src.HasUpdateTime
	dst.HasTableComment = dst.HasTableComment || src.HasTableComment
	dst.HasForeignKey = dst.HasForeignKey || src.HasForeignKey

	// [修改处] 如果旧对象是局部表，新对象是完整建表，需要覆盖并消除 Partial 标记
	if dst.IsPartial && !src.IsPartial {
		dst.IsPartial = false
		dst.HasPrimaryKey = src.HasPrimaryKey
		dst.HasCreateTime = src.HasCreateTime
		dst.HasUpdateTime = src.HasUpdateTime
		dst.HasTableComment = src.HasTableComment
		dst.HasForeignKey = src.HasForeignKey
	} else {
		dst.HasPrimaryKey = dst.HasPrimaryKey || src.HasPrimaryKey
		dst.HasCreateTime = dst.HasCreateTime || src.HasCreateTime
		dst.HasUpdateTime = dst.HasUpdateTime || src.HasUpdateTime
		dst.HasTableComment = dst.HasTableComment || src.HasTableComment
		dst.HasForeignKey = dst.HasForeignKey || src.HasForeignKey
	}

	if dst.Engine == "" {
		dst.Engine = src.Engine
	}
	if dst.Charset == "" {
		dst.Charset = src.Charset
	}
	if dst.Raw == "" {
		dst.Raw = src.Raw
	}
}

// checkStandaloneStatementRules 检查不依赖完整表结构上下文的单条语句级规则。
func checkStandaloneStatementRules(db DBType, stmt SQLStatement) []DDLIssue {
	raw := strings.TrimSpace(stmt.Raw)
	upper := strings.ToUpper(raw)
	issues := make([]DDLIssue, 0)

	if db == MySQL && strings.HasPrefix(upper, "CREATE TRIGGER") {
		name := extractObjectNameAfterPrefix(raw, "CREATE TRIGGER")
		issues = append(issues, newIssue(stmt.StartLine, DDLSeverityWarning, "trigger", strings.ToUpper(name), "避免使用触发器", "建议将业务逻辑下沉到应用层实现", "mysql_avoid_trigger"))
	}
	if db == MySQL && strings.HasPrefix(upper, "CREATE PROCEDURE") {
		name := extractObjectNameAfterPrefix(raw, "CREATE PROCEDURE")
		issues = append(issues, newIssue(stmt.StartLine, DDLSeverityWarning, "procedure", strings.ToUpper(name), "避免使用存储过程", "建议将复杂逻辑放在应用服务中实现", "mysql_avoid_procedure"))
	}

	if db == Oracle {
		if strings.HasPrefix(upper, "CREATE OR REPLACE VIEW") || strings.HasPrefix(upper, "CREATE VIEW") {
			name := extractObjectNameAfterPrefix(raw, "CREATE OR REPLACE VIEW")
			if name == "" {
				name = extractObjectNameAfterPrefix(raw, "CREATE VIEW")
			}
			if !strings.HasPrefix(strings.ToLower(cleanIdentifier(name)), "v_") {
				issues = append(issues, newIssue(stmt.StartLine, DDLSeverityError, "view", strings.ToUpper(cleanIdentifier(name)), "视图名称应以 v_ 开头", "例如 v_user_account、v_order_detail", "oracle_view_naming"))
			}
		}
		if strings.HasPrefix(upper, "CREATE SEQUENCE") {
			name := extractObjectNameAfterPrefix(raw, "CREATE SEQUENCE")
			if !strings.HasPrefix(strings.ToLower(cleanIdentifier(name)), "seq_") {
				issues = append(issues, newIssue(stmt.StartLine, DDLSeverityError, "sequence", strings.ToUpper(cleanIdentifier(name)), "序列名称应以 seq_ 开头", "例如 seq_user_account", "oracle_sequence_naming"))
			}
		}
		if strings.HasPrefix(upper, "CREATE OR REPLACE TRIGGER") || strings.HasPrefix(upper, "CREATE TRIGGER") {
			name := extractObjectNameAfterPrefix(raw, "CREATE OR REPLACE TRIGGER")
			if name == "" {
				name = extractObjectNameAfterPrefix(raw, "CREATE TRIGGER")
			}
			if !strings.HasPrefix(strings.ToLower(cleanIdentifier(name)), "trg_") {
				issues = append(issues, newIssue(stmt.StartLine, DDLSeverityError, "trigger", strings.ToUpper(cleanIdentifier(name)), "触发器名称应以 trg_ 开头", "例如 trg_user_account_bi", "oracle_trigger_naming"))
			}
		}
		if strings.HasPrefix(upper, "CREATE OR REPLACE PROCEDURE") || strings.HasPrefix(upper, "CREATE PROCEDURE") {
			name := extractObjectNameAfterPrefix(raw, "CREATE OR REPLACE PROCEDURE")
			if name == "" {
				name = extractObjectNameAfterPrefix(raw, "CREATE PROCEDURE")
			}
			if !strings.HasPrefix(strings.ToLower(cleanIdentifier(name)), "proc_") {
				issues = append(issues, newIssue(stmt.StartLine, DDLSeverityError, "procedure", strings.ToUpper(cleanIdentifier(name)), "存储过程名称应以 proc_ 开头", "例如 proc_sync_order_status", "oracle_procedure_naming"))
			}
		}
		if strings.Contains(upper, "AUTO_INCREMENT") || strings.Contains(upper, "ENGINE=") || strings.Contains(upper, "CHARSET=") || strings.Contains(raw, "`") {
			issues = append(issues, newIssue(stmt.StartLine, DDLSeverityError, "statement", "ORACLE", "Oracle DDL 中禁止使用 MySQL 特性", "请去掉 AUTO_INCREMENT、ENGINE、CHARSET 和反引号，改用 Oracle 语法", "oracle_forbid_mysql_syntax"))
		}
	}

	return issues
}

// checkTableRules 负责表级规范检查，是 DDL 模块的核心规则入口。
// 这里会集中检查主键、审计字段、表注释、字段注释、命名规范、引擎/字符集等问题。
func checkTableRules(db DBType, table ddlTable, commentedColumns map[string]bool) []DDLIssue {
	issues := make([]DDLIssue, 0)
	upperTable := strings.ToUpper(table.Name)

	// [新增包裹] 跳过仅针对完整建表语句的表级结构检查 (比如由 ALTER TABLE 或 CTAS 提取的局部表)
	if !table.IsPartial {
		if len([]rune(table.Name)) > 30 {
			// 去掉了报错文案里的“使用小写下划线命名”，只保留长度提示
			issues = append(issues, newIssue(1, DDLSeverityError, "table", upperTable, "表名长度不应超过 30 个字符", "表名请控制在 30 个字符以内，避免使用数据库关键字和保留字", "table_name_length_rule"))
		} else if isReservedWord(table.Name) {
			issues = append(issues, newIssue(1, DDLSeverityError, "table", upperTable, "表名应避免数据库关键字和保留字", "避免直接使用 order、group、user 等关键字", "table_reserved_word_rule"))
		}
		if !table.HasPrimaryKey {
			issues = append(issues, newIssue(1, DDLSeverityError, "table", upperTable, "表缺少主键", "请为表增加主键，例如 CONSTRAINT pk_xxx PRIMARY KEY (id)", "table_primary_key_required"))
		}

		if !table.HasCreateTime {
			issues = append(issues, newIssue(1, DDLSeverityWarning, "table", upperTable, "表缺少 create_time 字段", "建议增加 create_time 字段，用于记录创建时间", "table_create_time_required"))
		}
		if !table.HasUpdateTime {
			issues = append(issues, newIssue(1, DDLSeverityWarning, "table", upperTable, "表缺少 update_time 字段", "建议增加 update_time 字段，用于记录更新时间", "table_update_time_required"))
		}

		if db == MySQL && !table.HasTableComment {
			issues = append(issues, newIssue(1, DDLSeverityError, "table", upperTable, "表缺少注释", "请在建表语句末尾补充 COMMENT='...'", "mysql_table_comment_required"))
		}

		if db == Oracle {
			if !table.HasTableComment {
				issues = append(issues, newIssue(1, DDLSeverityError, "table", upperTable, "表缺少注释", fmt.Sprintf("请补充：COMMENT ON TABLE %s IS '...';", table.Name), "oracle_table_comment_required"))
			}
			if table.HasForeignKey {
				issues = append(issues, newIssue(1, DDLSeverityWarning, "table", upperTable, "避免使用外键，容易锁表", "建议通过应用逻辑或程序校验保持数据一致性", "oracle_avoid_foreign_key"))
			}
			if len(table.Indexes) > 5 {
				issues = append(issues, newIssue(1, DDLSeverityWarning, "table", upperTable, "单表上的索引建议不超过5个", "请评估索引必要性，避免过多索引影响写入性能", "oracle_index_count_limit"))
			}
		}

		if db == MySQL {
			if strings.ToUpper(table.Engine) != "INNODB" {
				issues = append(issues, newIssue(1, DDLSeverityError, "table", upperTable, "MySQL 表必须使用 InnoDB 存储引擎", "请在建表语句中指定 ENGINE=InnoDB", "mysql_engine_rule"))
			}
			if strings.ToLower(table.Charset) != "utf8mb4" {
				issues = append(issues, newIssue(1, DDLSeverityError, "table", upperTable, "MySQL 表必须使用 utf8mb4 字符集", "请在建表语句中指定 DEFAULT CHARSET=utf8mb4", "mysql_charset_rule"))
			}
		}
	} // [包裹结束]

	// ======= 字段级检查 (完整建表和 ALTER 增量字段均生效) =======
	for _, col := range table.Columns {
		upperCol := strings.ToUpper(col.Name)
		colUpperRaw := strings.ToUpper(col.Raw)

		if len([]rune(col.Name)) > 30 {
			issues = append(issues, newIssue(1, DDLSeverityError, "column", upperCol, "字段名长度不应超过 30 个字符", "请缩短字段名，保持语义清晰，例如 customer_no、order_code", "column_name_length_rule"))
		} else if !isLowerSnake(col.Name) {
			issues = append(issues, newIssue(1, DDLSeverityError, "column", upperCol, "字段名应使用小写下划线命名", "例如 customer_name、create_time、update_time", "column_name_format_rule"))
		} else if isReservedWord(col.Name) {
			issues = append(issues, newIssue(1, DDLSeverityError, "column", upperCol, "字段名避免使用数据库关键字和保留字", "请重命名字段，避免使用 user、order、group、level 等关键字", "column_reserved_word_rule"))
		}

		if db == MySQL {
			if !strings.Contains(colUpperRaw, " COMMENT ") {
				issues = append(issues, newIssue(1, DDLSeverityError, "column", upperCol, "字段缺少注释", "请在字段定义中补充 COMMENT '...'", "column_comment_required"))
			}
		}

		if db == Oracle {
			colKey := upperTable + "." + upperCol
			if !commentedColumns[colKey] {
				issues = append(issues, newIssue(1, DDLSeverityError, "column", upperCol, "字段缺少注释", fmt.Sprintf("请补充：COMMENT ON COLUMN %s.%s IS '...';", table.Name, col.Name), "column_comment_required"))
			}
		}

		if !strings.Contains(colUpperRaw, "NOT NULL") {
			issues = append(issues, newIssue(1, DDLSeverityWarning, "column", upperCol, "字段应尽量添加 NOT NULL 属性", "请根据业务含义为字段增加 NOT NULL 约束", "column_not_null_rule"))
		}
		if !strings.Contains(colUpperRaw, "DEFAULT ") {
			issues = append(issues, newIssue(1, DDLSeverityWarning, "column", upperCol, "字段应尽量设置默认值", "请根据业务含义为字段设置合理默认值", "column_default_rule"))
		}

		if db == MySQL && (strings.Contains(colUpperRaw, "TEXT") || strings.Contains(colUpperRaw, "BLOB")) {
			issues = append(issues, newIssue(1, DDLSeverityError, "column", upperCol, "MySQL 禁止使用 text、blob 此类大字段", "请使用长度明确的 VARCHAR 等类型，避免使用 text/blob", "mysql_no_text_blob"))
		}

		if db == Oracle {
			if strings.Contains(colUpperRaw, " COMMENT ") {
				issues = append(issues, newIssue(1, DDLSeverityError, "column", upperCol, "Oracle 字段注释应使用 COMMENT ON COLUMN 语法", "请改为 COMMENT ON COLUMN 表名.字段名 IS '...';", "oracle_comment_syntax"))
			}
			if strings.Contains(colUpperRaw, "VARCHAR2(4000)") {
				issues = append(issues, newIssue(1, DDLSeverityWarning, "column", upperCol, "字段数据类型和长度需要根据实际数据合理定义", "请根据实际数据范围合理设置 VARCHAR2 长度，避免盲目使用超大长度", "oracle_column_length_rule"))
			}
		}
	}

	// ======= 索引级检查 (完整建表和 ADD INDEX 均生效) =======
	issues = append(issues, checkIndexNamingRules(table.Indexes)...)
	issues = append(issues, redundantIndexIssues(table.Indexes)...)

	return issues
}

func checkIndexNamingRules(indexes []ddlIndex) []DDLIssue {
	issues := make([]DDLIssue, 0)
	for _, idx := range indexes {
		name := strings.ToLower(idx.Name)
		upperName := strings.ToUpper(idx.Name)
		if idx.IsUnique {
			if !strings.HasPrefix(name, "uk_") {
				issues = append(issues, newIssue(1, DDLSeverityError, "index", upperName, "唯一索引名应以 uk_ 开头", "例如 uk_trade_record_order_no", "unique_index_name_rule"))
			}
		} else {
			if !strings.HasPrefix(name, "idx_") {
				issues = append(issues, newIssue(1, DDLSeverityError, "index", upperName, "普通索引名应以 idx_ 开头", "例如 idx_trade_record_user_id", "normal_index_name_rule"))
			}
		}
	}
	return issues
}

func redundantIndexIssues(indexes []ddlIndex) []DDLIssue {
	issues := make([]DDLIssue, 0)
	for i := 0; i < len(indexes); i++ {
		for j := 0; j < len(indexes); j++ {
			if i == j {
				continue
			}
			if isPrefixColumns(indexes[i].Columns, indexes[j].Columns) && len(indexes[j].Columns) > len(indexes[i].Columns) {
				issues = append(issues, newIssue(1, DDLSeverityWarning, "index", strings.ToUpper(indexes[i].Name), "避免冗余索引", "例如 index(a,b,c) 已存在时，通常不需要再单独创建 index(a,b) 或 index(a)", "redundant_index_rule"))
				break
			}
		}
	}
	return issues
}

// parseCreateTable 尝试把 CREATE TABLE 文本解析成 ddlTable 结构。
// 若语句不符合当前支持的模式，则返回 false，外层会跳过该解析结果。
func parseCreateTable(stmt SQLStatement) (ddlTable, bool) {
	raw := strings.TrimSpace(stmt.Raw)
	m := reCreateTable.FindStringSubmatch(raw)
	if len(m) == 0 {
		return ddlTable{}, false
	}

	tableName := cleanIdentifier(m[1])
	body := m[2]
	tail := m[3]

	result := ddlTable{
		Name:            tableName,
		Columns:         make([]ddlColumn, 0),
		Indexes:         make([]ddlIndex, 0),
		HasTableComment: strings.Contains(strings.ToUpper(raw), " COMMENT=") || strings.Contains(strings.ToUpper(raw), " COMMENT ="),
		Raw:             raw,
	}

	if engine := extractTailKV(tail, `ENGINE\s*=\s*([A-Z0-9_]+)`); engine != "" {
		result.Engine = engine
	}
	if charset := extractTailKV(tail, `(?:DEFAULT\s+)?CHARSET\s*=\s*([A-Z0-9_]+)`); charset != "" {
		result.Charset = charset
	}

	parts := splitDDLItems(body)
	for _, item := range parts {
		trimmed := strings.TrimSpace(item)
		upper := strings.ToUpper(trimmed)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(upper, "PRIMARY KEY") || strings.Contains(upper, " PRIMARY KEY") {
			result.HasPrimaryKey = true
			continue
		}
		if strings.Contains(upper, "FOREIGN KEY") {
			result.HasForeignKey = true
			continue
		}
		if strings.HasPrefix(upper, "KEY ") || strings.HasPrefix(upper, "INDEX ") || strings.HasPrefix(upper, "UNIQUE KEY ") || strings.HasPrefix(upper, "UNIQUE INDEX ") || strings.HasPrefix(upper, "CONSTRAINT ") {
			if idx, ok := parseInlineIndex(trimmed, tableName); ok {
				result.Indexes = append(result.Indexes, idx)
			}
			if strings.Contains(upper, " PRIMARY KEY") {
				result.HasPrimaryKey = true
			}
			continue
		}

		name := extractLeadingIdentifier(trimmed)
		if name == "" {
			continue
		}
		col := ddlColumn{Name: cleanIdentifier(name), Raw: trimmed}
		result.Columns = append(result.Columns, col)

		lname := strings.ToLower(col.Name)

		// [修改] 放宽判断条件：匹配常见英文名，或者原始 SQL（Raw）中包含目标中文注释
		if lname == "create_time" || lname == "created_at" || lname == "gmt_create" ||
			strings.Contains(trimmed, "创建时间") || strings.Contains(trimmed, "插入时间") {
			result.HasCreateTime = true
		}

		if lname == "update_time" || lname == "updated_at" || lname == "gmt_modified" ||
			strings.Contains(trimmed, "更新时间") {
			result.HasUpdateTime = true
		}

		if strings.Contains(strings.ToUpper(trimmed), "PRIMARY KEY") {
			result.HasPrimaryKey = true
		}
	}

	return result, true
}

func parseStandaloneCreateIndex(raw string) (ddlIndex, bool) {
	m := reCreateIndex.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) == 0 {
		return ddlIndex{}, false
	}
	isUnique := strings.TrimSpace(m[1]) != ""
	indexName := cleanIdentifier(m[2])
	tableName := cleanIdentifier(m[3])
	cols := parseColumns(m[4])
	if indexName == "" || tableName == "" || len(cols) == 0 {
		return ddlIndex{}, false
	}
	return ddlIndex{
		Name:      indexName,
		TableName: tableName,
		Columns:   cols,
		IsUnique:  isUnique,
	}, true
}

func parseInlineIndex(raw string, tableName string) (ddlIndex, bool) {
	upper := strings.ToUpper(strings.TrimSpace(raw))
	if strings.HasPrefix(upper, "CONSTRAINT ") &&
		!strings.Contains(upper, " INDEX ") &&
		!strings.Contains(upper, " KEY ") &&
		!strings.Contains(upper, " UNIQUE ") {
		return ddlIndex{}, false
	}

	colStart := strings.Index(raw, "(")
	colEnd := strings.LastIndex(raw, ")")
	if colStart < 0 || colEnd < 0 || colEnd <= colStart {
		return ddlIndex{}, false
	}

	prefix := strings.TrimSpace(raw[:colStart])
	fields := strings.Fields(prefix)
	if len(fields) == 0 {
		return ddlIndex{}, false
	}

	name := ""
	isUnique := false

	for i, f := range fields {
		u := strings.ToUpper(f)
		if u == "UNIQUE" {
			isUnique = true
		}
		if u == "KEY" || u == "INDEX" {
			if i+1 < len(fields) {
				name = cleanIdentifier(fields[i+1])
			}
			break
		}
	}

	if name == "" && strings.HasPrefix(strings.ToUpper(prefix), "CONSTRAINT ") {
		fields = strings.Fields(prefix)
		if len(fields) >= 2 {
			name = cleanIdentifier(fields[1])
		}
		if strings.Contains(strings.ToUpper(prefix), "UNIQUE") {
			isUnique = true
		}
	}

	cols := parseColumns(raw[colStart+1 : colEnd])
	if name == "" || len(cols) == 0 {
		return ddlIndex{}, false
	}

	return ddlIndex{
		Name:      name,
		TableName: tableName,
		Columns:   cols,
		IsUnique:  isUnique,
	}, true
}

// splitDDLItems 用于拆分 CREATE TABLE 括号内部的字段/约束项。
// 这里需要兼顾括号嵌套、引号、函数表达式等情况，避免简单按逗号切分造成错误。
func splitDDLItems(body string) []string {
	items := make([]string, 0)
	var current strings.Builder
	depth := 0
	inSingleQuote := false

	for _, r := range body {
		switch r {
		case '\'':
			inSingleQuote = !inSingleQuote
			current.WriteRune(r)
		case '(':
			if !inSingleQuote {
				depth++
			}
			current.WriteRune(r)
		case ')':
			if !inSingleQuote && depth > 0 {
				depth--
			}
			current.WriteRune(r)
		case ',':
			if !inSingleQuote && depth == 0 {
				items = append(items, current.String())
				current.Reset()
				continue
			}
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}

	if strings.TrimSpace(current.String()) != "" {
		items = append(items, current.String())
	}
	return items
}

func extractLeadingIdentifier(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return ""
	}
	switch strings.ToUpper(fields[0]) {
	case "PRIMARY", "UNIQUE", "KEY", "INDEX", "CONSTRAINT", "FOREIGN", "CHECK":
		return ""
	default:
		return fields[0]
	}
}

func parseColumns(raw string) []string {
	parts := strings.Split(raw, ",")
	cols := make([]string, 0, len(parts))
	for _, p := range parts {
		fields := strings.Fields(strings.TrimSpace(p))
		if len(fields) == 0 {
			continue
		}
		name := cleanIdentifier(fields[0])
		if name != "" {
			cols = append(cols, strings.ToLower(name))
		}
	}
	return cols
}

func extractTailKV(text, pattern string) string {
	re := regexp.MustCompile(`(?is)` + pattern)
	m := re.FindStringSubmatch(text)
	if len(m) > 1 {
		return strings.ToUpper(strings.TrimSpace(m[1]))
	}
	return ""
}

func isPrefixColumns(shorter, longer []string) bool {
	if len(shorter) >= len(longer) {
		return false
	}
	for i := range shorter {
		if !strings.EqualFold(shorter[i], longer[i]) {
			return false
		}
	}
	return true
}

func extractObjectNameAfterPrefix(sql, prefix string) string {
	trimmed := strings.TrimSpace(sql)
	upperTrimmed := strings.ToUpper(trimmed)
	upperPrefix := strings.ToUpper(prefix)
	if !strings.HasPrefix(upperTrimmed, upperPrefix) {
		return ""
	}
	rest := strings.TrimSpace(trimmed[len(prefix):])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return cleanIdentifier(fields[0])
}

func cleanIdentifier(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "`\"")
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return strings.TrimSpace(name)
}

func isLowerSnake(name string) bool {
	ok, _ := regexp.MatchString(`^[a-z][a-z0-9_]*$`, name)
	return ok
}

func isReservedWord(name string) bool {
	return reReserved.MatchString(strings.ToUpper(strings.TrimSpace(name)))
}

func stripLineComments(sql string) string {
	sql = reBlockComment.ReplaceAllString(sql, "")

	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func newIssue(line int, severity DDLRuleSeverity, objectType, objectName, description, suggestion, ruleKey string) DDLIssue {
	return DDLIssue{
		Line:        line,
		Severity:    severity,
		ObjectType:  objectType,
		ObjectName:  objectName,
		Description: description,
		Suggestion:  suggestion,
		RuleKey:     ruleKey,
	}
}

// dedupeAndSortDDLIssues 对检查结果去重并排序。
// 这样前端看到的结果会更稳定，同类问题也只保留最早发现的一条。
func dedupeAndSortDDLIssues(items []DDLIssue) []DDLIssue {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Line == items[j].Line {
			return items[i].ObjectName < items[j].ObjectName
		}
		return items[i].Line < items[j].Line
	})

	seen := map[string]bool{}
	result := make([]DDLIssue, 0, len(items))

	for _, item := range items {
		key := item.RuleKey
		if key == "" {
			key = item.Description
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}
