package sql

import (
	"database/sql"
	"errors"
	"fmt"

	"sql_platform/server/config"
)

/*
connection.go
----------------------------------------------------------------------
该文件负责平台数据库连接配置管理的核心 SQL 逻辑，对应前端「连接管理」页面。

主要功能：
1. 查询全部数据库连接配置。
2. 创建/编辑/删除连接配置（含重名检查、名称变更时的级联更新）。
3. 解密连接密码明文（供连接测试使用）。

说明：
- HTTP 请求绑定结构体仍留在 routes/api.go，本文件只承载 DB 记录模型与 SQL。
- 删除连接前会校验是否仍被用户权限引用；SQL 收藏中的脏引用会被清空。
*/

// 哨兵错误，供 handler 映射对应的 HTTP 提示
var (
	ErrConnectionNameExists = errors.New("连接名称已存在")
	ErrConnectionInUse      = errors.New("该连接仍被用户权限引用，无法删除。请先取消用户分配后再删除")
	ErrConnectionNotFound    = errors.New("连接不存在或已被删除")
)

// 需要随 connection_name 级联更新的关联表
var connectionNameTables = []string{
	"platform_user_db_connection",
	"platform_sql_favorite",
	"platform_sql_audit",
}

// ConnectionRecord 数据库连接配置记录模型
type ConnectionRecord struct {
	ID           int64
	Name         string
	DBType       string
	Host         string
	Port         int
	Username     string
	Password     string // 写入时用于加密；查询时不填充
	DatabaseName string
	ServiceName  string
	IsEnabled    int
	CanConnect   int
	CreateTime   string
	UpdateTime  string
}

// ListConnections 查询全部数据库连接配置，按 id 倒序。
func ListConnections() ([]ConnectionRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
SELECT id, name, db_type, host, port, username, database_name, service_name, is_enabled, can_connect, create_time, update_time
FROM platform_db_connection
ORDER BY id DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ConnectionRecord, 0)
	for rows.Next() {
		var rec ConnectionRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.Name,
			&rec.DBType,
			&rec.Host,
			&rec.Port,
			&rec.Username,
			&rec.DatabaseName,
			&rec.ServiceName,
			&rec.IsEnabled,
			&rec.CanConnect,
			&rec.CreateTime,
			&rec.UpdateTime,
		); err != nil {
			return nil, err
		}
		items = append(items, rec)
	}
	return items, nil
}

// CreateConnection 创建连接配置。名称已存在时返回 ErrConnectionNameExists。
// 密码通过 fixed_aes_encrypt 加密后入库。
func CreateConnection(rec ConnectionRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM platform_db_connection WHERE name = ?`, rec.Name).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return ErrConnectionNameExists
	}

	if _, err := db.Exec(`
INSERT INTO platform_db_connection (
    name,
    db_type,
    host,
    port,
    username,
    password_cipher,
    database_name,
    service_name,
    is_enabled,
    can_connect
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    fixed_aes_encrypt(?),
    ?,
    ?,
    ?,
    ?
)
`, rec.Name, rec.DBType, rec.Host, rec.Port, rec.Username, rec.Password, rec.DatabaseName, rec.ServiceName, rec.IsEnabled, rec.CanConnect); err != nil {
		return err
	}
	return nil
}

// UpdateConnection 编辑连接配置。password 为空时不修改密码。
// 连接不存在返回 ErrConnectionNotFound；新名称被占用返回 ErrConnectionNameExists。
// 名称变更时在同一事务内级联更新所有引用 connection_name 的关联表。
func UpdateConnection(id int64, rec ConnectionRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	// 读取当前连接名称，用于判断是否需要级联更新
	var currentName string
	if err := db.QueryRow(`SELECT name FROM platform_db_connection WHERE id = ?`, id).Scan(&currentName); err != nil {
		if err == sql.ErrNoRows {
			return ErrConnectionNotFound
		}
		return err
	}

	nameChanged := currentName != rec.Name
	if nameChanged {
		var count int
		if err := db.QueryRow(`SELECT COUNT(1) FROM platform_db_connection WHERE name = ? AND id != ?`, rec.Name, id).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return ErrConnectionNameExists
		}
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if rec.Password == "" {
		_, err = tx.Exec(`
UPDATE platform_db_connection
SET name = ?, db_type = ?, host = ?, port = ?, username = ?, database_name = ?, service_name = ?, is_enabled = ?, can_connect = ?
WHERE id = ?
`, rec.Name, rec.DBType, rec.Host, rec.Port, rec.Username, rec.DatabaseName, rec.ServiceName, rec.IsEnabled, rec.CanConnect, id)
	} else {
		_, err = tx.Exec(`
UPDATE platform_db_connection
SET name = ?, db_type = ?, host = ?, port = ?, username = ?, password_cipher = fixed_aes_encrypt(?), database_name = ?, service_name = ?, is_enabled = ?, can_connect = ?
WHERE id = ?
`, rec.Name, rec.DBType, rec.Host, rec.Port, rec.Username, rec.Password, rec.DatabaseName, rec.ServiceName, rec.IsEnabled, rec.CanConnect, id)
	}
	if err != nil {
		return err
	}

	if nameChanged {
		for _, table := range connectionNameTables {
			if _, err = tx.Exec(fmt.Sprintf(`UPDATE %s SET connection_name = ? WHERE connection_name = ?`, table), rec.Name, currentName); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// DeleteConnection 删除连接配置。
// 连接不存在返回 ErrConnectionNotFound；仍被用户权限引用返回 ErrConnectionInUse。
// SQL 收藏中的同名脏引用会被清空（不阻止删除）。
func DeleteConnection(id int64) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var name string
	if err := db.QueryRow(`SELECT name FROM platform_db_connection WHERE id = ?`, id).Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return ErrConnectionNotFound
		}
		return err
	}

	var refCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM platform_user_db_connection WHERE connection_name = ?`, name).Scan(&refCount); err != nil {
		return err
	}
	if refCount > 0 {
		return ErrConnectionInUse
	}

	// SQL 收藏里如果仍有该 connection_name，不阻止删除，但会把它清空，避免脏引用
	if _, err := db.Exec(`
UPDATE platform_sql_favorite
SET connection_name = ''
WHERE connection_name = ?
`, name); err != nil {
		return err
	}

	res, err := db.Exec(`DELETE FROM platform_db_connection WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrConnectionNotFound
	}
	return nil
}

// GetConnectionPasswordPlain 解密指定连接的密码明文，仅供连接测试使用。
// 连接不存在时返回 ("", nil)，与历史行为一致（静默忽略，由调用方处理空密码）。
func GetConnectionPasswordPlain(id int64) (string, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return "", err
	}

	var plain string
	if err := db.QueryRow("SELECT fixed_aes_decrypt(password_cipher) FROM platform_db_connection WHERE id = ?", id).Scan(&plain); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return plain, nil
}
