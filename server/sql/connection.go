package sql

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"db_platform/server/config"
)

/*
connection.go
----------------------------------------------------------------------
负责 MySQL、Oracle、PostgreSQL、MSSQL 连接配置及用户连接授权校验。

主要职责：
1. 管理员查询、新增、编辑、删除数据库连接配置。
2. 连接名称变更时，级联更新用户授权关系和 SQL 收藏中的连接名称。
3. 连接删除前，阻止删除仍被用户授权引用的连接。
4. 按数据库类型保存数据库名、Oracle 服务名及 PostgreSQL/Oracle schema 配置。
5. 查询和元数据读取前，按用户角色验证连接是否启用且已授权。
*/

var (
	ErrConnectionNameExists = errors.New("连接名称已存在")
	ErrConnectionInUse      = errors.New("该连接仍被用户权限引用，无法删除。请先取消用户分配后再删除")
	ErrConnectionNotFound   = errors.New("连接不存在或已被删除")
)

var connectionNameTables = []string{
	"user_db_connection",
	"sql_favorite",
}

// ConnectionRecord 是管理员维护连接配置时使用的记录模型。
type ConnectionRecord struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	DBType       string `json:"dbType"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password,omitempty"`
	DatabaseName string `json:"databaseName"`
	ServiceName  string `json:"serviceName"`
	IsEnabled    int    `json:"isEnabled"`
	CanConnect   int    `json:"canConnect"`
	CreateTime   string `json:"createTime"`
	UpdateTime   string `json:"updateTime"`
}

// ListConnections 返回全部数据库连接配置，按 id 倒序排列。
func ListConnections() ([]ConnectionRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
SELECT id, name, db_type, host, port, username, database_name, service_name, is_enabled, can_connect, create_time, update_time
FROM db_connection
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
	return items, rows.Err()
}

// CreateConnection 新增数据库连接配置。
func CreateConnection(rec ConnectionRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM db_connection WHERE name = ?`, rec.Name).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return ErrConnectionNameExists
	}

	_, err = db.Exec(`
INSERT INTO db_connection (
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
) VALUES (?, ?, ?, ?, ?, fixed_aes_encrypt(?), ?, ?, ?, ?)
`, rec.Name, rec.DBType, rec.Host, rec.Port, rec.Username, rec.Password, rec.DatabaseName, rec.ServiceName, rec.IsEnabled, rec.CanConnect)
	return err
}

// UpdateConnection 编辑数据库连接配置。
// password 为空时不修改原密码；连接名称改动时会一并更新关联表。
func UpdateConnection(id int64, rec ConnectionRecord) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var currentName string
	if err := db.QueryRow(`SELECT name FROM db_connection WHERE id = ?`, id).Scan(&currentName); err != nil {
		if err == sql.ErrNoRows {
			return ErrConnectionNotFound
		}
		return err
	}

	nameChanged := currentName != rec.Name
	if nameChanged {
		var count int
		if err := db.QueryRow(`SELECT COUNT(1) FROM db_connection WHERE name = ? AND id != ?`, rec.Name, id).Scan(&count); err != nil {
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
UPDATE db_connection
SET name = ?, db_type = ?, host = ?, port = ?, username = ?, database_name = ?, service_name = ?, is_enabled = ?, can_connect = ?
WHERE id = ?
`, rec.Name, rec.DBType, rec.Host, rec.Port, rec.Username, rec.DatabaseName, rec.ServiceName, rec.IsEnabled, rec.CanConnect, id)
	} else {
		_, err = tx.Exec(`
UPDATE db_connection
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

// DeleteConnection 删除数据库连接配置。
// 若连接仍被用户授权引用，则返回 ErrConnectionInUse。
func DeleteConnection(id int64) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var name string
	if err := db.QueryRow(`SELECT name FROM db_connection WHERE id = ?`, id).Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			return ErrConnectionNotFound
		}
		return err
	}

	var refCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM user_db_connection WHERE connection_name = ?`, name).Scan(&refCount); err != nil {
		return err
	}
	if refCount > 0 {
		return ErrConnectionInUse
	}

	if _, err := db.Exec(`UPDATE sql_favorite SET connection_name = '' WHERE connection_name = ?`, name); err != nil {
		return err
	}

	res, err := db.Exec(`DELETE FROM db_connection WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrConnectionNotFound
	}
	return nil
}

// GetConnectionPasswordPlain 按连接 id 解密数据库密码，供连接测试使用。
func GetConnectionPasswordPlain(id int64) (string, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return "", err
	}

	var plain string
	if err := db.QueryRow("SELECT fixed_aes_decrypt(password_cipher) FROM db_connection WHERE id = ?", id).Scan(&plain); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return plain, nil
}

// DBConnectionRecord 是查询侧加载连接时使用的记录模型，包含密码密文。
type DBConnectionRecord struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	DBType         string `json:"dbType"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	PasswordCipher string `json:"passwordCipher"`
	DatabaseName   string `json:"databaseName"`
	ServiceName    string `json:"serviceName"`
	IsEnabled      int    `json:"isEnabled"`
	CanConnect     int    `json:"canConnect"`
}

// LoadEnabledConnections 返回全部启用连接。
func LoadEnabledConnections() ([]DBConnectionRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
SELECT id, name, db_type, host, port, username, password_cipher, database_name, service_name, is_enabled, can_connect
FROM db_connection
WHERE is_enabled = 1
ORDER BY db_type, name
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDBConnectionRecords(rows)
}

// LoadEnabledConnectionsForUser 按当前用户权限返回可见连接。
// admin 可见全部启用连接；普通用户只可见已授权连接。
func LoadEnabledConnectionsForUser(userID int64, roleName string) ([]DBConnectionRecord, error) {
	if strings.EqualFold(strings.TrimSpace(roleName), "admin") {
		return LoadEnabledConnections()
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
SELECT c.id, c.name, c.db_type, c.host, c.port, c.username, c.password_cipher, c.database_name, c.service_name, c.is_enabled, c.can_connect
FROM db_connection c
INNER JOIN user_db_connection uc
        ON c.name = uc.connection_name
WHERE uc.user_id = ?
  AND c.is_enabled = 1
ORDER BY c.db_type, c.name
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanDBConnectionRecords(rows)
}

func scanDBConnectionRecords(rows *sql.Rows) ([]DBConnectionRecord, error) {
	items := make([]DBConnectionRecord, 0)
	for rows.Next() {
		var item DBConnectionRecord
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.DBType,
			&item.Host,
			&item.Port,
			&item.Username,
			&item.PasswordCipher,
			&item.DatabaseName,
			&item.ServiceName,
			&item.IsEnabled,
			&item.CanConnect,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// LoadConnectionByName 按连接名称读取一个启用连接。
func LoadConnectionByName(name string) (DBConnectionRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return DBConnectionRecord{}, err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return DBConnectionRecord{}, errors.New("连接名称不能为空")
	}

	var item DBConnectionRecord
	err = db.QueryRow(`
SELECT id, name, db_type, host, port, username, password_cipher, database_name, service_name, is_enabled, can_connect
FROM db_connection
WHERE name = ? AND is_enabled = 1
LIMIT 1
`, name).Scan(
		&item.ID,
		&item.Name,
		&item.DBType,
		&item.Host,
		&item.Port,
		&item.Username,
		&item.PasswordCipher,
		&item.DatabaseName,
		&item.ServiceName,
		&item.IsEnabled,
		&item.CanConnect,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DBConnectionRecord{}, errors.New("未找到连接配置: " + name)
		}
		return DBConnectionRecord{}, err
	}

	return item, nil
}

// GetConnectionPlainPassword 解密已加载连接记录中的密码密文。
func GetConnectionPlainPassword(record DBConnectionRecord) (string, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return "", err
	}

	var plainPassword sql.NullString
	err = db.QueryRow(`SELECT fixed_aes_decrypt(?)`, record.PasswordCipher).Scan(&plainPassword)
	if err != nil {
		return "", err
	}
	if !plainPassword.Valid {
		return "", errors.New("数据库连接密码密文解密失败")
	}

	return plainPassword.String, nil
}

// UserCanAccessConnection 判断用户是否有权限访问指定连接。
func UserCanAccessConnection(userID int64, roleName string, connectionName string) (bool, error) {
	connName := strings.TrimSpace(connectionName)
	if connName == "" {
		return false, errors.New("连接名称不能为空")
	}

	if _, err := LoadConnectionByName(connName); err != nil {
		return false, err
	}

	if strings.EqualFold(strings.TrimSpace(roleName), "admin") {
		return true, nil
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		return false, err
	}

	var count int
	err = db.QueryRow(`
SELECT COUNT(1)
FROM user_db_connection
WHERE user_id = ? AND connection_name = ?
`, userID, connName).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
