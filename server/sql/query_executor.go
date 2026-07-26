package sql

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/sijms/go-ora/v2"
	"github.com/xuri/excelize/v2"
)

/*
query_executor.go
----------------------------------------------------------------------
负责只读 SQL 查询执行和 Excel 导出。

核心规则：
1. 每次查询都按当前用户校验数据库连接权限。
2. 普通用户只能查询管理员授权的连接。
3. 管理员可以查询全部启用连接。
4. 只允许单条 SELECT / WITH 查询。
5. 执行前阻止显式跨 MySQL 数据库、PostgreSQL/Oracle schema、MSSQL 数据库查询。
6. 在数据库执行层限制最多返回 500 行。
7. 支持 MySQL、Oracle、PostgreSQL 和 MSSQL，并使用各自的行数限制语法。
8. Excel 导出复用同一权限与范围校验，并将值按字符串写入以避免长数字失真。
*/

const maxQueryRows = 500

// QueryExecuteResponse 是查询接口的统一返回结构。
type QueryExecuteResponse struct {
	OK        bool                     `json:"ok"`        // 是否成功
	Message   string                   `json:"message"`   // 返回消息
	Columns   []string                 `json:"columns"`   // 列名列表
	Rows      []map[string]interface{} `json:"rows"`      // 数据行
	RowCount  int                      `json:"rowCount"`  // 返回行数
	ElapsedMs int64                    `json:"elapsedMs"` // 执行耗时，单位毫秒
}

// ExecuteQueryByConnectionWithContext 依次完成连接授权、SQL 类型、查询范围和 500 行限制后执行查询。
func ExecuteQueryByConnectionWithContext(
	ctx context.Context,
	userID int64,
	roleName string,
	connectionName string,
	sqlText string,
) QueryExecuteResponse {
	start := time.Now()

	canAccess, err := UserCanAccessConnection(userID, roleName, connectionName)
	if err != nil {
		return queryError(start, err.Error())
	}
	if !canAccess {
		return queryError(start, "当前用户无权访问该数据库连接")
	}

	conn, err := LoadConnectionByName(connectionName)
	if err != nil {
		return queryError(start, err.Error())
	}

	rawSQL := strings.TrimSpace(sqlText)
	if err := validateQuerySQL(rawSQL); err != nil {
		return queryError(start, err.Error())
	}
	if err := validateQueryScope(conn, rawSQL); err != nil {
		return queryError(start, err.Error())
	}

	limitedSQL := buildLimitedQuerySQL(conn.DBType, rawSQL, maxQueryRows)

	db, err := openDBByConnectionRecord(conn)
	if err != nil {
		return queryError(start, "数据库连接失败: "+err.Error())
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, limitedSQL)
	if err != nil {
		return queryError(start, err.Error())
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return queryError(start, err.Error())
	}

	resultRows := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return queryError(start, err.Error())
		}

		rowMap := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			rowMap[col] = normalizeDBValue(values[i])
		}
		resultRows = append(resultRows, rowMap)
	}

	if err := rows.Err(); err != nil {
		return queryError(start, err.Error())
	}

	return QueryExecuteResponse{
		OK:        true,
		Message:   fmt.Sprintf("查询成功，最多返回 %d 行", maxQueryRows),
		Columns:   columns,
		Rows:      resultRows,
		RowCount:  len(resultRows),
		ElapsedMs: time.Since(start).Milliseconds(),
	}
}

func queryError(start time.Time, message string) QueryExecuteResponse {
	return QueryExecuteResponse{
		OK:        false,
		Message:   message,
		Columns:   []string{},
		Rows:      []map[string]interface{}{},
		RowCount:  0,
		ElapsedMs: time.Since(start).Milliseconds(),
	}
}

// ExportQueryResultToExcelWithContext 按用户权限导出查询结果。
// 导出内容与查询接口一致，仍然受 500 行上限保护。
func ExportQueryResultToExcelWithContext(
	ctx context.Context,
	userID int64,
	roleName string,
	connectionName string,
	sqlText string,
) (string, []byte, error) {
	result := ExecuteQueryByConnectionWithContext(ctx, userID, roleName, connectionName, sqlText)
	if !result.OK {
		return "", nil, errors.New(result.Message)
	}

	f := excelize.NewFile()
	sheetName := "QueryResult"
	f.SetSheetName("Sheet1", sheetName)

	for i, col := range result.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheetName, cell, col)
	}

	for rowIndex, row := range result.Rows {
		for colIndex, colName := range result.Columns {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			_ = f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", row[colName]))
		}
	}

	for i := range result.Columns {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheetName, colName, colName, 20)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return "", nil, err
	}

	fileName := fmt.Sprintf("query_result_%s.xlsx", time.Now().Format("20060102150405"))
	return fileName, buf.Bytes(), nil
}

// openDBByConnectionRecord 根据连接配置打开真实目标库连接。
func openDBByConnectionRecord(record DBConnectionRecord) (*sql.DB, error) {
	dbType := strings.ToLower(strings.TrimSpace(record.DBType))

	plainPassword, err := GetConnectionPlainPassword(record)
	if err != nil {
		return nil, err
	}

	switch dbType {
	case "mysql":
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			record.Username,
			plainPassword,
			record.Host,
			record.Port,
			record.DatabaseName,
		)
		return openAndPing("mysql", dsn)

	case "oracle":
		dsn := fmt.Sprintf(
			"oracle://%s:%s@%s:%d/%s",
			record.Username,
			plainPassword,
			record.Host,
			record.Port,
			record.ServiceName,
		)
		return openAndPing("oracle", dsn)

	case "postgres":
		dsn := buildPostgresDSN(record.Username, plainPassword, record.Host, record.Port, record.DatabaseName, record.ServiceName)
		return openAndPing("postgres", dsn)

	case "mssql":
		dsn := buildMSSQLDSN(record.Username, plainPassword, record.Host, record.Port, record.DatabaseName)
		return openAndPing("sqlserver", dsn)

	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", record.DBType)
	}
}

// buildPostgresDSN 连接到指定数据库；填写 schema 时通过 search_path 固定默认 schema。
func buildPostgresDSN(username string, password string, host string, port int, databaseName string, schemaName string) string {
	values := url.Values{}
	values.Set("sslmode", "disable")
	if strings.TrimSpace(schemaName) != "" {
		values.Set("search_path", strings.TrimSpace(schemaName))
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?%s",
		url.QueryEscape(username),
		url.QueryEscape(password),
		host,
		port,
		url.PathEscape(strings.TrimSpace(databaseName)),
		values.Encode(),
	)
}

// buildMSSQLDSN 连接到指定 SQL Server 数据库，连接方式与 MySQL 一样只依赖数据库名。
func buildMSSQLDSN(username string, password string, host string, port int, databaseName string) string {
	values := url.Values{}
	values.Set("database", strings.TrimSpace(databaseName))
	values.Set("encrypt", "disable")

	return fmt.Sprintf(
		"sqlserver://%s:%s@%s:%d?%s",
		url.QueryEscape(username),
		url.QueryEscape(password),
		host,
		port,
		values.Encode(),
	)
}

func openAndPing(driverName string, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// validateQuerySQL 校验 SQL 是否是单条只读查询。
func validateQuerySQL(sqlText string) error {
	text := strings.TrimSpace(sqlText)
	if text == "" {
		return errors.New("SQL 不能为空")
	}

	text = strings.TrimSpace(strings.TrimSuffix(text, ";"))
	lower := strings.ToLower(text)

	if !(strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with")) {
		return errors.New("只允许执行查询语句，仅支持 SELECT 或 WITH 开头")
	}
	if strings.Contains(text, ";") {
		return errors.New("不允许执行多条 SQL 语句")
	}

	return nil
}

// buildLimitedQuerySQL 为查询 SQL 添加数据库层行数限制。
func buildLimitedQuerySQL(dbType string, rawSQL string, limit int) string {
	sqlText := strings.TrimSpace(strings.TrimSuffix(rawSQL, ";"))
	dbType = strings.ToLower(strings.TrimSpace(dbType))

	switch dbType {
	case "oracle":
		return fmt.Sprintf("SELECT * FROM (\n%s\n) WHERE ROWNUM <= %d", sqlText, limit)
	case "mssql":
		return fmt.Sprintf("SELECT TOP (%d) * FROM (\n%s\n) AS query_result_limit_alias", limit, sqlText)
	default:
		return fmt.Sprintf("SELECT * FROM (\n%s\n) AS query_result_limit_alias LIMIT %d", sqlText, limit)
	}
}

// normalizeDBValue 规范化数据库返回值，避免长数字和时间类型在前端展示时失真。
func normalizeDBValue(v interface{}) interface{} {
	switch value := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(value)
	case time.Time:
		return value.Format("2006-01-02 15:04:05")
	default:
		return value
	}
}
