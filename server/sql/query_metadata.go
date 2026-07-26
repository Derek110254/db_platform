package sql

import (
	"database/sql"
	"fmt"
	"strings"
)

/*
query_metadata.go
----------------------------------------------------------------------
按当前用户被授权的连接读取表、视图和字段元数据。

数据库范围：
1. MySQL 读取连接指定数据库。
2. Oracle 读取配置 schema；未配置时使用当前连接用户。
3. PostgreSQL 读取配置 schema；未配置时使用 current_schema()，通常为 public。
4. MSSQL 读取当前数据库内可见的 schema、表、视图和字段。
5. 表注释和字段注释统一返回给前端，空注释使用空字符串。
*/

// MetadataTable 是元数据浏览中的表信息。
type MetadataTable struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

// MetadataColumn 是元数据浏览中的字段信息。
type MetadataColumn struct {
	TableName  string `json:"tableName"`
	ColumnName string `json:"columnName"`
	ColumnType string `json:"columnType"`
	Comment    string `json:"comment"`
}

// QueryMetadataResponse 是表和字段查询接口的统一返回结构。
type QueryMetadataResponse struct {
	OK      bool             `json:"ok"`
	Message string           `json:"message"`
	Tables  []MetadataTable  `json:"tables"`
	Columns []MetadataColumn `json:"columns"`
}

// SearchQueryMetadataByConnectionWithUser 校验用户连接权限后，按数据库类型读取目标范围元数据。
func SearchQueryMetadataByConnectionWithUser(
	userID int64,
	roleName string,
	connectionName string,
	keyword string,
) QueryMetadataResponse {
	connName := strings.TrimSpace(connectionName)
	if connName == "" {
		return emptyMetadataResponse(false, "连接名称不能为空")
	}

	ok, err := UserCanAccessConnection(userID, roleName, connName)
	if err != nil {
		return emptyMetadataResponse(false, err.Error())
	}
	if !ok {
		return emptyMetadataResponse(false, "当前用户无权访问该数据库连接")
	}

	conn, err := LoadConnectionByName(connName)
	if err != nil {
		return emptyMetadataResponse(false, err.Error())
	}

	db, err := openDBByConnectionRecord(conn)
	if err != nil {
		return emptyMetadataResponse(false, "数据库连接失败: "+err.Error())
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
		tables, columns, err = searchOracleMetadata(db, conn.DatabaseName, kw)
	case "postgres":
		tables, columns, err = searchPostgresMetadata(db, conn.ServiceName, kw)
	case "mssql":
		tables, columns, err = searchMSSQLMetadata(db, kw)
	default:
		err = fmt.Errorf("不支持的数据库类型: %s", dbType)
	}
	if err != nil {
		return emptyMetadataResponse(false, err.Error())
	}

	return QueryMetadataResponse{
		OK:      true,
		Message: "元数据查询成功",
		Tables:  tables,
		Columns: columns,
	}
}

// SearchQueryMetadataByConnection 保留旧调用入口，默认按管理员权限查询。
func SearchQueryMetadataByConnection(connectionName string, keyword string) QueryMetadataResponse {
	return SearchQueryMetadataByConnectionWithUser(0, "admin", connectionName, keyword)
}

// searchMySQLMetadata 查询当前 MySQL 库中的表和字段。
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

// searchOracleMetadata 查询当前 Oracle 用户或指定 schema 下可见的表和字段。
func searchOracleMetadata(db *sql.DB, schemaName string, keyword string) ([]MetadataTable, []MetadataColumn, error) {
	schemaName = strings.ToUpper(strings.TrimSpace(schemaName))
	kwLike := "%"
	if keyword != "" {
		kwLike = "%" + strings.ToUpper(strings.TrimSpace(keyword)) + "%"
	}

	tableNameExpr := "obj.object_name"
	columnTableNameExpr := "col.table_name"
	tableOwnerFilter := "obj.owner = USER"
	columnOwnerFilter := "col.owner = USER"
	tableArgs := []interface{}{kwLike, kwLike}
	columnArgs := []interface{}{kwLike, kwLike, kwLike}
	if schemaName != "" {
		tableNameExpr = "obj.owner || '.' || obj.object_name"
		columnTableNameExpr = "col.owner || '.' || col.table_name"
		tableOwnerFilter = "obj.owner = :1"
		columnOwnerFilter = "col.owner = :1"
		tableArgs = []interface{}{schemaName, kwLike, kwLike}
		columnArgs = []interface{}{schemaName, kwLike, kwLike, kwLike}
	}

	tableSQL := fmt.Sprintf(`
SELECT
    %s,
    NVL(c.comments, ' ')
FROM all_objects obj
LEFT JOIN all_tab_comments c
  ON obj.owner = c.owner
 AND obj.object_name = c.table_name
WHERE obj.object_type IN ('TABLE', 'VIEW')
  AND %s
  AND (
        UPPER(obj.object_name) LIKE :%d
        OR UPPER(NVL(c.comments, ' ')) LIKE :%d
      )
ORDER BY obj.owner, obj.object_name
`, tableNameExpr, tableOwnerFilter, len(tableArgs)-1, len(tableArgs))

	columnSQL := fmt.Sprintf(`
SELECT
    %s,
    col.column_name,
    col.data_type ||
    CASE
      WHEN col.data_type IN ('VARCHAR2', 'CHAR', 'NVARCHAR2', 'NCHAR') THEN '(' || col.data_length || ')'
      WHEN col.data_type = 'NUMBER' AND col.data_precision IS NOT NULL AND col.data_scale IS NOT NULL THEN '(' || col.data_precision || ',' || col.data_scale || ')'
      WHEN col.data_type = 'NUMBER' AND col.data_precision IS NOT NULL THEN '(' || col.data_precision || ')'
      ELSE ''
    END AS column_type,
    NVL(cc.comments, ' ')
FROM all_tab_columns col
LEFT JOIN all_col_comments cc
  ON col.owner = cc.owner
 AND col.table_name = cc.table_name
 AND col.column_name = cc.column_name
WHERE %s
  AND (
        UPPER(col.table_name) LIKE :%d
        OR UPPER(col.column_name) LIKE :%d
        OR UPPER(NVL(cc.comments, ' ')) LIKE :%d
      )
ORDER BY col.owner, col.table_name, col.column_id
`, columnTableNameExpr, columnOwnerFilter, len(columnArgs)-2, len(columnArgs)-1, len(columnArgs))

	tables, err := queryMetadataTables(db, tableSQL, tableArgs...)
	if err != nil {
		return nil, nil, err
	}

	columns, err := queryMetadataColumns(db, columnSQL, columnArgs...)
	if err != nil {
		return nil, nil, err
	}

	return tables, columns, nil
}

// searchMSSQLMetadata 查询当前 SQL Server 数据库内可见 schema 的表、视图和字段。
func searchMSSQLMetadata(db *sql.DB, keyword string) ([]MetadataTable, []MetadataColumn, error) {
	kwLike := "%"
	if keyword != "" {
		kwLike = "%" + keyword + "%"
	}

	tableSQL := `
SELECT TOP (100)
    o.name,
    COALESCE(CAST(ep.value AS NVARCHAR(4000)), '')
FROM sys.objects o
JOIN sys.schemas s
  ON s.schema_id = o.schema_id
LEFT JOIN sys.extended_properties ep
  ON ep.major_id = o.object_id
 AND ep.minor_id = 0
 AND ep.name = 'MS_Description'
WHERE o.type IN ('U', 'V')
  AND s.name = SCHEMA_NAME()
  AND (
        o.name LIKE @p1
        OR COALESCE(CAST(ep.value AS NVARCHAR(4000)), '') LIKE @p2
      )
ORDER BY o.name
`

	columnSQL := `
SELECT TOP (300)
    o.name,
    c.name,
    t.name +
      CASE
        WHEN t.name IN ('varchar', 'char', 'varbinary', 'binary') THEN '(' + IIF(c.max_length = -1, 'max', CAST(c.max_length AS VARCHAR(10))) + ')'
        WHEN t.name IN ('nvarchar', 'nchar') THEN '(' + IIF(c.max_length = -1, 'max', CAST(c.max_length / 2 AS VARCHAR(10))) + ')'
        WHEN t.name IN ('decimal', 'numeric') THEN '(' + CAST(c.precision AS VARCHAR(10)) + ',' + CAST(c.scale AS VARCHAR(10)) + ')'
        ELSE ''
      END,
    COALESCE(CAST(ep.value AS NVARCHAR(4000)), '')
FROM sys.objects o
JOIN sys.schemas s
  ON s.schema_id = o.schema_id
JOIN sys.columns c
  ON c.object_id = o.object_id
JOIN sys.types t
  ON t.user_type_id = c.user_type_id
LEFT JOIN sys.extended_properties ep
  ON ep.major_id = o.object_id
 AND ep.minor_id = c.column_id
 AND ep.name = 'MS_Description'
WHERE o.type IN ('U', 'V')
  AND s.name = SCHEMA_NAME()
  AND (
        o.name LIKE @p1
        OR c.name LIKE @p2
        OR COALESCE(CAST(ep.value AS NVARCHAR(4000)), '') LIKE @p3
      )
ORDER BY o.name, c.column_id
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

// searchPostgresMetadata 查询指定 PostgreSQL schema 中的表和字段，未填写时使用当前默认 schema。
func searchPostgresMetadata(db *sql.DB, schemaName string, keyword string) ([]MetadataTable, []MetadataColumn, error) {
	schemaName = strings.TrimSpace(schemaName)
	kwLike := "%"
	if keyword != "" {
		kwLike = "%" + keyword + "%"
	}

	tableSQL := `
SELECT
    c.relname,
    COALESCE(obj_description(c.oid, 'pg_class'), '')
FROM pg_class c
JOIN pg_namespace n
  ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f')
  AND n.nspname = COALESCE(NULLIF($1, ''), current_schema())
  AND (
        c.relname ILIKE $2
        OR COALESCE(obj_description(c.oid, 'pg_class'), '') ILIKE $3
      )
ORDER BY c.relname
LIMIT 100
`

	columnSQL := `
SELECT
    c.relname,
    a.attname,
    format_type(a.atttypid, a.atttypmod),
    COALESCE(col_description(c.oid, a.attnum), '')
FROM pg_class c
JOIN pg_namespace n
  ON n.oid = c.relnamespace
JOIN pg_attribute a
  ON a.attrelid = c.oid
WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f')
  AND n.nspname = COALESCE(NULLIF($1, ''), current_schema())
  AND a.attnum > 0
  AND NOT a.attisdropped
  AND (
        c.relname ILIKE $2
        OR a.attname ILIKE $3
        OR COALESCE(col_description(c.oid, a.attnum), '') ILIKE $4
      )
ORDER BY c.relname, a.attnum
LIMIT 300
`

	tables, err := queryMetadataTables(db, tableSQL, schemaName, kwLike, kwLike)
	if err != nil {
		return nil, nil, err
	}

	columns, err := queryMetadataColumns(db, columnSQL, schemaName, kwLike, kwLike, kwLike)
	if err != nil {
		return nil, nil, err
	}

	return tables, columns, nil
}

// queryMetadataTables 扫描表元数据，并安全处理可为空的注释字段。
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
		item.Comment = nullableString(comment)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// queryMetadataColumns 扫描字段元数据，并安全处理可为空的类型和注释字段。
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
		item.ColumnType = nullableString(columnType)
		item.Comment = nullableString(comment)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func emptyMetadataResponse(ok bool, message string) QueryMetadataResponse {
	return QueryMetadataResponse{
		OK:      ok,
		Message: message,
		Tables:  []MetadataTable{},
		Columns: []MetadataColumn{},
	}
}

func nullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}
