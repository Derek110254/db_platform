package sql

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"sql_platform/server/auth"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/sijms/go-ora/v2"
	"github.com/xuri/excelize/v2"
)

/*
query_executor.go
----------------------------------------------------------------------
该文件负责数据库查询执行与 Excel 导出。

本版本包含：
1. 按当前用户权限校验 connectionName
2. 普通用户只能查询被分配给自己的连接
3. admin 默认可查询全部启用连接
4. 只允许执行 SELECT / WITH 开头的查询
5. 在数据库层直接限制最多返回 500 行
6. 支持 MySQL / Oracle 查询
7. 导出 Excel
8. 保证长数字不失真（避免被转成浮点数）
*/

const maxQueryRows = 500

// QueryExecuteResponse
// ----------------------------------------------------------------------
// 查询接口统一返回结构
type QueryExecuteResponse struct {
	OK        bool                     `json:"ok"`        // 是否成功
	Message   string                   `json:"message"`   // 返回消息
	Columns   []string                 `json:"columns"`   // 列名列表
	Rows      []map[string]interface{} `json:"rows"`      // 数据行
	RowCount  int                      `json:"rowCount"`  // 返回行数
	ElapsedMs int64                    `json:"elapsedMs"` // 执行耗时（毫秒）
	Score     int                      `json:"score"`     // 执行计划评分
}

// ExecuteQueryByConnectionWithContext
// ----------------------------------------------------------------------
// 按用户权限执行查询。
//
// 参数说明：
// - userID / roleName：当前登录用户信息，用于权限校验
// - connectionName：目标连接名称
// - sqlText：用户输入的 SQL
func ExecuteQueryByConnectionWithContext(
	ctx context.Context,
	userID int64,
	roleName string,
	connectionName string,
	sqlText string,
) QueryExecuteResponse {
	start := time.Now()

	// 1. 先校验当前用户是否有权访问该连接
	canAccess, err := auth.UserCanAccessConnection(userID, roleName, connectionName)
	if err != nil {
		return QueryExecuteResponse{
			OK:        false,
			Message:   err.Error(),
			Columns:   []string{},
			Rows:      []map[string]interface{}{},
			RowCount:  0,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}
	if !canAccess {
		return QueryExecuteResponse{
			OK:        false,
			Message:   "当前用户无权访问该数据库连接",
			Columns:   []string{},
			Rows:      []map[string]interface{}{},
			RowCount:  0,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}

	// 2. 读取连接配置
	conn, err := auth.LoadConnectionByName(connectionName)
	if err != nil {
		return QueryExecuteResponse{
			OK:        false,
			Message:   err.Error(),
			Columns:   []string{},
			Rows:      []map[string]interface{}{},
			RowCount:  0,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}

	// 3. 校验 SQL 只允许查询语句
	rawSQL := strings.TrimSpace(sqlText)
	if err := validateQuerySQL(rawSQL); err != nil {
		return QueryExecuteResponse{
			OK:        false,
			Message:   err.Error(),
			Columns:   []string{},
			Rows:      []map[string]interface{}{},
			RowCount:  0,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}

	// 4. 在数据库层直接限制最大返回 500 行
	limitedSQL := buildLimitedQuerySQL(conn.DBType, rawSQL, maxQueryRows)

	// 5. 打开目标数据库连接
	db, err := openDBByConnectionRecord(conn)
	if err != nil {
		return QueryExecuteResponse{
			OK:        false,
			Message:   "数据库连接失败：" + err.Error(),
			Columns:   []string{},
			Rows:      []map[string]interface{}{},
			RowCount:  0,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}
	defer db.Close()

	// 6. 执行查询
	rows, err := db.QueryContext(ctx, limitedSQL)
	if err != nil {
		return QueryExecuteResponse{
			OK:        false,
			Message:   err.Error(),
			Columns:   []string{},
			Rows:      []map[string]interface{}{},
			RowCount:  0,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}
	defer rows.Close()

	// 7. 读取列名
	columns, err := rows.Columns()
	if err != nil {
		return QueryExecuteResponse{
			OK:        false,
			Message:   err.Error(),
			Columns:   []string{},
			Rows:      []map[string]interface{}{},
			RowCount:  0,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}

	// 8. 逐行扫描结果
	resultRows := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return QueryExecuteResponse{
				OK:        false,
				Message:   err.Error(),
				Columns:   []string{},
				Rows:      []map[string]interface{}{},
				RowCount:  0,
				ElapsedMs: time.Since(start).Milliseconds(),
			}
		}

		rowMap := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			rowMap[col] = normalizeDBValue(values[i])
		}

		resultRows = append(resultRows, rowMap)
	}

	if err := rows.Err(); err != nil {
		return QueryExecuteResponse{
			OK:        false,
			Message:   err.Error(),
			Columns:   []string{},
			Rows:      []map[string]interface{}{},
			RowCount:  0,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
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

// ExportQueryResultToExcelWithContext
// ----------------------------------------------------------------------
// 按用户权限导出查询结果为 Excel。
// 导出的也是数据库层已经限制后的结果（最多 500 行）。
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

	// 写表头
	for i, col := range result.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheetName, cell, col)
	}

	// 写数据
	for rowIndex, row := range result.Rows {
		for colIndex, colName := range result.Columns {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			_ = f.SetCellValue(sheetName, cell, fmt.Sprintf("%v", row[colName]))
		}
	}

	// 调整列宽
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

// openDBByConnectionRecord
// ----------------------------------------------------------------------
// 根据平台配置表中的连接记录，打开真实数据库连接。
//
// 支持：
// 1. MySQL
// 2. Oracle
//
// 这里把缺失的函数直接补齐，避免 undefined: openDBByConnectionRecord
func openDBByConnectionRecord(record auth.DBConnectionRecord) (*sql.DB, error) {
	dbType := strings.ToLower(strings.TrimSpace(record.DBType))

	// 先从平台库中把数据库连接密码解密出来
	plainPassword, err := auth.GetConnectionPlainPassword(record)
	if err != nil {
		return nil, err
	}

	switch dbType {
	case "mysql":
		// MySQL DSN
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			record.Username,
			plainPassword,
			record.Host,
			record.Port,
			record.DatabaseName,
		)

		db, err := sql.Open("mysql", dsn)
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

	case "oracle":
		// go-ora DSN
		//
		// 适配 Oracle 11g：
		// 服务名模式最稳
		dsn := fmt.Sprintf(
			"oracle://%s:%s@%s:%d/%s",
			record.Username,
			plainPassword,
			record.Host,
			record.Port,
			record.ServiceName,
		)

		db, err := sql.Open("oracle", dsn)
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

	default:
		return nil, fmt.Errorf("不支持的数据库类型：%s", record.DBType)
	}
}

// validateQuerySQL
// ----------------------------------------------------------------------
// 校验 SQL 是否只允许查询语句。
// 只支持：
// - SELECT ...
// - WITH ... SELECT ...
//
// 同时禁止多语句。
func validateQuerySQL(sqlText string) error {
	text := strings.TrimSpace(sqlText)
	if text == "" {
		return errors.New("SQL 不能为空")
	}

	// 去掉末尾分号，避免单条查询误判
	text = strings.TrimSpace(strings.TrimSuffix(text, ";"))
	lower := strings.ToLower(text)

	if !(strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with")) {
		return errors.New("只允许执行查询语句：仅支持 SELECT 或 WITH 开头的查询")
	}

	// 粗略禁止多语句
	if strings.Contains(text, ";") {
		return errors.New("不允许执行多条 SQL 语句")
	}

	return nil
}

// buildLimitedQuerySQL
// ----------------------------------------------------------------------
// 给查询 SQL 增加“数据库层的最大返回行数限制”。
//
// 说明：
// 1. 不是先查全量再截断
// 2. 而是在数据库执行时就限制返回 500 行
func buildLimitedQuerySQL(dbType string, rawSQL string, limit int) string {
	sqlText := strings.TrimSpace(strings.TrimSuffix(rawSQL, ";"))
	dbType = strings.ToLower(strings.TrimSpace(dbType))

	switch dbType {
	case "oracle":
		// Oracle 11g 使用 ROWNUM 包裹
		// 加换行符防止 SQL 末尾的单行注释（如 --）将外层括号及 ROWNUM 限制条件注释掉
		return fmt.Sprintf("SELECT * FROM (\n%s\n) WHERE ROWNUM <= %d", sqlText, limit)
	default:
		// MySQL 用 LIMIT
		// 加换行符防止 SQL 末尾的单行注释（如 # 或 --）将外层括号及 LIMIT 条件注释掉
		return fmt.Sprintf("SELECT * FROM (\n%s\n) AS query_result_limit_alias LIMIT %d", sqlText, limit)
	}
}

// normalizeDBValue
// ----------------------------------------------------------------------
// 规范化数据库返回值，避免长数字失真。
//
// 关键规则：
// - []byte 统一转 string
// - time.Time 统一格式化
// - nil 转空字符串
//
// 这样像：10070010000000000171
// 不会因为被转成浮点数而失真。
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

// onlySelectRegex
// ----------------------------------------------------------------------
// 预留一个正则变量，后续若要增强 SQL 校验可继续使用。
var onlySelectRegex = regexp.MustCompile(`(?is)^\s*(select|with)\b`)
