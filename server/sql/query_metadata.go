package sql

import (
	"database/sql"
	"fmt"
	"strings"

	"gin-vue-redhat/server/auth"
)

/*
query_metadata.go
----------------------------------------------------------------------
该文件负责数据库元数据查询（表名 / 字段提示）。

本次重点新增：
1. 按当前用户权限校验 connectionName
2. 普通用户只能查询自己被分配的连接元数据
3. admin 默认可查询所有启用连接的元数据

同时保留之前 Oracle 11g 的修复：
1. Oracle 使用 :1 / :2 / :3 占位符，不能用 ?
2. Oracle 注释字段使用 sql.NullString 扫描，避免 NULL 转 string 报错
*/

// MetadataTable
// ------------------------------------------------------------
// 表元数据
type MetadataTable struct {
	Name    string `json:"name"`    // 表名
	Comment string `json:"comment"` // 表注释
}

// MetadataColumn
// ------------------------------------------------------------
// 字段元数据
type MetadataColumn struct {
	TableName  string `json:"tableName"`  // 所属表名
	ColumnName string `json:"columnName"` // 字段名
	ColumnType string `json:"columnType"` // 字段类型
	Comment    string `json:"comment"`    // 字段注释
}

// QueryMetadataResponse
// ------------------------------------------------------------
// 元数据查询接口返回结构
type QueryMetadataResponse struct {
	OK      bool             `json:"ok"`
	Message string           `json:"message"`
	Tables  []MetadataTable  `json:"tables"`
	Columns []MetadataColumn `json:"columns"`
}

// SearchQueryMetadataByConnectionWithUser
// ------------------------------------------------------------
// 按用户权限查询元数据。
//
// 权限规则：
// - admin：可查询全部启用连接的元数据
// - user：只能查询被分配给自己的连接元数据
func SearchQueryMetadataByConnectionWithUser(
	userID int64,
	roleName string,
	connectionName string,
	keyword string,
) QueryMetadataResponse {
	connName := strings.TrimSpace(connectionName)
	if connName == "" {
		return QueryMetadataResponse{
			OK:      false,
			Message: "连接名称不能为空",
			Tables:  []MetadataTable{},
			Columns: []MetadataColumn{},
		}
	}

	// 权限校验
	ok, err := auth.UserCanAccessConnection(userID, roleName, connName)
	if err != nil {
		return QueryMetadataResponse{
			OK:      false,
			Message: err.Error(),
			Tables:  []MetadataTable{},
			Columns: []MetadataColumn{},
		}
	}
	if !ok {
		return QueryMetadataResponse{
			OK:      false,
			Message: "当前用户无权访问该数据库连接",
			Tables:  []MetadataTable{},
			Columns: []MetadataColumn{},
		}
	}

	conn, err := auth.LoadConnectionByName(connName)
	if err != nil {
		return QueryMetadataResponse{
			OK:      false,
			Message: err.Error(),
			Tables:  []MetadataTable{},
			Columns: []MetadataColumn{},
		}
	}

	db, err := openDBByConnectionRecord(conn)
	if err != nil {
		return QueryMetadataResponse{
			OK:      false,
			Message: "数据库连接失败：" + err.Error(),
			Tables:  []MetadataTable{},
			Columns: []MetadataColumn{},
		}
	}
	defer db.Close()

	dbType := strings.ToLower(strings.TrimSpace(conn.DBType))
	kw := strings.TrimSpace(keyword)

	var tables []MetadataTable
	var columns []MetadataColumn

	switch dbType {
	case "mysql":
		tables, columns, err = searchMySQLMetadata(db, kw)
	case "oracle":
		tables, columns, err = searchOracleMetadata(db, kw)
	default:
		err = fmt.Errorf("不支持的数据库类型：%s", dbType)
	}

	if err != nil {
		return QueryMetadataResponse{
			OK:      false,
			Message: err.Error(),
			Tables:  []MetadataTable{},
			Columns: []MetadataColumn{},
		}
	}

	return QueryMetadataResponse{
		OK:      true,
		Message: "元数据查询成功",
		Tables:  tables,
		Columns: columns,
	}
}

// SearchQueryMetadataByConnection
// ------------------------------------------------------------
// 保留旧函数名，兼容旧调用。
// 旧函数默认按 admin 权限查询。
func SearchQueryMetadataByConnection(connectionName string, keyword string) QueryMetadataResponse {
	return SearchQueryMetadataByConnectionWithUser(0, "admin", connectionName, keyword)
}

// searchMySQLMetadata
// ------------------------------------------------------------
// MySQL 表/字段元数据查询
func searchMySQLMetadata(db *sql.DB, keyword string) ([]MetadataTable, []MetadataColumn, error) {
	kwLike := "%"
	if keyword != "" {
		kwLike = "%" + keyword + "%"
	}

	tableSQL := `
SELECT table_name, IFNULL(table_comment, '')
FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND (table_name LIKE ? OR table_comment LIKE ?)
ORDER BY table_name
LIMIT 100
`

	columnSQL := `
SELECT table_name, column_name, column_type, IFNULL(column_comment, '')
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND (table_name LIKE ? OR column_name LIKE ? OR column_comment LIKE ?)
ORDER BY table_name, ordinal_position
LIMIT 300
`

	tables, err := queryMetadataTables(db, tableSQL, kwLike, kwLike)
	if err != nil {
		return nil, nil, err
	}

	columns, err := queryMetadataColumns(db, columnSQL, kwLike, kwLike, kwLike)
	if err != nil {
		return nil, nil, err
	}

	return tables, columns, nil
}

// searchOracleMetadata
// ------------------------------------------------------------
// Oracle 11g 表/字段元数据查询
//
// 关键点：
// 1. Oracle 不能用 ? 占位符，必须使用 :1 / :2 / :3
// 2. Oracle 空字符串会被当成 NULL，所以注释和表达式都要按 NullString 扫描
// 3. 使用 user_ 视图适配当前登录用户对象
func searchOracleMetadata(db *sql.DB, keyword string) ([]MetadataTable, []MetadataColumn, error) {
	kwLike := "%"
	if keyword != "" {
		kwLike = "%" + strings.ToUpper(strings.TrimSpace(keyword)) + "%"
	}

	tableSQL := `
SELECT
    t.table_name,
    NVL(c.comments, ' ')
FROM user_tables t
LEFT JOIN user_tab_comments c
  ON t.table_name = c.table_name
WHERE (
        UPPER(t.table_name) LIKE :1
        OR UPPER(NVL(c.comments, ' ')) LIKE :2
      )
ORDER BY t.table_name
`

	columnSQL := `
SELECT
    col.table_name,
    col.column_name,
    col.data_type ||
    CASE
      WHEN col.data_type IN ('VARCHAR2', 'CHAR', 'NVARCHAR2', 'NCHAR') THEN '(' || col.data_length || ')'
      WHEN col.data_type = 'NUMBER' AND col.data_precision IS NOT NULL AND col.data_scale IS NOT NULL THEN '(' || col.data_precision || ',' || col.data_scale || ')'
      WHEN col.data_type = 'NUMBER' AND col.data_precision IS NOT NULL THEN '(' || col.data_precision || ')'
      ELSE ''
    END AS column_type,
    NVL(cc.comments, ' ')
FROM user_tab_columns col
LEFT JOIN user_col_comments cc
  ON col.table_name = cc.table_name
 AND col.column_name = cc.column_name
WHERE (
        UPPER(col.table_name) LIKE :1
        OR UPPER(col.column_name) LIKE :2
        OR UPPER(NVL(cc.comments, ' ')) LIKE :3
      )
ORDER BY col.table_name, col.column_id
`

	tables, err := queryMetadataTables(db, tableSQL, kwLike, kwLike)
	if err != nil {
		return nil, nil, err
	}

	columns, err := queryMetadataColumns(db, columnSQL, kwLike, kwLike, kwLike)
	if err != nil {
		return nil, nil, err
	}

	return tables, columns, nil
}

// queryMetadataTables
// ------------------------------------------------------------
// 通用表元数据查询函数
//
// 为什么这里用 sql.NullString：
// - Oracle 注释字段可能是 NULL
// - Oracle 中 ” 也会被当成 NULL
// - 如果直接扫到 string，会报 converting NULL to string is unsupported
func queryMetadataTables(db *sql.DB, sqlText string, args ...interface{}) ([]MetadataTable, error) {
	rows, err := db.Query(sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]MetadataTable, 0)
	for rows.Next() {
		var item MetadataTable
		var comment sql.NullString

		if err := rows.Scan(&item.Name, &comment); err != nil {
			return nil, err
		}

		if comment.Valid {
			item.Comment = strings.TrimSpace(comment.String)
		} else {
			item.Comment = ""
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// queryMetadataColumns
// ------------------------------------------------------------
// 通用字段元数据查询函数
func queryMetadataColumns(db *sql.DB, sqlText string, args ...interface{}) ([]MetadataColumn, error) {
	rows, err := db.Query(sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]MetadataColumn, 0)
	for rows.Next() {
		var item MetadataColumn
		var columnType sql.NullString
		var comment sql.NullString

		if err := rows.Scan(&item.TableName, &item.ColumnName, &columnType, &comment); err != nil {
			return nil, err
		}

		if columnType.Valid {
			item.ColumnType = strings.TrimSpace(columnType.String)
		} else {
			item.ColumnType = ""
		}

		if comment.Valid {
			item.Comment = strings.TrimSpace(comment.String)
		} else {
			item.Comment = ""
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
