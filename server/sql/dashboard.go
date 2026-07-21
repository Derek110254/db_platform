package sql

import (
	"database/sql"
	"fmt"
	"time"

	"sql_platform/server/config"
)

/*
dashboard.go
----------------------------------------------------------------------
该文件负责首页看板和年度工作量统计的数据查询逻辑。

主要功能：
1. GetDashboardOverview：首页概览（用户数、连接数、今日检测数、待办数等）。
2. GetYearlyDashboard：年度工作量统计（数字卡片 + 月度趋势 + 分类分布 + 用户排名）。
*/

// NameValue 通用的名称-数值对
type NameValue struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// MonthItem 月度统计项
type MonthItem struct {
	Month     string `json:"month"`
	SqlAudit  int    `json:"sqlAudit"`
	ChangeReq int    `json:"changeReq"`
	Alert     int    `json:"alert"`
	OpsChange int    `json:"opsChange"`
}

// DashboardOverview 首页概览
type DashboardOverview struct {
	Ok                  bool `json:"ok"`
	UserCount           int  `json:"userCount"`
	ConnCount           int  `json:"connCount"`
	TodayAudit          int  `json:"todayAudit"`
	PendingAudit        int  `json:"pendingAudit"`
	ChangeReqCount      int  `json:"changeReqCount"`
	PendingVerifyChange int  `json:"pendingVerifyChange"`
	AlertCount          int  `json:"alertCount"`
	OpsChangeCount      int  `json:"opsChangeCount"`
	PendingReviewOps    int  `json:"pendingReviewOps"`
}

// GetDashboardOverview
// ------------------------------------------------------------
// 获取首页概览统计数据（白名单，无需登录）。
func GetDashboardOverview() DashboardOverview {
	db, err := config.GetPlatformDB()
	if err != nil {
		return DashboardOverview{Ok: false}
	}

	var result DashboardOverview
	result.Ok = true

	db.QueryRow("SELECT COUNT(1) FROM platform_user WHERE is_deleted = 0").Scan(&result.UserCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_db_connection WHERE is_enabled = 1").Scan(&result.ConnCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_sql_audit WHERE DATE(create_time) = CURDATE()").Scan(&result.TodayAudit)
	db.QueryRow("SELECT COUNT(1) FROM platform_sql_audit WHERE submit_audit > 0 AND audit_passed = 0").Scan(&result.PendingAudit)
	db.QueryRow("SELECT COUNT(1) FROM platform_db_change_request").Scan(&result.ChangeReqCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_db_change_request WHERE release_verifier = '' OR release_verifier IS NULL").Scan(&result.PendingVerifyChange)
	db.QueryRow("SELECT COUNT(1) FROM platform_db_alert_handle").Scan(&result.AlertCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_ops_change_record").Scan(&result.OpsChangeCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_ops_change_record WHERE change_result = '待复核'").Scan(&result.PendingReviewOps)

	return result
}

// YearlyDashboard 年度工作量统计
type YearlyDashboard struct {
	Ok               bool        `json:"ok"`
	Cards            interface{} `json:"cards"`
	Monthly          []MonthItem `json:"monthly"`
	AlertCategories  []NameValue `json:"alertCategories"`
	OpsChangeTypes   []NameValue `json:"opsChangeTypes"`
	OpsChangeResults []NameValue `json:"opsChangeResults"`
	TopUsers         []NameValue `json:"topUsers"`
}

// yearlyCards 数字卡片
type yearlyCards struct {
	SqlAuditCount       int `json:"sqlAuditCount"`
	AuditSubmitCount    int `json:"auditSubmitCount"`
	AuditPassedCount    int `json:"auditPassedCount"`
	ChangeReqCount      int `json:"changeReqCount"`
	VerifiedChangeCount int `json:"verifiedChangeCount"`
	SyncReqCount        int `json:"syncReqCount"`
	AlertHandleCount    int `json:"alertHandleCount"`
	OpsChangeCount      int `json:"opsChangeCount"`
}

// GetYearlyDashboard
// ------------------------------------------------------------
// 获取年度工作量统计数据（需登录）。
func GetYearlyDashboard() YearlyDashboard {
	db, err := config.GetPlatformDB()
	if err != nil {
		return YearlyDashboard{Ok: false}
	}

	var result YearlyDashboard
	result.Ok = true
	yearStart := fmt.Sprintf("%d-01-01 00:00:00", time.Now().Year())

	// log.Println(yearStart)
	// ========== 数字卡片 ==========
	var cards yearlyCards
	db.QueryRow("SELECT COUNT(1) FROM platform_sql_audit WHERE create_time >= ?", yearStart).Scan(&cards.SqlAuditCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_sql_audit WHERE submit_audit > 0 AND create_time >= ?", yearStart).Scan(&cards.AuditSubmitCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_sql_audit WHERE audit_passed = 1 AND create_time >= ?", yearStart).Scan(&cards.AuditPassedCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_db_change_request WHERE create_time >= ?", yearStart).Scan(&cards.ChangeReqCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_db_change_request WHERE release_verifier != '' AND release_verifier IS NOT NULL AND create_time >= ?", yearStart).Scan(&cards.VerifiedChangeCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_db_data_sync_request WHERE create_time >= ?", yearStart).Scan(&cards.SyncReqCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_db_alert_handle WHERE create_time >= ?", yearStart).Scan(&cards.AlertHandleCount)
	db.QueryRow("SELECT COUNT(1) FROM platform_ops_change_record WHERE create_time >= ?", yearStart).Scan(&cards.OpsChangeCount)
	result.Cards = cards

	// ========== 月度趋势 ==========
	sqlMonthly := queryMonthly(db, "SELECT MONTH(create_time), COUNT(1) FROM platform_sql_audit WHERE create_time >= ? GROUP BY MONTH(create_time)", yearStart)
	changeMonthly := queryMonthly(db, "SELECT MONTH(create_time), COUNT(1) FROM platform_db_change_request WHERE create_time >= ? GROUP BY MONTH(create_time)", yearStart)
	alertMonthly := queryMonthly(db, "SELECT MONTH(create_time), COUNT(1) FROM platform_db_alert_handle WHERE create_time >= ? GROUP BY MONTH(create_time)", yearStart)
	opsMonthly := queryMonthly(db, "SELECT MONTH(create_time), COUNT(1) FROM platform_ops_change_record WHERE create_time >= ? GROUP BY MONTH(create_time)", yearStart)

	result.Monthly = make([]MonthItem, 12)
	for m := 1; m <= 12; m++ {
		result.Monthly[m-1] = MonthItem{
			Month:     fmt.Sprintf("%d月", m),
			SqlAudit:  sqlMonthly[m],
			ChangeReq: changeMonthly[m],
			Alert:     alertMonthly[m],
			OpsChange: opsMonthly[m],
		}
	}

	// ========== 分类统计 ==========
	result.AlertCategories = queryNameValue(db, "SELECT alert_category, COUNT(1) FROM platform_db_alert_handle WHERE create_time >= ? GROUP BY alert_category", yearStart)
	result.OpsChangeTypes = queryNameValue(db, "SELECT change_type, COUNT(1) FROM platform_ops_change_record WHERE create_time >= ? GROUP BY change_type", yearStart)
	result.OpsChangeResults = queryNameValue(db, "SELECT change_result, COUNT(1) FROM platform_ops_change_record WHERE create_time >= ? GROUP BY change_result", yearStart)

	// ========== 用户排名 TOP5 ==========
	result.TopUsers = queryNameValue(db, `
SELECT u.display_name, COUNT(1)
FROM platform_sql_audit a
INNER JOIN platform_user u ON a.user_id = u.id
WHERE a.create_time >= ?
GROUP BY a.user_id, u.display_name
ORDER BY COUNT(1) DESC
LIMIT 5
`, yearStart)

	return result
}

// queryMonthly 辅助：按月查询，返回 map[月份]数量
func queryMonthly(db *sql.DB, query, yearStart string) map[int]int {
	result := map[int]int{}
	rows, err := db.Query(query, yearStart)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var m, c int
		rows.Scan(&m, &c)
		result[m] = c
	}
	return result
}

// queryNameValue 辅助：查询名称-数值对
func queryNameValue(db *sql.DB, query string, args ...interface{}) []NameValue {
	rows, err := db.Query(query, args...)
	if err != nil {
		return []NameValue{}
	}
	defer rows.Close()

	var result []NameValue
	for rows.Next() {
		var nv NameValue
		rows.Scan(&nv.Name, &nv.Value)
		result = append(result, nv)
	}
	if result == nil {
		result = []NameValue{}
	}
	return result
}
