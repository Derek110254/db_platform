package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

/*
middleware.go
----------------------------------------------------------------------
定义认证与管理员鉴权中间件。

RequireLogin 用于普通登录态校验：
1. 从 Cookie 读取 session token。
2. 根据 token 查询当前用户。
3. token 无效或过期时返回 401。
4. 校验成功后，将用户 ID、用户名、显示名和角色写入 gin.Context。

RequireAdmin 用于管理接口校验：
1. 先完成登录态校验。
2. 再要求当前用户角色为 admin。
3. 权限不足时返回 403。

前端会隐藏无权限入口，但真正的安全边界必须由后端中间件兜住。
*/

// RequireLogin 要求请求方必须已登录。
func RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ReadSessionCookie(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "未登录或登录已失效",
			})
			return
		}

		user, ok, err := GetUserBySessionToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "登录状态校验失败: " + err.Error(),
			})
			return
		}

		if !ok {
			ClearSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "未登录或登录已失效",
			})
			return
		}

		c.Set("currentUserID", user.UserID)
		c.Set("currentUsername", user.Username)
		c.Set("currentDisplayName", user.DisplayName)
		c.Set("currentRole", user.RoleName)

		c.Next()
	}
}

// RequireAdmin 要求请求方必须已登录且具备 admin 角色。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ReadSessionCookie(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "未登录或登录已失效",
			})
			return
		}

		user, ok, err := GetUserBySessionToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "登录状态校验失败: " + err.Error(),
			})
			return
		}

		if !ok {
			ClearSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "未登录或登录已失效",
			})
			return
		}

		if user.RoleName != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"ok":      false,
				"message": "仅管理员可访问",
			})
			return
		}

		c.Set("currentUserID", user.UserID)
		c.Set("currentUsername", user.Username)
		c.Set("currentDisplayName", user.DisplayName)
		c.Set("currentRole", user.RoleName)

		c.Next()
	}
}
