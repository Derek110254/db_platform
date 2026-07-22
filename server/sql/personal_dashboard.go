package sql

import (
	"fmt"
	"time"

	"sql_platform/server/config"
)

/*
personal_dashboard.go
----------------------------------------------------------------------
该文件负责个人看板的数据查询逻辑，统计当前用户当年的工作量和待办事项。

主要功能：
1. GetPersonalDashboard：获取当前用户的个人看板数据（数字卡片 + 待办事项）。
*/

// PersonalDashboard 个人看板数据
type PersonalDashboard struct {
	Ok bool `json:"ok"`

	// 当年工作量
	SqlAuditCount    int `json:"sqlAuditCount"`    // SQL审核次数
	ChangeProdCount  int `json:"changeProdCount"`  // 变更处理次数（只统计生产）
	SyncHandleCount  int `json:"syncHandleCount"`  // 数据同步处理次数
	AlertHandleCount int `json:"alertHandleCount"` // 告警处理次数
	OpsChangeCount   int `json:"opsChangeCount"`   // 运维变更次数

	// 待办事项
	PendingAudit     int `json:"pendingAudit"`     // 待审核的SQL
	PendingChange    int `json:"pendingChange"`    // 待处理的数据库变更
	PendingSync      int `json:"pendingSync"`      // 待处理的数据库同步
	PendingAlert     int `json:"pendingAlert"`     // 待处理的告警
	PendingOpsReview int `json:"pendingOpsReview"` // 待复核的运维变更
}

// GetPersonalDashboard
// ------------------------------------------------------------
// 获取当前用户的个人看板数据。
// userID: 当前登录用户ID
// username: 当前登录用户名（用于 applicant/handler/operator 匹配）
func GetPersonalDashboard(userID int64, username string) PersonalDashboard {
	db, err := config.GetPlatformDB()
	if err != nil {
		return PersonalDashboard{Ok: false}
	}

	var result PersonalDashboard
	result.Ok = true
	yearStart := fmt.Sprintf("%d-01-01 00:00:00", time.Now().Year())

	// ========== 当年工作量 ==========

	// SQL审核次数：当前用户提交的SQL审核记录
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_sql_audit WHERE user_id = ? AND create_time >= ?",
		userID, yearStart,
	).Scan(&result.SqlAuditCount)

	// 变更处理次数（只统计生产）：当前用户作为生产线发布人或验证人的变更记录
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_db_change_request WHERE (prod_publisher = ? OR release_verifier = ?) AND create_time >= ?",
		username, username, yearStart,
	).Scan(&result.ChangeProdCount)

	// 数据同步处理次数：当前用户作为执行DBA的同步记录
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_db_data_sync_request WHERE execute_dba = ? AND create_time >= ?",
		username, yearStart,
	).Scan(&result.SyncHandleCount)

	// 告警处理次数：当前用户处理的告警记录
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_db_alert_handle WHERE handler = ? AND create_time >= ?",
		username, yearStart,
	).Scan(&result.AlertHandleCount)

	// 运维变更次数：当前用户操作的运维变更记录
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_ops_change_record WHERE operator = ? AND create_time >= ?",
		username, yearStart,
	).Scan(&result.OpsChangeCount)

	// ========== 待办事项 ==========

	// 待审核的SQL：当前用户提交的、已提交但未审核的SQL
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_sql_audit WHERE user_id = ? AND submit_audit > 0 AND audit_passed = 0",
		userID,
	).Scan(&result.PendingAudit)

	// 待处理的数据库变更：当前用户提交的、未完成验证的变更申请
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_db_change_request WHERE applicant = ? AND (release_verifier = '' OR release_verifier IS NULL)",
		username,
	).Scan(&result.PendingChange)

	// 待处理的数据库同步：当前用户提交的、未执行的同步申请
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_db_data_sync_request WHERE applicant = ? AND (execute_dba = '' OR execute_dba IS NULL)",
		username,
	).Scan(&result.PendingSync)

	// 待处理的告警：当前用户处理的、未完成的告警记录
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_db_alert_handle WHERE handler = ? AND handle_end_time IS NULL",
		username,
	).Scan(&result.PendingAlert)

	// 待复核的运维变更：当前用户操作的、待复核的运维变更
	db.QueryRow(
		"SELECT COUNT(1) FROM platform_ops_change_record WHERE operator = ? AND change_result = '待复核'",
		username,
	).Scan(&result.PendingOpsReview)

	return result
}
