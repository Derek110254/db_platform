package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"time"

	"db_platform/server/config"

	"github.com/gin-gonic/gin"
)

/*
session.go
----------------------------------------------------------------------
负责数据库查询平台的账号登录、会话写入、会话读取和过期会话清理。

表结构、加解密函数和默认管理员都由 init.sql 初始化
*/

// SessionUser 表示当前登录用户的会话视图。
type SessionUser struct {
	UserID        int64  `json:"userId"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	RoleName      string `json:"roleName"`
	CanQueryData  int    `json:"canQueryData"`
	NeedChangePwd int    `json:"needChangePwd"`
}

func getAuthDB() (*sql.DB, error) {
	return config.GetPlatformDB()
}

// ReadSessionCookie 从请求 Cookie 中读取会话 token。
func ReadSessionCookie(c *gin.Context) string {
	sessionConfig := config.GetSessionConfig()
	val, err := c.Cookie(sessionConfig.CookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

// WriteSessionCookie 将会话 token 写入浏览器 Cookie。
func WriteSessionCookie(c *gin.Context, token string) {
	sessionConfig := config.GetSessionConfig()
	maxAge := sessionConfig.ExpireHours * 3600
	c.SetCookie(sessionConfig.CookieName, token, maxAge, "/", "", false, true)
}

// ClearSessionCookie 清除浏览器中的会话 Cookie。
func ClearSessionCookie(c *gin.Context) {
	sessionConfig := config.GetSessionConfig()
	c.SetCookie(sessionConfig.CookieName, "", -1, "/", "", false, true)
}

// Login 使用账号密码登录，并在校验通过后创建一条会话记录。
func Login(username string, password string) (SessionUser, string, error) {
	db, err := getAuthDB()
	if err != nil {
		return SessionUser{}, "", err
	}

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return SessionUser{}, "", errors.New("用户名和密码不能为空")
	}

	var user SessionUser
	var storedPassword string
	var isEnabled int

	query := `
SELECT
    id,
    username,
    display_name,
    role_name,
    fixed_aes_decrypt(password_cipher) AS plain_password,
    is_enabled,
    can_query_data,
    need_change_pwd
FROM user
WHERE username = ? AND is_deleted = 0
LIMIT 1
`

	err = db.QueryRow(query, username).Scan(
		&user.UserID,
		&user.Username,
		&user.DisplayName,
		&user.RoleName,
		&storedPassword,
		&isEnabled,
		&user.CanQueryData,
		&user.NeedChangePwd,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SessionUser{}, "", errors.New("用户名或密码错误")
		}
		return SessionUser{}, "", err
	}

	if isEnabled != 1 {
		return SessionUser{}, "", errors.New("当前用户已被禁用")
	}
	if strings.TrimSpace(storedPassword) == "" {
		return SessionUser{}, "", errors.New("用户密码密文解密失败")
	}
	if storedPassword != password {
		return SessionUser{}, "", errors.New("用户名或密码错误")
	}

	token, err := generateSessionToken()
	if err != nil {
		return SessionUser{}, "", err
	}

	sessionConfig := config.GetSessionConfig()
	expireTime := time.Now().Add(time.Duration(sessionConfig.ExpireHours) * time.Hour)
	_, err = db.Exec(`
INSERT INTO session (session_token, user_id, username, expire_time)
VALUES (?, ?, ?, ?)
`, token, user.UserID, user.Username, expireTime)
	if err != nil {
		return SessionUser{}, "", err
	}

	_ = DeleteExpiredSessions()
	return user, token, nil
}

// GetUserBySessionToken 根据会话 token 查询当前登录用户。
func GetUserBySessionToken(token string) (SessionUser, bool, error) {
	db, err := getAuthDB()
	if err != nil {
		return SessionUser{}, false, err
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return SessionUser{}, false, nil
	}

	query := `
SELECT u.id, u.username, u.display_name, u.role_name, u.can_query_data, u.need_change_pwd
FROM session s
INNER JOIN user u ON s.user_id = u.id
WHERE s.session_token = ?
  AND s.expire_time > NOW()
  AND u.is_enabled = 1
  AND u.is_deleted = 0
LIMIT 1
`

	var user SessionUser
	err = db.QueryRow(query, token).Scan(
		&user.UserID,
		&user.Username,
		&user.DisplayName,
		&user.RoleName,
		&user.CanQueryData,
		&user.NeedChangePwd,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SessionUser{}, false, nil
		}
		return SessionUser{}, false, err
	}

	return user, true, nil
}

// DeleteSessionByToken 删除指定会话 token。
func DeleteSessionByToken(token string) error {
	db, err := getAuthDB()
	if err != nil {
		return err
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}

	_, err = db.Exec(`DELETE FROM session WHERE session_token = ?`, token)
	return err
}

// DeleteExpiredSessions 删除所有已过期会话。
func DeleteExpiredSessions() error {
	db, err := getAuthDB()
	if err != nil {
		return err
	}

	_, err = db.Exec(`DELETE FROM session WHERE expire_time <= NOW()`)
	return err
}

// generateSessionToken 生成随机、URL 安全的会话 token。
func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
