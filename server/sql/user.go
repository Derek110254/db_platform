package sql

import (
	"database/sql"
	"errors"
	"strings"

	"db_platform/server/config"
)

/*
user.go
----------------------------------------------------------------------
负责账号密码、用户状态、角色和数据库连接授权关系。

主要职责：
1. 当前用户修改密码。
2. 管理员查询、新增、编辑、删除用户。
3. 管理员为普通用户分配可查询的数据库连接。
4. 管理员角色直接访问全部启用连接，普通用户按授权关系访问。
5. 在同一事务内维护用户和连接权限，删除用户时同步清理会话与收藏。
*/

var (
	ErrOldPasswordMismatch = errors.New("原密码不正确")
	ErrUsernameExists      = errors.New("用户名已存在")
	ErrUserNotFound        = errors.New("用户不存在或已被删除")
)

// UserRecord 是管理员页面使用的用户记录模型。
type UserRecord struct {
	ID                 int64    `json:"id"`
	Username           string   `json:"username"`
	DisplayName        string   `json:"displayName"`
	RoleName           string   `json:"roleName"`
	IsEnabled          int      `json:"isEnabled"`
	CanQueryData       int      `json:"canQueryData"`
	AllowedConnections []string `json:"allowedConnections"`
	CreateTime         string   `json:"createTime"`
	UpdateTime         string   `json:"updateTime"`
}

// ChangePassword 校验原密码并更新新密码。
func ChangePassword(userID int64, oldPassword, newPassword string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var storedPassword string
	if err := db.QueryRow(`SELECT fixed_aes_decrypt(password_cipher) FROM user WHERE id = ?`, userID).Scan(&storedPassword); err != nil {
		return err
	}
	if storedPassword != oldPassword {
		return ErrOldPasswordMismatch
	}

	_, err = db.Exec(`UPDATE user SET password_cipher = fixed_aes_encrypt(?), need_change_pwd = 0 WHERE id = ?`, newPassword, userID)
	return err
}

// ListUsers 返回全部未删除用户，并附带每个用户被授权的连接名称。
func ListUsers() ([]UserRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
SELECT id, username, display_name, role_name, is_enabled, can_query_data, create_time, update_time
FROM user
WHERE is_deleted = 0
ORDER BY id DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]UserRecord, 0)
	for rows.Next() {
		var rec UserRecord
		if err := rows.Scan(&rec.ID, &rec.Username, &rec.DisplayName, &rec.RoleName, &rec.IsEnabled, &rec.CanQueryData, &rec.CreateTime, &rec.UpdateTime); err != nil {
			return nil, err
		}

		names, err := ListUserAllowedConnectionNames(rec.ID)
		if err != nil {
			return nil, err
		}
		rec.AllowedConnections = names
		items = append(items, rec)
	}
	return items, rows.Err()
}

// CreateUser 创建用户。普通用户的连接权限会在同一事务中写入。
func CreateUser(username, password, displayName, roleName string, isEnabled, canQueryData int, allowedConnections []string) (int64, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return 0, err
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM user WHERE username = ? AND is_deleted = 0`, username).Scan(&exists); err != nil {
		return 0, err
	}
	if exists > 0 {
		return 0, ErrUsernameExists
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.Exec(`
INSERT INTO user (
    username,
    password_cipher,
    display_name,
    role_name,
    is_enabled,
    can_query_data
) VALUES (?, fixed_aes_encrypt(?), ?, ?, ?, ?)
`, username, password, displayName, roleName, isEnabled, canQueryData)
	if err != nil {
		return 0, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if roleName == "user" {
		if err := SaveUserAllowedConnectionsTx(tx, userID, allowedConnections); err != nil {
			return 0, err
		}
	}

	return userID, tx.Commit()
}

// UpdateUser 编辑用户。password 为空时保留原密码；admin 用户不保留连接授权关系。
func UpdateUser(id int64, username, password, displayName, roleName string, isEnabled, canQueryData int, allowedConnections []string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM user WHERE username = ? AND id <> ? AND is_deleted = 0`, username, id).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return ErrUsernameExists
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if password == "" {
		_, err = tx.Exec(`
UPDATE user
SET username = ?, display_name = ?, role_name = ?, is_enabled = ?, can_query_data = ?
WHERE id = ?
`, username, displayName, roleName, isEnabled, canQueryData, id)
	} else {
		_, err = tx.Exec(`
UPDATE user
SET username = ?, password_cipher = fixed_aes_encrypt(?), display_name = ?, role_name = ?, is_enabled = ?, can_query_data = ?
WHERE id = ?
`, username, password, displayName, roleName, isEnabled, canQueryData, id)
	}
	if err != nil {
		return err
	}

	if roleName == "user" {
		if err := SaveUserAllowedConnectionsTx(tx, id, allowedConnections); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(`DELETE FROM user_db_connection WHERE user_id = ?`, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteUser 删除用户，并清理其连接授权、SQL 收藏和会话记录。
func DeleteUser(id int64) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM user_db_connection WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM sql_favorite WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM session WHERE user_id = ?`, id); err != nil {
		return err
	}

	res, err := tx.Exec(`UPDATE user SET is_deleted = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrUserNotFound
	}

	return tx.Commit()
}

// NormalizeConnectionNames 去空、去重并保留原顺序。
func NormalizeConnectionNames(names []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)

	for _, name := range names {
		v := strings.TrimSpace(name)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

// ValidateEnabledConnectionNames 校验连接名称是否存在且启用。
func ValidateEnabledConnectionNames(names []string) error {
	normalized := NormalizeConnectionNames(names)
	if len(normalized) == 0 {
		return nil
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	for _, name := range normalized {
		var count int
		err := db.QueryRow(`
SELECT COUNT(1)
FROM db_connection
WHERE name = ? AND is_enabled = 1
`, name).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			return errors.New("连接不存在或未启用: " + name)
		}
	}

	return nil
}

// DeleteUserAllowedConnections 删除指定用户的全部连接授权。
func DeleteUserAllowedConnections(userID int64) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	_, err = db.Exec(`DELETE FROM user_db_connection WHERE user_id = ?`, userID)
	return err
}

// SaveUserAllowedConnections 保存用户连接授权。
func SaveUserAllowedConnections(userID int64, connectionNames []string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := SaveUserAllowedConnectionsTx(tx, userID, connectionNames); err != nil {
		return err
	}

	return tx.Commit()
}

// SaveUserAllowedConnectionsTx 在事务中先删除旧授权，再写入新授权。
func SaveUserAllowedConnectionsTx(tx *sql.Tx, userID int64, connectionNames []string) error {
	normalized := NormalizeConnectionNames(connectionNames)

	if _, err := tx.Exec(`DELETE FROM user_db_connection WHERE user_id = ?`, userID); err != nil {
		return err
	}

	for _, name := range normalized {
		if _, err := tx.Exec(`
INSERT INTO user_db_connection (user_id, connection_name)
VALUES (?, ?)
`, userID, name); err != nil {
			return err
		}
	}

	return nil
}

// ListUserAllowedConnectionNames 返回指定用户被授权访问的连接名称。
func ListUserAllowedConnectionNames(userID int64) ([]string, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
SELECT connection_name
FROM user_db_connection
WHERE user_id = ?
ORDER BY connection_name
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		items = append(items, name)
	}
	return items, rows.Err()
}
