package sql

import (
	"errors"

	"sql_platform/server/auth"
	"sql_platform/server/config"
)

/*
user.go
----------------------------------------------------------------------
该文件负责平台用户管理的核心 SQL 逻辑，对应前端「用户管理」页面。

主要功能：
1. 修改当前用户密码（校验原密码）。
2. 管理员查询用户列表（含每用户的可查询连接权限）。
3. 管理员创建/编辑/删除用户（事务保证用户与连接权限的原子性）。

说明：
- HTTP 请求绑定结构体仍留在 routes/api.go，本文件只承载 DB 记录模型与 SQL。
- 用户连接权限关系（platform_user_db_connection）的维护复用 auth 包的辅助函数。
*/

// 哨兵错误，供 handler 映射对应的 HTTP 提示
var (
	ErrOldPasswordMismatch = errors.New("原密码不正确")
	ErrUsernameExists      = errors.New("用户名已存在")
	ErrUserNotFound        = errors.New("用户不存在或已被删除")
)

// UserRecord 用户记录模型
type UserRecord struct {
	ID                 int64
	Username           string
	DisplayName       string
	RoleName          string
	IsEnabled         int
	CanQueryData      int
	CanQueryPlan      int
	AllowedConnections []string // ListUsers 时由 auth.ListUserAllowedConnectionNames 填充
	CreateTime         string
	UpdateTime        string
}

// ChangePassword 校验原密码并更新为新密码。
// 原密码不匹配时返回 ErrOldPasswordMismatch。
func ChangePassword(userID int64, oldPassword, newPassword string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var storedPassword string
	if err := db.QueryRow(`SELECT fixed_aes_decrypt(password_cipher) FROM platform_user WHERE id = ?`, userID).Scan(&storedPassword); err != nil {
		return err
	}

	if storedPassword != oldPassword {
		return ErrOldPasswordMismatch
	}

	if _, err := db.Exec(`UPDATE platform_user SET password_cipher = fixed_aes_encrypt(?), need_change_pwd = 0 WHERE id = ?`, newPassword, userID); err != nil {
		return err
	}
	return nil
}

// ListUsers 查询全部未删除用户，并逐行填充其可查询连接名称列表。
func ListUsers() ([]UserRecord, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
SELECT id, username, display_name, role_name, is_enabled, can_query_data, can_query_plan, create_time, update_time
FROM platform_user
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
		if err := rows.Scan(&rec.ID, &rec.Username, &rec.DisplayName, &rec.RoleName, &rec.IsEnabled, &rec.CanQueryData, &rec.CanQueryPlan, &rec.CreateTime, &rec.UpdateTime); err != nil {
			return nil, err
		}
		names, err := auth.ListUserAllowedConnectionNames(rec.ID)
		if err != nil {
			return nil, err
		}
		rec.AllowedConnections = names
		items = append(items, rec)
	}
	return items, nil
}

// CreateUser 创建用户。role 为 user 时在同一事务内保存其可查询连接权限。
// 用户名已存在时返回 ErrUsernameExists。返回新用户 ID。
func CreateUser(username, password, displayName, roleName string, isEnabled, canQueryData, canQueryPlan int, allowedConnections []string) (int64, error) {
	db, err := config.GetPlatformDB()
	if err != nil {
		return 0, err
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM platform_user WHERE username = ? AND is_deleted = 0`, username).Scan(&exists); err != nil {
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
INSERT INTO platform_user (
    username,
    password_cipher,
    display_name,
    role_name,
    is_enabled,
    can_query_data,
    can_query_plan
) VALUES (
    ?,
    fixed_aes_encrypt(?),
    ?,
    ?,
    ?,
    ?,
    ?
)
`, username, password, displayName, roleName, isEnabled, canQueryData, canQueryPlan)
	if err != nil {
		return 0, err
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if roleName == "user" {
		if err := auth.SaveUserAllowedConnectionsTx(tx, userID, allowedConnections); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return userID, nil
}

// UpdateUser 编辑用户。password 为空时不修改密码。
// 用户名被其他用户占用时返回 ErrUsernameExists。role 为 admin 时清空其连接权限关系。
func UpdateUser(id int64, username, password, displayName, roleName string, isEnabled, canQueryData, canQueryPlan int, allowedConnections []string) error {
	db, err := config.GetPlatformDB()
	if err != nil {
		return err
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM platform_user WHERE username = ? AND id <> ? AND is_deleted = 0`, username, id).Scan(&exists); err != nil {
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
UPDATE platform_user
SET username = ?, display_name = ?, role_name = ?, is_enabled = ?, can_query_data = ?, can_query_plan = ?
WHERE id = ?
`, username, displayName, roleName, isEnabled, canQueryData, canQueryPlan, id)
	} else {
		_, err = tx.Exec(`
UPDATE platform_user
SET username = ?, password_cipher = fixed_aes_encrypt(?), display_name = ?, role_name = ?, is_enabled = ?, can_query_data = ?, can_query_plan = ?
WHERE id = ?
`, username, password, displayName, roleName, isEnabled, canQueryData, canQueryPlan, id)
	}
	if err != nil {
		return err
	}

	if roleName == "user" {
		if err := auth.SaveUserAllowedConnectionsTx(tx, id, allowedConnections); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(`DELETE FROM platform_user_db_connection WHERE user_id = ?`, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteUser 删除用户（级联清理连接权限、SQL 收藏、会话，再软删用户）。
// 用户不存在时返回 ErrUserNotFound。
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

	if _, err := tx.Exec(`DELETE FROM platform_user_db_connection WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM platform_sql_favorite WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM platform_session WHERE user_id = ?`, id); err != nil {
		return err
	}

	res, err := tx.Exec(`UPDATE platform_user SET is_deleted = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrUserNotFound
	}

	return tx.Commit()
}
