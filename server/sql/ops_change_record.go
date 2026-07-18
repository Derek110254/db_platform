package sql

import (
	"database/sql"
	"errors"
	"strings"

	"sql_platform/server/config"
)

/*
ops_change_record.go
----------------------------------------------------------------------
该文件负责运维变更记录的核心逻辑处理。

主要功能：
1. 运维变更记录的增、删、改、查（CRUD）。
2. 操作人默认为创建人，admin 可管理所有记录。
3. 管理员可更新复核人信息。
*/

// OpsChangeRecord
// ------------------------------------------------------------
// 运维变更记录模型
type OpsChangeRecord struct {
	ID             int64  `json:"id"`
	ChangeTitle    string `json:"changeTitle"`
	ChangeType     string `json:"changeType"`
	ChangeLevel    string `json:"changeLevel"`
	ChangeContent  string `json:"changeContent"`
	ImpactScope    string `json:"impactScope"`
	ChangeIPList   string `json:"changeIpList"`
	ChangeTime     string `json:"changeTime"`
	Operator       string `json:"operator"`
	Reviewer       string `json:"reviewer"`
	ChangeResult   string `json:"changeResult"`
	RollbackPlan   string `json:"rollbackPlan"`
	RollbackStatus string `json:"rollbackStatus"`
	Remark         string `json:"remark"`
	CreateTime     string `json:"createTime"`
	UpdateTime     string `json:"updateTime"`
}

// CreateOpsChangeRecord
// ------------------------------------------------------------
// 创建运维变更记录。operator 默认为创建人。
func CreateOpsChangeRecord(item OpsChangeRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var timeVal interface{} = item.ChangeTime
	if item.ChangeTime == "" {
		timeVal = nil
	}

	query := `
INSERT INTO platform_ops_change_record (
	change_title, change_type, change_level, change_content, impact_scope, change_ip_list, change_time,
	operator, reviewer, change_result, rollback_plan, rollback_status, remark
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '待复核', ?, '待确认', ?)
`
	_, err = db.Exec(query,
		item.ChangeTitle, item.ChangeType, item.ChangeLevel, item.ChangeContent, item.ImpactScope, item.ChangeIPList, timeVal,
		item.Operator, item.Reviewer, item.RollbackPlan, item.Remark,
	)
	return err
}

// UpdateOpsChangeRecord
// ------------------------------------------------------------
// 更新运维变更记录。admin 可更新任意记录，普通用户只能更新 operator 为自己的记录。
func UpdateOpsChangeRecord(item OpsChangeRecord, roleName string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if item.ID <= 0 {
		return errors.New("记录ID不能为空")
	}

	// 检查记录是否存在且属于当前用户，并获取变更结果和回滚状态
	var existOperator, changeResult, rollbackStatus sql.NullString
	if roleName == "admin" {
		err = db.QueryRow("SELECT operator, change_result, rollback_status FROM platform_ops_change_record WHERE id = ?", item.ID).Scan(&existOperator, &changeResult, &rollbackStatus)
	} else {
		err = db.QueryRow("SELECT operator, change_result, rollback_status FROM platform_ops_change_record WHERE id = ? AND operator = ?", item.ID, item.Operator).Scan(&existOperator, &changeResult, &rollbackStatus)
	}
	if err == sql.ErrNoRows {
		return errors.New("记录不存在或无权限编辑")
	} else if err != nil {
		return err
	}

	// 已完成的记录不允许编辑
	if isOpsChangeCompleted(changeResult, rollbackStatus) {
		return errors.New("该变更记录已完成，不允许编辑")
	}

	var timeVal interface{} = item.ChangeTime
	if item.ChangeTime == "" {
		timeVal = nil
	}

	query := `
UPDATE platform_ops_change_record
SET
	change_title = ?,
	change_type = ?,
	change_level = ?,
	change_content = ?,
	impact_scope = ?,
	change_ip_list = ?,
	change_time = ?,
	rollback_plan = ?,
	remark = ?
`
	if roleName == "admin" {
		query += " WHERE id = ?"
		_, err = db.Exec(query,
			item.ChangeTitle, item.ChangeType, item.ChangeLevel, item.ChangeContent, item.ImpactScope, item.ChangeIPList, timeVal,
			item.RollbackPlan, item.Remark,
			item.ID,
		)
	} else {
		query += " WHERE id = ? AND operator = ?"
		_, err = db.Exec(query,
			item.ChangeTitle, item.ChangeType, item.ChangeLevel, item.ChangeContent, item.ImpactScope, item.ChangeIPList, timeVal,
			item.RollbackPlan, item.Remark,
			item.ID, item.Operator,
		)
	}

	return err
}

// DeleteOpsChangeRecord
// ------------------------------------------------------------
// 删除运维变更记录。admin 可删除任意记录，普通用户只能删除 operator 为自己的记录。
func DeleteOpsChangeRecord(id int64, operator string, roleName string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("记录ID不能为空")
	}

	var existOperator, changeResult, rollbackStatus sql.NullString
	if roleName == "admin" {
		err = db.QueryRow("SELECT operator, change_result, rollback_status FROM platform_ops_change_record WHERE id = ?", id).Scan(&existOperator, &changeResult, &rollbackStatus)
	} else {
		err = db.QueryRow("SELECT operator, change_result, rollback_status FROM platform_ops_change_record WHERE id = ? AND operator = ?", id, operator).Scan(&existOperator, &changeResult, &rollbackStatus)
	}
	if err == sql.ErrNoRows {
		return errors.New("记录不存在或无权限删除")
	} else if err != nil {
		return err
	}

	// 已完成的记录不允许删除
	if isOpsChangeCompleted(changeResult, rollbackStatus) {
		return errors.New("该变更记录已完成，不允许删除")
	}

	var res sql.Result
	if roleName == "admin" {
		res, err = db.Exec(`DELETE FROM platform_ops_change_record WHERE id = ?`, id)
	} else {
		res, err = db.Exec(`DELETE FROM platform_ops_change_record WHERE id = ? AND operator = ?`, id, operator)
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

// QueryOpsChangeRecords
// ------------------------------------------------------------
// 分页查询运维变更记录，按变更时间倒序。
// operator 为空时返回全部（admin 场景）；changeType / changeLevel 为可选过滤条件。
func QueryOpsChangeRecords(page, pageSize int, operator, changeType, changeLevel string) (int64, []OpsChangeRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return 0, nil, err
	}

	baseWhere := "WHERE 1=1"
	var args []interface{}

	if operator = strings.TrimSpace(operator); operator != "" {
		baseWhere += " AND operator = ?"
		args = append(args, operator)
	}
	if changeType = strings.TrimSpace(changeType); changeType != "" {
		baseWhere += " AND change_type = ?"
		args = append(args, changeType)
	}
	if changeLevel = strings.TrimSpace(changeLevel); changeLevel != "" {
		baseWhere += " AND change_level = ?"
		args = append(args, changeLevel)
	}

	countQuery := "SELECT COUNT(1) FROM platform_ops_change_record " + baseWhere
	var total int64
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return 0, nil, err
	}

	query := `
SELECT
	id, change_title, change_type, change_level, change_content, impact_scope, change_ip_list, change_time,
	operator, reviewer, change_result, rollback_plan, rollback_status, remark, create_time, update_time
FROM platform_ops_change_record
` + baseWhere + ` ORDER BY change_time DESC, id DESC LIMIT ? OFFSET ?`

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var items []OpsChangeRecord
	for rows.Next() {
		var item OpsChangeRecord
		var changeTime, rollbackPlan sql.NullString
		if err := rows.Scan(
			&item.ID, &item.ChangeTitle, &item.ChangeType, &item.ChangeLevel, &item.ChangeContent, &item.ImpactScope, &item.ChangeIPList, &changeTime,
			&item.Operator, &item.Reviewer, &item.ChangeResult, &rollbackPlan, &item.RollbackStatus, &item.Remark, &item.CreateTime, &item.UpdateTime,
		); err != nil {
			return 0, nil, err
		}
		if changeTime.Valid {
			item.ChangeTime = changeTime.String
		}
		if rollbackPlan.Valid {
			item.RollbackPlan = rollbackPlan.String
		}
		items = append(items, item)
	}

	return total, items, nil
}

// UpdateOpsChangeRecordReviewer
// ------------------------------------------------------------
// 管理员复核运维变更记录，设置复核人并将状态推进为"待变更"。
func UpdateOpsChangeRecordReviewer(id int64, reviewer string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("记录ID不能为空")
	}

	query := `UPDATE platform_ops_change_record SET reviewer = ?, change_result = '待变更' WHERE id = ?`
	res, err := db.Exec(query, reviewer, id)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("记录不存在")
	}

	return nil
}

// UpdateOpsChangeRecordResult
// ------------------------------------------------------------
// 管理员确认变更结果。成功时自动设回滚状态为「无需回滚」。
func UpdateOpsChangeRecordResult(id int64, changeResult string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("记录ID不能为空")
	}

	var query string
	if changeResult == "成功" {
		query = `UPDATE platform_ops_change_record SET change_result = ?, rollback_status = '无需回滚' WHERE id = ?`
	} else {
		query = `UPDATE platform_ops_change_record SET change_result = ? WHERE id = ?`
	}
	res, err := db.Exec(query, changeResult, id)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("记录不存在")
	}

	return nil
}

// UpdateOpsChangeRecordRollback
// ------------------------------------------------------------
// 管理员确认回滚状态。
func UpdateOpsChangeRecordRollback(id int64, rollbackStatus string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("记录ID不能为空")
	}

	query := `UPDATE platform_ops_change_record SET rollback_status = ? WHERE id = ?`
	res, err := db.Exec(query, rollbackStatus, id)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("记录不存在")
	}

	return nil
}

// isOpsChangeCompleted 判断变更记录是否已完成（不可再编辑/删除）
func isOpsChangeCompleted(changeResult, rollbackStatus sql.NullString) bool {
	if !changeResult.Valid {
		return false
	}
	cr := strings.TrimSpace(changeResult.String)
	if cr == "待复核" || cr == "待变更" {
		return false
	}
	if (cr == "失败" || cr == "部分成功") && rollbackStatus.Valid && strings.TrimSpace(rollbackStatus.String) == "待确认" {
		return false
	}
	return true
}
