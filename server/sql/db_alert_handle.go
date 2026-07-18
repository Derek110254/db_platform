package sql

import (
	"database/sql"
	"errors"
	"strings"

	"sql_platform/server/config"
)

/*
db_alert_handle.go
----------------------------------------------------------------------
该文件负责数据库告警处理记录的核心逻辑处理。

主要功能：
1. 数据库告警处理记录的增、删、改、查（CRUD）。
2. 按用户角色区分操作权限，admin可管理所有，普通用户管理自己（按 handler 归属）。
3. 管理员更新处理结果（处理人、处理开始/结束时间、处理结果）。
*/

// DBAlertHandleRecord
// ------------------------------------------------------------
// 数据库告警处理记录模型
type DBAlertHandleRecord struct {
	ID              int64  `json:"id"`
	DBType          string `json:"dbType"`
	AlertLevel      string `json:"alertLevel"`
	AlertCategory   string `json:"alertCategory"`
	AlertContent    string `json:"alertContent"`
	ImpactScope     string `json:"impactScope"`
	AlertTime       string `json:"alertTime"`
	Handler         string `json:"handler"`
	HandleStartTime string `json:"handleStartTime"`
	HandleEndTime   string `json:"handleEndTime"`
	HandleResult    string `json:"handleResult"`
	CreateTime      string `json:"createTime"`
	UpdateTime      string `json:"updateTime"`
}

// CreateDBAlertHandle
// ------------------------------------------------------------
// 创建数据库告警处理记录。
func CreateDBAlertHandle(item DBAlertHandleRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	query := `
INSERT INTO platform_db_alert_handle (
	db_type, alert_level, alert_category, alert_content, impact_scope, alert_time, handler,
	handle_start_time, handle_end_time, handle_result
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
	var alertTimeVal interface{} = item.AlertTime
	if item.AlertTime == "" {
		alertTimeVal = nil
	}
	var handleStartVal interface{}
	if v := strings.TrimSpace(item.HandleStartTime); v != "" {
		handleStartVal = v
	}
	var handleEndVal interface{}
	if v := strings.TrimSpace(item.HandleEndTime); v != "" {
		handleEndVal = v
	}

	_, err = db.Exec(query,
		item.DBType, item.AlertLevel, item.AlertCategory, item.AlertContent, item.ImpactScope, alertTimeVal, item.Handler,
		handleStartVal, handleEndVal, item.HandleResult,
	)
	return err
}

// UpdateDBAlertHandle
// ------------------------------------------------------------
// 更新数据库告警处理记录。
// admin 可更新任意记录，普通用户只能更新 handler 为自己的记录。
func UpdateDBAlertHandle(item DBAlertHandleRecord, roleName string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if item.ID <= 0 {
		return errors.New("记录ID不能为空")
	}

	// 检查记录是否存在且属于当前用户
	var existHandler string
	if roleName == "admin" {
		err = db.QueryRow("SELECT handler FROM platform_db_alert_handle WHERE id = ?", item.ID).Scan(&existHandler)
	} else {
		err = db.QueryRow("SELECT handler FROM platform_db_alert_handle WHERE id = ? AND handler = ?", item.ID, item.Handler).Scan(&existHandler)
	}
	if err == sql.ErrNoRows {
		return errors.New("记录不存在或无权限编辑")
	} else if err != nil {
		return err
	}

	query := `
UPDATE platform_db_alert_handle
SET
	db_type = ?,
	alert_level = ?,
	alert_category = ?,
	alert_content = ?,
	impact_scope = ?,
	alert_time = ?,
	handle_start_time = ?,
	handle_end_time = ?,
	handle_result = ?
`
	var alertTimeVal interface{} = item.AlertTime
	if item.AlertTime == "" {
		alertTimeVal = nil
	}
	var handleStartVal interface{}
	if v := strings.TrimSpace(item.HandleStartTime); v != "" {
		handleStartVal = v
	}
	var handleEndVal interface{}
	if v := strings.TrimSpace(item.HandleEndTime); v != "" {
		handleEndVal = v
	}

	if roleName == "admin" {
		query += " WHERE id = ?"
		_, err = db.Exec(query,
			item.DBType, item.AlertLevel, item.AlertCategory, item.AlertContent, item.ImpactScope, alertTimeVal,
			handleStartVal, handleEndVal, item.HandleResult,
			item.ID,
		)
	} else {
		query += " WHERE id = ? AND handler = ?"
		_, err = db.Exec(query,
			item.DBType, item.AlertLevel, item.AlertCategory, item.AlertContent, item.ImpactScope, alertTimeVal,
			handleStartVal, handleEndVal, item.HandleResult,
			item.ID, item.Handler,
		)
	}

	return err
}

// DeleteDBAlertHandle
// ------------------------------------------------------------
// 删除数据库告警处理记录。
// admin 可删除任意记录，普通用户只能删除 handler 为自己的记录。
func DeleteDBAlertHandle(id int64, handler string, roleName string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("记录ID不能为空")
	}

	var existHandler string
	if roleName == "admin" {
		err = db.QueryRow("SELECT handler FROM platform_db_alert_handle WHERE id = ?", id).Scan(&existHandler)
	} else {
		err = db.QueryRow("SELECT handler FROM platform_db_alert_handle WHERE id = ? AND handler = ?", id, handler).Scan(&existHandler)
	}
	if err == sql.ErrNoRows {
		return errors.New("记录不存在或无权限删除")
	} else if err != nil {
		return err
	}

	var res sql.Result
	if roleName == "admin" {
		res, err = db.Exec(`DELETE FROM platform_db_alert_handle WHERE id = ?`, id)
	} else {
		res, err = db.Exec(`DELETE FROM platform_db_alert_handle WHERE id = ? AND handler = ?`, id, handler)
	}

	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("记录不存在或无权限删除")
	}

	return nil
}

// QueryDBAlertHandles
// ------------------------------------------------------------
// 分页查询数据库告警处理记录，按告警时间倒序。
// handler 为空时返回全部（admin 场景）；dbType / alertLevel / alertCategory 为可选过滤条件。
func QueryDBAlertHandles(page, pageSize int, handler, dbType, alertLevel, alertCategory string) (int64, []DBAlertHandleRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return 0, nil, err
	}

	baseWhere := "WHERE 1=1"
	var args []interface{}

	if handler = strings.TrimSpace(handler); handler != "" {
		baseWhere += " AND handler = ?"
		args = append(args, handler)
	}
	if dbType = strings.TrimSpace(dbType); dbType != "" {
		baseWhere += " AND db_type = ?"
		args = append(args, dbType)
	}
	if alertLevel = strings.TrimSpace(alertLevel); alertLevel != "" {
		baseWhere += " AND alert_level = ?"
		args = append(args, alertLevel)
	}
	if alertCategory = strings.TrimSpace(alertCategory); alertCategory != "" {
		baseWhere += " AND alert_category = ?"
		args = append(args, alertCategory)
	}

	countQuery := "SELECT COUNT(1) FROM platform_db_alert_handle " + baseWhere
	var total int64
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return 0, nil, err
	}

	query := `
SELECT
	id, db_type, alert_level, alert_category, alert_content, impact_scope, alert_time, handler,
	handle_start_time, handle_end_time, handle_result, create_time, update_time
FROM platform_db_alert_handle
` + baseWhere + ` ORDER BY alert_time DESC, id DESC LIMIT ? OFFSET ?`

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var items []DBAlertHandleRecord
	for rows.Next() {
		var item DBAlertHandleRecord
		var alertTime, handleStartTime, handleEndTime sql.NullString
		if err := rows.Scan(
			&item.ID, &item.DBType, &item.AlertLevel, &item.AlertCategory, &item.AlertContent, &item.ImpactScope, &alertTime, &item.Handler,
			&handleStartTime, &handleEndTime, &item.HandleResult, &item.CreateTime, &item.UpdateTime,
		); err != nil {
			return 0, nil, err
		}
		if alertTime.Valid {
			item.AlertTime = alertTime.String
		}
		if handleStartTime.Valid {
			item.HandleStartTime = handleStartTime.String
		}
		if handleEndTime.Valid {
			item.HandleEndTime = handleEndTime.String
		}
		items = append(items, item)
	}

	return total, items, nil
}

// UpdateDBAlertHandleResult
// ------------------------------------------------------------
// 管理员更新告警处理记录的处理信息（处理人、处理开始/结束时间、处理结果）。
func UpdateDBAlertHandleResult(id int64, handler, handleStartTime, handleEndTime, handleResult string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("记录ID不能为空")
	}

	var handleStartVal interface{}
	if v := strings.TrimSpace(handleStartTime); v != "" {
		handleStartVal = v
	}
	var handleEndVal interface{}
	if v := strings.TrimSpace(handleEndTime); v != "" {
		handleEndVal = v
	}

	query := `
UPDATE platform_db_alert_handle
SET handler = ?,
    handle_start_time = ?,
    handle_end_time = ?,
    handle_result = ?
WHERE id = ?
`
	res, err := db.Exec(query, handler, handleStartVal, handleEndVal, handleResult, id)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("记录不存在")
	}

	return nil
}
