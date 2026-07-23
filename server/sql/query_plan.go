package sql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"sql_platform/server/auth"
	"sql_platform/server/config"
)

/*
query_plan.go
----------------------------------------------------------------------
该文件负责获取数据库的执行计划，并调用 AI（通义千问）进行深度解读。

主要功能：
1. 根据用户权限校验数据库连接访问权。
2. 支持 MySQL、PostgreSQL 和 Oracle 数据库的执行计划获取（所有语句均可检测性能）。
3. 调用通义千问 API 解读执行计划，给出优化建议及 SQL 性能评分。
4. 记录 SQL 执行审计日志。
*/

// --- AI API 相关的结构体定义 ---

// QwenRequest 对应 OpenAI 兼容的 chat/completions 请求格式
type QwenRequest struct {
	Model    string        `json:"model"`
	Messages []QwenMessage `json:"messages"`
}

// QwenMessage 对应 OpenAI 兼容格式中的一条对话消息（system/user/assistant）。
type QwenMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// QwenResponse 对应 OpenAI 兼容的 chat/completions 响应格式
type QwenResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// SqlAuditRecord 表示一条 SQL 执行审计记录，用于落库 SQL 文本、AI 建议与性能评分。
type SqlAuditRecord struct {
	UserID         int64  `json:"user_id"`
	ConnectionName string `json:"connection_name"`
	SqlText        string `json:"sql_text"`
	AiSuggestion   string `json:"ai_suggestion"`
	SqlDigest      string `json:"sqlDigest"` // 新增：指纹字段
	AiScore        int    `json:"ai_score"`  // 新增评分字段
}

// --- 核心逻辑函数 ---

/*
ExplainQueryByConnectionWithContext
获取 SQL 执行计划并调用 AI (通义千问) 进行深度解读

执行流程：
1. 校验当前用户是否具有目标连接的访问权限。
2. 拦截非查询类语句，防止越权执行写操作。
3. 根据不同的数据库类型（Oracle、MySQL、PostgreSQL）构造并执行 EXPLAIN 获取原始执行计划。
4. 将原始执行计划文本化，并调用大模型进行解读。
5. 保存执行的 SQL 和 AI 的分析建议及打分到审计日志中。
*/

func ExplainQueryByConnectionWithContext(
	ctx context.Context,
	userID int64,
	roleName string,
	connectionName string,
	sqlText string,
) QueryExecuteResponse {
	start := time.Now()

	// 1. 权限校验
	canAccess, err := auth.UserCanAccessConnection(userID, roleName, connectionName)
	if err != nil {
		return failPlanResponse(err.Error(), start)
	}
	if !canAccess {
		return failPlanResponse("当前用户无权访问该数据库连接", start)
	}

	// 2. 读取连接配置
	conn, err := auth.LoadConnectionByName(connectionName)
	if err != nil {
		return failPlanResponse(err.Error(), start)
	}

	// 3. 校验 SQL：非空、单语句、仅允许 DML/DQL（增删改查）
	rawSQL := strings.TrimSpace(sqlText)
	if rawSQL == "" {
		return failPlanResponse("SQL 不能为空", start)
	}
	rawSQL = strings.TrimSuffix(rawSQL, ";")
	if strings.Contains(rawSQL, ";") {
		return failPlanResponse("不允许执行多条 SQL 语句", start)
	}
	// 仅允许 DQL（SELECT/WITH）和 DML（INSERT/UPDATE/DELETE），不允许 DDL
	lower := strings.ToLower(strings.TrimSpace(rawSQL))
	if !(strings.HasPrefix(lower, "select") ||
		strings.HasPrefix(lower, "with") ||
		strings.HasPrefix(lower, "insert") ||
		strings.HasPrefix(lower, "update") ||
		strings.HasPrefix(lower, "delete")) {
		return failPlanResponse("仅支持检测 DML/DQL 语句（SELECT、INSERT、UPDATE、DELETE）的性能，不支持 DDL 语句", start)
	}

	// 4. 打开目标数据库连接
	db, err := openDBByConnectionRecord(conn)
	if err != nil {
		return failPlanResponse("数据库连接失败："+err.Error(), start)
	}
	defer db.Close()

	// 5. 根据数据库类型构造并执行 EXPLAIN
	dbType := strings.ToLower(strings.TrimSpace(conn.DBType))
	var rawResult QueryExecuteResponse

	if dbType == "oracle" {
		// Oracle 逻辑：先生成计划到系统表，再查询系统表
		planSQL := fmt.Sprintf("EXPLAIN PLAN FOR %s", strings.TrimSuffix(rawSQL, ";"))
		_, err = db.ExecContext(ctx, planSQL)
		if err != nil {
			return failPlanResponse("生成执行计划失败(Oracle)："+err.Error(), start)
		}
		rawResult = fetchPlanResult(ctx, db, "SELECT * FROM TABLE(DBMS_XPLAN.DISPLAY)", start)
	} else {
		// MySQL/PostgreSQL 逻辑：直接执行 EXPLAIN
		planSQL := fmt.Sprintf("EXPLAIN %s", strings.TrimSuffix(rawSQL, ";"))
		rawResult = fetchPlanResult(ctx, db, planSQL, start)
	}

	// 检查数据库返回的原始计划是否成功
	if !rawResult.OK {
		return rawResult
	}

	// 6. 将原始计划转换为 AI 可读的文本
	planText := convertPlanToString(rawResult.Rows)

	// 7. 调用通义千问 API 进行解读
	aiInterpretation, score, err := callQwenInterpret(rawSQL, planText) // 接收 score
	if err != nil {
		return failPlanResponse("AI 解读失败: "+err.Error(), start)
	}

	// ================= 新增：异步或同步写入审核记录 =================
	digest := auth.GenerateSQLDigest(sqlText)
	// 执行保存操作
	// 构造审计记录
	auditRecord := auth.SqlAuditRecord{
		UserID:         userID, // 传入 SessionUser 中的 UserID
		ConnectionName: connectionName,
		SqlText:        sqlText,
		SqlDigest:      digest,
		ExecutionPlan:  planText,
		AiSuggestion:   aiInterpretation,
		AiScore:        score,
	}

	// 同步保存记录以获取 ID
	var auditID int64
	if id, err := auth.SaveSqlAuditRecord(auditRecord); err != nil {
		// 记录错误日志
		log.Printf("[AuditError] Failed to save record for user %d: %v", userID, err)
	} else {
		auditID = id
	}

	// 8. 封装最终响应结果
	return QueryExecuteResponse{
		OK:        true,
		Message:   aiInterpretation,
		Score:     score,   // 传回分数
		AuditID:   auditID, // 新增传回 AuditID
		Columns:   rawResult.Columns,
		Rows:      rawResult.Rows,
		RowCount:  len(rawResult.Rows),
		ElapsedMs: time.Since(start).Milliseconds(),
	}
}

// --- 辅助工具函数 ---

// convertPlanToString 将数据库返回的 map 数组转换成格式化的字符串
func convertPlanToString(rows []map[string]interface{}) string {
	var sb strings.Builder
	for i, row := range rows {
		sb.WriteString(fmt.Sprintf("节点 %d: ", i+1))
		for k, v := range row {
			sb.WriteString(fmt.Sprintf("[%s: %v] ", k, v))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// callQwenInterpret 封装通义千问 API 请求逻辑
func callQwenInterpret(sqlText, planText string) (string, int, error) {
	// 获取 API Key (统一配置在 platform_db.go 中)
	apiKey := config.QwenAPIKey
	if apiKey == "" {
		return "", 0, fmt.Errorf("未配置 DASHSCOPE_API_KEY")
	}

	// apiUrl := "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"
	apiUrl := "https://dd-ai-api.eastmoney.com/v1/chat/completions" // 内部API

	// 构造 Prompt，明确指定专家身份和分析重点
	prompt := fmt.Sprintf(`你是一位资深的 DBA 专家。请简洁地解读以下 SQL 执行计划。由于目前在测试环境，尽量忽略估计的行数。并为该 SQL 的执行计划性能打分（0-100分），
    格式必须为 "评分：[数字]"。并指出最严重的 1-2 个性能瓶颈并给出改进后的 SQL 建议，字数控制在 200 字以内。
	
	[待分析 SQL]:
	%s

	[执行计划原文]:
	%s`, sqlText, planText)

	// 构造请求数据（OpenAI 兼容格式）
	var reqBody QwenRequest
	reqBody.Model = "DeepSeek-V4-Flash"
	reqBody.Messages = []QwenMessage{
		{Role: "system", Content: "你是一个专业的数据库性能分析助手。"},
		{Role: "user", Content: prompt},
	}

	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	// 设置 30 秒超时，因为 AI 推理较慢
	client := &http.Client{Timeout: 600 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("API 返回异常: %s", string(bodyBytes))
	}

	var qwenResp QwenResponse
	if err := json.Unmarshal(bodyBytes, &qwenResp); err != nil {
		return "", 0, err
	}

	// 检查错误响应
	if qwenResp.Error.Message != "" {
		return "", 0, fmt.Errorf("API 错误: %s", qwenResp.Error.Message)
	}

	if len(qwenResp.Choices) == 0 {
		return "", 0, fmt.Errorf("API 返回结果为空")
	}

	// 在获取到 AI 的 content 后，提取分数
	content := qwenResp.Choices[0].Message.Content
	score := 0

	// 使用正则匹配“评分：85”或“评分: 85”
	re := regexp.MustCompile(`评分[：:]\s*(\d+)`)
	match := re.FindStringSubmatch(content)
	if len(match) > 1 {
		fmt.Sscanf(match[1], "%d", &score)
		// 可选：把“评分：XX”从正文中去掉，让展示更干净
		content = re.ReplaceAllString(content, "")
	}

	return strings.TrimSpace(content), score, nil

}

// failPlanResponse 统一的失败响应构造器
func failPlanResponse(msg string, start time.Time) QueryExecuteResponse {
	return QueryExecuteResponse{
		OK:        false,
		Message:   msg,
		Columns:   []string{},
		Rows:      []map[string]interface{}{},
		RowCount:  0,
		ElapsedMs: time.Since(start).Milliseconds(),
	}
}

// fetchPlanResult 执行查询并封装结果 (复用原有逻辑)
func fetchPlanResult(ctx context.Context, db *sql.DB, query string, start time.Time) QueryExecuteResponse {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return failPlanResponse(err.Error(), start)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return failPlanResponse(err.Error(), start)
	}

	resultRows := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return failPlanResponse(err.Error(), start)
		}

		rowMap := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			rowMap[col] = normalizeDBValue(values[i])
		}
		resultRows = append(resultRows, rowMap)
	}

	return QueryExecuteResponse{
		OK:        true,
		Message:   "获取执行计划成功",
		Columns:   columns,
		Rows:      resultRows,
		RowCount:  len(resultRows),
		ElapsedMs: time.Since(start).Milliseconds(),
	}
}

// ManualExplainQuery
// ------------------------------------------------------------
// 手动提交 SQL 执行计划进行 AI 性能检测。
//
// 用于数据库不可自动连接（can_connect=0）的场景：
// 用户在自己的数据库客户端手动执行 EXPLAIN，将输出粘贴提交。
//
// 流程：
// 1. 校验当前用户是否具有目标连接的访问权限。
// 2. 校验 SQL 仅允许 DML/DQL 语句。
// 3. 调用 AI（通义千问）解读用户手动提交的执行计划。
// 4. 保存审核记录（含手动提交的执行计划文本）。
func ManualExplainQuery(
	userID int64,
	roleName string,
	connectionName string,
	sqlText string,
	executionPlan string,
) QueryExecuteResponse {
	start := time.Now()

	// 1. 权限校验
	canAccess, err := auth.UserCanAccessConnection(userID, roleName, connectionName)
	if err != nil {
		return failPlanResponse(err.Error(), start)
	}
	if !canAccess {
		return failPlanResponse("当前用户无权访问该数据库连接", start)
	}

	// 2. 校验 SQL：非空、单语句、仅允许 DML/DQL
	rawSQL := strings.TrimSpace(sqlText)
	if rawSQL == "" {
		return failPlanResponse("SQL 不能为空", start)
	}
	rawSQL = strings.TrimSuffix(rawSQL, ";")
	if strings.Contains(rawSQL, ";") {
		return failPlanResponse("不允许执行多条 SQL 语句", start)
	}
	lower := strings.ToLower(strings.TrimSpace(rawSQL))
	if !(strings.HasPrefix(lower, "select") ||
		strings.HasPrefix(lower, "with") ||
		strings.HasPrefix(lower, "insert") ||
		strings.HasPrefix(lower, "update") ||
		strings.HasPrefix(lower, "delete")) {
		return failPlanResponse("仅支持检测 DML/DQL 语句（SELECT、INSERT、UPDATE、DELETE）的性能，不支持 DDL 语句", start)
	}

	// 校验执行计划非空
	planText := strings.TrimSpace(executionPlan)
	if planText == "" {
		return failPlanResponse("执行计划不能为空，请粘贴 EXPLAIN 的输出", start)
	}

	// 3. 调用 AI 解读
	aiInterpretation, score, err := callQwenInterpret(rawSQL, planText)
	if err != nil {
		return failPlanResponse("AI 解读失败: "+err.Error(), start)
	}

	// 4. 保存审核记录
	digest := auth.GenerateSQLDigest(sqlText)
	auditRecord := auth.SqlAuditRecord{
		UserID:         userID,
		ConnectionName: connectionName,
		SqlText:        sqlText,
		SqlDigest:      digest,
		ExecutionPlan:  planText,
		AiSuggestion:   aiInterpretation,
		AiScore:        score,
	}

	var auditID int64
	if id, err := auth.SaveSqlAuditRecord(auditRecord); err != nil {
		log.Printf("[AuditError] Failed to save manual record for user %d: %v", userID, err)
	} else {
		auditID = id
	}

	// 5. 封装响应
	return QueryExecuteResponse{
		OK:        true,
		Message:   aiInterpretation,
		Score:     score,
		AuditID:   auditID,
		Columns:   []string{},
		Rows:      []map[string]interface{}{},
		RowCount:  0,
		ElapsedMs: time.Since(start).Milliseconds(),
	}
}
