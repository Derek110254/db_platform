package sql

import (
	"database/sql"
	"errors"
	"strings"

	"sql_platform/server/config"
)

/*
db_data_sync_request.go
----------------------------------------------------------------------
该文件负责数据库数据同步申请的核心逻辑处理。

主要功能：
1. 数据库数据同步申请记录的增、删、改、查（CRUD）。
2. 按用户角色区分操作权限，admin可管理所有，普通用户管理自己。
3. 执行DBA更新（如果执行DBA已有值，则不允许修改/删除等逻辑）。
*/

// DBDataSyncRequestRecord
// ------------------------------------------------------------
// 数据库数据同步申请记录模型
type DBDataSyncRequestRecord struct {
	ID                    int64  `json:"id"`
	Applicant             string `json:"applicant"`
	ApplicantTeam         string `json:"applicantTeam"`
	Environment           string `json:"environment"`
	ExpectedFinishTime    string `json:"expectedFinishTime"`
	UrgencyLevel          string `json:"urgencyLevel"`
	UrgencyReason         string `json:"urgencyReason"`
	ExecuteDba            string `json:"executeDba"`
	ApplyReason           string `json:"applyReason"`
	OperateType           int    `json:"operateType"`
	SourceDb              string `json:"sourceDb"`
	TargetDbOrPerson      string `json:"targetDbOrPerson"`
	InvolvedDbSchemaTable string `json:"involvedDbSchemaTable"`
	DataFilterCondition   string `json:"dataFilterCondition"`
	EstimatedDataVolume   string `json:"estimatedDataVolume"`
	ContainsSensitiveData int    `json:"containsSensitiveData"`
	DesensitizationRule   string `json:"desensitizationRule"`
	CreateTime            string `json:"createTime"`
	UpdateTime            string `json:"updateTime"`
}

// CreateDBDataSyncRequest
// ------------------------------------------------------------
// 创建数据库数据同步申请。
func CreateDBDataSyncRequest(item DBDataSyncRequestRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	query := `
INSERT INTO platform_db_data_sync_request (
	applicant, applicant_team, environment, expected_finish_time, urgency_level, urgency_reason,
	apply_reason, operate_type, source_db, target_db_or_person,
	involved_db_schema_table, data_filter_condition, estimated_data_volume,
	contains_sensitive_data, desensitization_rule
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
	var timeVal interface{} = item.ExpectedFinishTime
	if item.ExpectedFinishTime == "" {
		timeVal = nil
	}

	_, err = db.Exec(query,
		item.Applicant, item.ApplicantTeam, item.Environment, timeVal, item.UrgencyLevel, item.UrgencyReason,
		item.ApplyReason, item.OperateType, item.SourceDb, item.TargetDbOrPerson,
		item.InvolvedDbSchemaTable, item.DataFilterCondition, item.EstimatedDataVolume,
		item.ContainsSensitiveData, item.DesensitizationRule,
	)
	return err
}

// UpdateDBDataSyncRequest
// ------------------------------------------------------------
// 更新数据库数据同步申请。
func UpdateDBDataSyncRequest(item DBDataSyncRequestRecord, roleName string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if item.ID <= 0 {
		return errors.New("申请ID不能为空")
	}

	// 检查记录是否存在且属于当前用户，并检查是否已执行
	var executeDba sql.NullString
	if roleName == "admin" {
		err = db.QueryRow("SELECT execute_dba FROM platform_db_data_sync_request WHERE id = ?", item.ID).Scan(&executeDba)
	} else {
		err = db.QueryRow("SELECT execute_dba FROM platform_db_data_sync_request WHERE id = ? AND applicant = ?", item.ID, item.Applicant).Scan(&executeDba)
	}
	if err == sql.ErrNoRows {
		return errors.New("申请不存在或无权限编辑")
	} else if err != nil {
		return err
	}

	if executeDba.Valid && strings.TrimSpace(executeDba.String) != "" {
		return errors.New("该申请DBA已执行，无法再进行编辑")
	}

	query := `
UPDATE platform_db_data_sync_request
SET
	applicant_team = ?,
	environment = ?,
	expected_finish_time = ?,
	urgency_level = ?,
	urgency_reason = ?,
	apply_reason = ?,
	operate_type = ?,
	source_db = ?,
	target_db_or_person = ?,
	involved_db_schema_table = ?,
	data_filter_condition = ?,
	estimated_data_volume = ?,
	contains_sensitive_data = ?,
	desensitization_rule = ?
`
	var timeVal interface{} = item.ExpectedFinishTime
	if item.ExpectedFinishTime == "" {
		timeVal = nil
	}

	if roleName == "admin" {
		query += " WHERE id = ?"
		_, err = db.Exec(query,
			item.ApplicantTeam, item.Environment, timeVal, item.UrgencyLevel, item.UrgencyReason,
			item.ApplyReason, item.OperateType, item.SourceDb, item.TargetDbOrPerson,
			item.InvolvedDbSchemaTable, item.DataFilterCondition, item.EstimatedDataVolume,
			item.ContainsSensitiveData, item.DesensitizationRule,
			item.ID,
		)
	} else {
		query += " WHERE id = ? AND applicant = ?"
		_, err = db.Exec(query,
			item.ApplicantTeam, item.Environment, timeVal, item.UrgencyLevel, item.UrgencyReason,
			item.ApplyReason, item.OperateType, item.SourceDb, item.TargetDbOrPerson,
			item.InvolvedDbSchemaTable, item.DataFilterCondition, item.EstimatedDataVolume,
			item.ContainsSensitiveData, item.DesensitizationRule,
			item.ID, item.Applicant,
		)
	}

	return err
}

// DeleteDBDataSyncRequest
// ------------------------------------------------------------
// 删除数据库数据同步申请。
func DeleteDBDataSyncRequest(id int64, applicant string, roleName string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("申请ID不能为空")
	}

	var executeDba sql.NullString
	if roleName == "admin" {
		err = db.QueryRow("SELECT execute_dba FROM platform_db_data_sync_request WHERE id = ?", id).Scan(&executeDba)
	} else {
		err = db.QueryRow("SELECT execute_dba FROM platform_db_data_sync_request WHERE id = ? AND applicant = ?", id, applicant).Scan(&executeDba)
	}
	if err == sql.ErrNoRows {
		return errors.New("申请不存在或无权限删除")
	} else if err != nil {
		return err
	}

	if executeDba.Valid && strings.TrimSpace(executeDba.String) != "" {
		return errors.New("该申请DBA已执行，无法再进行删除")
	}

	var res sql.Result
	if roleName == "admin" {
		res, err = db.Exec(`DELETE FROM platform_db_data_sync_request WHERE id = ?`, id)
	} else {
		res, err = db.Exec(`DELETE FROM platform_db_data_sync_request WHERE id = ? AND applicant = ?`, id, applicant)
	}

	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("申请不存在或无权限删除")
	}

	return nil
}

// QueryDBDataSyncRequests
// ------------------------------------------------------------
// 分页查询数据库数据同步申请。
func QueryDBDataSyncRequests(page, pageSize int, applicant, urgencyLevel string, operateType int) (int64, []DBDataSyncRequestRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return 0, nil, err
	}

	baseWhere := "WHERE 1=1"
	var args []interface{}

	if applicant = strings.TrimSpace(applicant); applicant != "" {
		baseWhere += " AND applicant = ?"
		args = append(args, applicant)
	}
	if urgencyLevel = strings.TrimSpace(urgencyLevel); urgencyLevel != "" {
		baseWhere += " AND urgency_level = ?"
		args = append(args, urgencyLevel)
	}
	if operateType > 0 {
		baseWhere += " AND operate_type = ?"
		args = append(args, operateType)
	}

	countQuery := "SELECT COUNT(1) FROM platform_db_data_sync_request " + baseWhere
	var total int64
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return 0, nil, err
	}

	query := `
SELECT 
	id, applicant, applicant_team, environment, expected_finish_time, urgency_level, urgency_reason,
	execute_dba, apply_reason, operate_type, source_db, target_db_or_person,
	involved_db_schema_table, data_filter_condition, estimated_data_volume,
	contains_sensitive_data, desensitization_rule, create_time, update_time
FROM platform_db_data_sync_request
` + baseWhere + ` ORDER BY id DESC LIMIT ? OFFSET ?`

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var items []DBDataSyncRequestRecord
	for rows.Next() {
		var item DBDataSyncRequestRecord
		var expectedTime sql.NullString
		if err := rows.Scan(
			&item.ID, &item.Applicant, &item.ApplicantTeam, &item.Environment, &expectedTime, &item.UrgencyLevel, &item.UrgencyReason,
			&item.ExecuteDba, &item.ApplyReason, &item.OperateType, &item.SourceDb, &item.TargetDbOrPerson,
			&item.InvolvedDbSchemaTable, &item.DataFilterCondition, &item.EstimatedDataVolume,
			&item.ContainsSensitiveData, &item.DesensitizationRule, &item.CreateTime, &item.UpdateTime,
		); err != nil {
			return 0, nil, err
		}
		if expectedTime.Valid {
			item.ExpectedFinishTime = expectedTime.String
		}
		items = append(items, item)
	}

	return total, items, nil
}

// UpdateDBDataSyncRequestDBA
// ------------------------------------------------------------
// 更新数据库数据同步申请的执行DBA信息。
func UpdateDBDataSyncRequestDBA(id int64, executeDba string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("申请ID不能为空")
	}

	query := `
UPDATE platform_db_data_sync_request
SET execute_dba = ?
WHERE id = ?
`
	res, err := db.Exec(query, executeDba, id)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("申请不存在")
	}

	return nil
}
