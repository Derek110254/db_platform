package sql

import (
	"database/sql"
	"errors"
	"strings"

	"sql_platform/server/config"
)

/*
db_change_request.go
----------------------------------------------------------------------
该文件负责数据库变更申请的核心逻辑处理。

主要功能：
1. 数据库变更申请记录的增、删、改、查（CRUD）。
2. 按用户角色（admin/普通用户）区分操作权限。
3. 对已发布验证成功的变更申请，禁止二次编辑或删除。
4. 提供针对发布验证页面的专属查询接口。
*/

// DBChangeRequestRecord
// ------------------------------------------------------------
// 数据库变更申请记录模型
type DBChangeRequestRecord struct {
	ID                int64  `json:"id"`
	Applicant         string `json:"applicant"`         // 申请人
	ApplicantTeam     string `json:"applicantTeam"`     // 申请团队
	PlannedChangeTime string `json:"plannedChangeTime"` // 计划变更时间
	UrgencyLevel      string `json:"urgencyLevel"`      // 紧急程度
	TestPublisher     string `json:"testPublisher"`     // 测试线发布人
	ProdPublisher     string `json:"prodPublisher"`     // 生产线发布人
	ChangeType        string `json:"changeType"`        // 变更类型
	ChangeReason      string `json:"changeReason"`      // 变更原因
	RequirementUrl    string `json:"requirementUrl"`    // 需求url
	ImpactScope       string `json:"impactScope"`       // 影响范围
	DbType            string `json:"dbType"`            // 数据库类型
	TestDbIp          string `json:"testDbIp"`          // 测试线数据库IP
	TestDbName        string `json:"testDbName"`        // 测试线数据库名
	TestDbSchema      string `json:"testDbSchema"`      // 测试线数据库schema
	DbIp              string `json:"dbIp"`              // 生产线数据库IP
	DbName            string `json:"dbName"`            // 生产线数据库实例/数据库名
	DbSchema          string `json:"dbSchema"`          // 生产线数据库schema
	ChangeContent     string `json:"changeContent"`     // 变更内容
	ReleaseVerifier   string `json:"releaseVerifier"`   // 发布验证人
	CreateTime        string `json:"createTime"`        // 创建时间
	UpdateTime        string `json:"updateTime"`        // 更新时间
}

// CreateDBChangeRequest
// ------------------------------------------------------------
// 创建数据库变更申请。
// 将前端传入的申请记录插入到 platform_db_change_request 表中。
// 如果 planned_change_time 为空，则存入 NULL。
func CreateDBChangeRequest(item DBChangeRequestRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	query := `
INSERT INTO platform_db_change_request (
	applicant, applicant_team, planned_change_time, urgency_level,
	change_type, change_reason,
	requirement_url, impact_scope, db_type, test_db_ip, test_db_name, test_db_schema, db_ip, db_name, db_schema,
	change_content
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
	var timeVal interface{} = item.PlannedChangeTime
	if item.PlannedChangeTime == "" {
		timeVal = nil
	}

	_, err = db.Exec(query,
		item.Applicant, item.ApplicantTeam, timeVal, item.UrgencyLevel,
		item.ChangeType, item.ChangeReason,
		item.RequirementUrl, item.ImpactScope, item.DbType, item.TestDbIp, item.TestDbName, item.TestDbSchema, item.DbIp, item.DbName, item.DbSchema,
		item.ChangeContent,
	)
	return err
}

// UpdateDBChangeRequest
// ------------------------------------------------------------
// 更新数据库变更申请。
//
// 业务逻辑与权限控制：
//  1. 根据当前用户的 roleName 判断权限，admin 可以编辑所有记录，普通用户只能编辑自己的申请。
//  2. 检查记录是否已经通过发布验证（测试线发布人、生产线发布人、发布验证人均不为空），
//     如果已验证，则拒绝修改操作。
func UpdateDBChangeRequest(item DBChangeRequestRecord, roleName string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if item.ID <= 0 {
		return errors.New("变更申请ID不能为空")
	}

	// 检查记录是否存在且属于当前用户
	var testPublisher, prodPublisher, releaseVerifier sql.NullString
	if roleName == "admin" {
		err = db.QueryRow("SELECT test_publisher, prod_publisher, release_verifier FROM platform_db_change_request WHERE id = ?", item.ID).Scan(&testPublisher, &prodPublisher, &releaseVerifier)
	} else {
		err = db.QueryRow("SELECT test_publisher, prod_publisher, release_verifier FROM platform_db_change_request WHERE id = ? AND applicant = ?", item.ID, item.Applicant).Scan(&testPublisher, &prodPublisher, &releaseVerifier)
	}
	if err == sql.ErrNoRows {
		return errors.New("申请不存在或无权限编辑")
	} else if err != nil {
		return err
	}

	// 如果三个字段都不为空，说明验证成功，无法编辑
	if testPublisher.Valid && strings.TrimSpace(testPublisher.String) != "" &&
		prodPublisher.Valid && strings.TrimSpace(prodPublisher.String) != "" &&
		releaseVerifier.Valid && strings.TrimSpace(releaseVerifier.String) != "" {
		return errors.New("该申请已经发布验证成功，无法再进行编辑")
	}

	query := `
UPDATE platform_db_change_request
SET
	applicant_team = ?,
	planned_change_time = ?,
	urgency_level = ?,
	change_type = ?,
	change_reason = ?,
	requirement_url = ?,
	impact_scope = ?,
	db_type = ?,
	test_db_ip = ?,
	test_db_name = ?,
	test_db_schema = ?,
	db_ip = ?,
	db_name = ?,
	db_schema = ?,
	change_content = ?
`
	var timeVal interface{} = item.PlannedChangeTime
	if item.PlannedChangeTime == "" {
		timeVal = nil
	}

	if roleName == "admin" {
		query += "WHERE id = ?"
		_, err = db.Exec(query,
			item.ApplicantTeam, timeVal, item.UrgencyLevel,
			item.ChangeType, item.ChangeReason,
			item.RequirementUrl, item.ImpactScope, item.DbType, item.TestDbIp, item.TestDbName, item.TestDbSchema, item.DbIp, item.DbName, item.DbSchema,
			item.ChangeContent,
			item.ID,
		)
	} else {
		query += "WHERE id = ? AND applicant = ?"
		_, err = db.Exec(query,
			item.ApplicantTeam, timeVal, item.UrgencyLevel,
			item.ChangeType, item.ChangeReason,
			item.RequirementUrl, item.ImpactScope, item.DbType, item.TestDbIp, item.TestDbName, item.TestDbSchema, item.DbIp, item.DbName, item.DbSchema,
			item.ChangeContent,
			item.ID, item.Applicant,
		)
	}
	if err != nil {
		return err
	}

	return nil
}

// DeleteDBChangeRequest
// ------------------------------------------------------------
// 删除数据库变更申请。
//
// 业务逻辑与权限控制：
// 1. admin 可以删除所有记录，普通用户只能删除自己的记录。
// 2. 如果记录已经通过发布验证（相关发布人和验证人字段均不为空），则拒绝删除。
func DeleteDBChangeRequest(id int64, applicant string, roleName string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("申请ID不能为空")
	}

	// 检查是否已经验证通过
	var testPublisher, prodPublisher, releaseVerifier sql.NullString
	if roleName == "admin" {
		err = db.QueryRow("SELECT test_publisher, prod_publisher, release_verifier FROM platform_db_change_request WHERE id = ?", id).Scan(&testPublisher, &prodPublisher, &releaseVerifier)
	} else {
		err = db.QueryRow("SELECT test_publisher, prod_publisher, release_verifier FROM platform_db_change_request WHERE id = ? AND applicant = ?", id, applicant).Scan(&testPublisher, &prodPublisher, &releaseVerifier)
	}
	if err == sql.ErrNoRows {
		return errors.New("申请不存在或无权限删除")
	} else if err != nil {
		return err
	}

	// 如果三个字段都不为空，说明验证成功，无法删除
	if testPublisher.Valid && strings.TrimSpace(testPublisher.String) != "" &&
		prodPublisher.Valid && strings.TrimSpace(prodPublisher.String) != "" &&
		releaseVerifier.Valid && strings.TrimSpace(releaseVerifier.String) != "" {
		return errors.New("该申请已经发布验证成功，无法再进行删除")
	}

	var res sql.Result
	if roleName == "admin" {
		res, err = db.Exec(`DELETE FROM platform_db_change_request WHERE id = ?`, id)
	} else {
		res, err = db.Exec(`DELETE FROM platform_db_change_request WHERE id = ? AND applicant = ?`, id, applicant)
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

// QueryDBChangeRequests
// ------------------------------------------------------------
// 分页查询数据库变更申请。
//
// 参数：
// - page: 当前页码
// - pageSize: 每页大小
// - applicant: 申请人（精确匹配）
// - applicantTeam: 申请团队（模糊匹配）
// - urgencyLevel: 紧急程度（精确匹配）
// - dbType: 数据库类型（精确匹配，通过 FIND_IN_SET 支持逗号分隔的格式处理）
func QueryDBChangeRequests(page, pageSize int, applicant, applicantTeam, urgencyLevel, dbType string) (int64, []DBChangeRequestRecord, error) {
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
	if applicantTeam = strings.TrimSpace(applicantTeam); applicantTeam != "" {
		baseWhere += " AND applicant_team LIKE ?"
		args = append(args, "%"+applicantTeam+"%")
	}
	if urgencyLevel = strings.TrimSpace(urgencyLevel); urgencyLevel != "" {
		baseWhere += " AND urgency_level = ?"
		args = append(args, urgencyLevel)
	}
	if dbType = strings.TrimSpace(dbType); dbType != "" {
		baseWhere += " AND FIND_IN_SET(?, REPLACE(db_type, ' ', '')) > 0"
		args = append(args, dbType)
	}

	countQuery := "SELECT COUNT(1) FROM platform_db_change_request " + baseWhere
	var total int64
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return 0, nil, err
	}

	query := `
SELECT 
	id, applicant, applicant_team, planned_change_time, urgency_level,
	test_publisher, prod_publisher,
	change_type, change_reason,
	requirement_url, impact_scope, db_type, 
	test_db_ip, test_db_name, test_db_schema,
	db_ip, db_name, db_schema,
	change_content, release_verifier, create_time, update_time
FROM platform_db_change_request
` + baseWhere + ` ORDER BY id DESC LIMIT ? OFFSET ?`

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var items []DBChangeRequestRecord
	for rows.Next() {
		var item DBChangeRequestRecord
		var plannedTime sql.NullString
		if err := rows.Scan(
			&item.ID, &item.Applicant, &item.ApplicantTeam, &plannedTime, &item.UrgencyLevel,
			&item.TestPublisher, &item.ProdPublisher,
			&item.ChangeType, &item.ChangeReason,
			&item.RequirementUrl, &item.ImpactScope, &item.DbType,
			&item.TestDbIp, &item.TestDbName, &item.TestDbSchema,
			&item.DbIp, &item.DbName, &item.DbSchema,
			&item.ChangeContent, &item.ReleaseVerifier, &item.CreateTime, &item.UpdateTime,
		); err != nil {
			return 0, nil, err
		}
		if plannedTime.Valid {
			item.PlannedChangeTime = plannedTime.String
		}
		items = append(items, item)
	}

	return total, items, nil
}

// QueryDBChangeRequestsForRelease
// ------------------------------------------------------------
// 分页查询数据库变更申请 (专门针对发布验证页面)。
//
// 区别于普通查询：
// 可以通过 isVerified 过滤已验证（三者均不为空）和未验证的记录。
func QueryDBChangeRequestsForRelease(page, pageSize int, applicantTeam, urgencyLevel, dbType, isVerified string) (int64, []DBChangeRequestRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return 0, nil, err
	}

	baseWhere := "WHERE 1=1"
	var args []interface{}

	switch isVerified {
	case "1":
		baseWhere += " AND (test_publisher != '' AND test_publisher IS NOT NULL AND prod_publisher != '' AND prod_publisher IS NOT NULL AND release_verifier != '' AND release_verifier IS NOT NULL)"
	case "0":
		baseWhere += " AND (test_publisher = '' OR test_publisher IS NULL OR prod_publisher = '' OR prod_publisher IS NULL OR release_verifier = '' OR release_verifier IS NULL)"
	}

	if applicantTeam = strings.TrimSpace(applicantTeam); applicantTeam != "" {
		baseWhere += " AND applicant_team LIKE ?"
		args = append(args, "%"+applicantTeam+"%")
	}
	if urgencyLevel = strings.TrimSpace(urgencyLevel); urgencyLevel != "" {
		baseWhere += " AND urgency_level = ?"
		args = append(args, urgencyLevel)
	}
	if dbType = strings.TrimSpace(dbType); dbType != "" {
		baseWhere += " AND FIND_IN_SET(?, REPLACE(db_type, ' ', '')) > 0"
		args = append(args, dbType)
	}

	countQuery := "SELECT COUNT(1) FROM platform_db_change_request " + baseWhere
	var total int64
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return 0, nil, err
	}

	query := `
SELECT 
	id, applicant, applicant_team, planned_change_time, urgency_level,
	test_publisher, prod_publisher,
	change_type, change_reason,
	requirement_url, impact_scope, db_type, 
	test_db_ip, test_db_name, test_db_schema,
	db_ip, db_name, db_schema,
	change_content, release_verifier, create_time, update_time
FROM platform_db_change_request
` + baseWhere + ` ORDER BY id DESC LIMIT ? OFFSET ?`

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var items []DBChangeRequestRecord
	for rows.Next() {
		var item DBChangeRequestRecord
		var plannedTime sql.NullString
		if err := rows.Scan(
			&item.ID, &item.Applicant, &item.ApplicantTeam, &plannedTime, &item.UrgencyLevel,
			&item.TestPublisher, &item.ProdPublisher,
			&item.ChangeType, &item.ChangeReason,
			&item.RequirementUrl, &item.ImpactScope, &item.DbType,
			&item.TestDbIp, &item.TestDbName, &item.TestDbSchema,
			&item.DbIp, &item.DbName, &item.DbSchema,
			&item.ChangeContent, &item.ReleaseVerifier, &item.CreateTime, &item.UpdateTime,
		); err != nil {
			return 0, nil, err
		}
		if plannedTime.Valid {
			item.PlannedChangeTime = plannedTime.String
		}
		items = append(items, item)
	}

	return total, items, nil
}

// UpdateDBChangeRequestRelease
// ------------------------------------------------------------
// 更新数据库变更申请的发布验证信息。
//
// 更新记录的以下字段：
// - 测试线发布人 (test_publisher)
// - 生产线发布人 (prod_publisher)
// - 发布验证人 (release_verifier)
func UpdateDBChangeRequestRelease(id int64, testPublisher, prodPublisher, releaseVerifier string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("变更申请ID不能为空")
	}

	query := `
UPDATE platform_db_change_request
SET
	test_publisher = ?,
	prod_publisher = ?,
	release_verifier = ?
WHERE id = ?
`
	res, err := db.Exec(query, testPublisher, prodPublisher, releaseVerifier, id)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("申请不存在")
	}

	return nil
}
