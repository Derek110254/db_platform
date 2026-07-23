package sql

import (
	"errors"

	"sql_platform/server/config"
)

/*
team_db_env.go
----------------------------------------------------------------------
该文件负责团队数据库环境配置的管理，对应前端「团队环境配置」相关页面。

主要功能：
1. 维护各团队（交易/运营/后台/增长等）的测试线、生产线数据库连接信息。
2. 提供按团队分页查询、全量查询，以及新增/编辑/删除环境配置。
3. 用户在提交变更/同步申请时，可按团队选择环境自动填入连接信息，减少手工录入。
*/

// TeamDbEnvRecord
// ------------------------------------------------------------
// 团队数据库环境配置表
type TeamDbEnvRecord struct {
	ID             int64  `json:"id"`
	TeamName       string `json:"teamName"`
	EnvName        string `json:"envName"`
	DbType         string `json:"dbType"`
	TestDbIp       string `json:"testDbIp"`
	TestDbName     string `json:"testDbName"`
	TestDbSchema   string `json:"testDbSchema"`
	ProdDbIp       string `json:"prodDbIp"`
	ProdDbName     string `json:"prodDbName"`
	ProdDbSchema   string `json:"prodDbSchema"`
	CreateTime     string `json:"createTime"`
	UpdateTime     string `json:"updateTime"`
}

// CreateTeamDbEnv 新增一条团队数据库环境配置。
func CreateTeamDbEnv(item TeamDbEnvRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	query := `
INSERT INTO platform_team_db_env (
	team_name, env_name, db_type,
	test_db_ip, test_db_name, test_db_schema,
	prod_db_ip, prod_db_name, prod_db_schema
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`
	_, err = db.Exec(query,
		item.TeamName, item.EnvName, item.DbType,
		item.TestDbIp, item.TestDbName, item.TestDbSchema,
		item.ProdDbIp, item.ProdDbName, item.ProdDbSchema,
	)
	return err
}

// UpdateTeamDbEnv 按主键更新一条团队数据库环境配置。
func UpdateTeamDbEnv(item TeamDbEnvRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if item.ID <= 0 {
		return errors.New("ID不能为空")
	}

	query := `
UPDATE platform_team_db_env
SET
	team_name = ?,
	env_name = ?,
	db_type = ?,
	test_db_ip = ?,
	test_db_name = ?,
	test_db_schema = ?,
	prod_db_ip = ?,
	prod_db_name = ?,
	prod_db_schema = ?
WHERE id = ?
`
	_, err = db.Exec(query,
		item.TeamName, item.EnvName, item.DbType,
		item.TestDbIp, item.TestDbName, item.TestDbSchema,
		item.ProdDbIp, item.ProdDbName, item.ProdDbSchema,
		item.ID,
	)
	return err
}

// DeleteTeamDbEnv 按主键删除一条团队数据库环境配置。
func DeleteTeamDbEnv(id int64) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("ID不能为空")
	}

	_, err = db.Exec("DELETE FROM platform_team_db_env WHERE id = ?", id)
	return err
}

// ListTeamDbEnvs 分页查询团队数据库环境配置，可按团队名过滤，按 id 倒序返回。
func ListTeamDbEnvs(page int, pageSize int, teamName string) (int64, []TeamDbEnvRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return 0, nil, err
	}

	var total int64
	
	countQuery := "SELECT COUNT(*) FROM platform_team_db_env WHERE 1=1"
	var args []interface{}
	
	if teamName != "" {
		countQuery += " AND team_name = ?"
		args = append(args, teamName)
	}

	err = db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return 0, nil, err
	}

	query := `
SELECT id, team_name, env_name, db_type,
	test_db_ip, test_db_name, test_db_schema,
	prod_db_ip, prod_db_name, prod_db_schema,
	create_time, update_time
FROM platform_team_db_env
WHERE 1=1
`
	if teamName != "" {
		query += " AND team_name = ?"
	}

	query += " ORDER BY id DESC LIMIT ? OFFSET ?"

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var list []TeamDbEnvRecord
	for rows.Next() {
		var item TeamDbEnvRecord
		if err := rows.Scan(
			&item.ID, &item.TeamName, &item.EnvName, &item.DbType,
			&item.TestDbIp, &item.TestDbName, &item.TestDbSchema,
			&item.ProdDbIp, &item.ProdDbName, &item.ProdDbSchema,
			&item.CreateTime, &item.UpdateTime,
		); err != nil {
			return 0, nil, err
		}
		list = append(list, item)
	}

	return total, list, nil
}

// ListAllTeamDbEnvs 查询全部团队数据库环境配置，按团队名、环境名排序，供前端下拉选择使用。
func ListAllTeamDbEnvs() ([]TeamDbEnvRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return nil, err
	}

	query := `
SELECT id, team_name, env_name, db_type,
	test_db_ip, test_db_name, test_db_schema,
	prod_db_ip, prod_db_name, prod_db_schema,
	create_time, update_time
FROM platform_team_db_env
ORDER BY team_name, env_name
`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []TeamDbEnvRecord
	for rows.Next() {
		var item TeamDbEnvRecord
		if err := rows.Scan(
			&item.ID, &item.TeamName, &item.EnvName, &item.DbType,
			&item.TestDbIp, &item.TestDbName, &item.TestDbSchema,
			&item.ProdDbIp, &item.ProdDbName, &item.ProdDbSchema,
			&item.CreateTime, &item.UpdateTime,
		); err != nil {
			return nil, err
		}
		list = append(list, item)
	}

	return list, nil
}
