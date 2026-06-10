package sql

import (
	"errors"

	"sql_platform/server/config"
)

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
