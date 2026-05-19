package middleware

import (
	"net/http"

	"sql_platform/server/auth"

	"github.com/gin-gonic/gin"
)

/*
auth.go
----------------------------------------------------------------------
该文件定义认证与权限中间件。

当前包含两个中间件：

1. RequireLogin
   - 要求用户必须已登录
   - 用于数据库查询相关接口：
     - /api/query-connections
     - /api/query-data
     - /api/query-export-excel
     - /api/query-metadata

2. RequireAdmin
   - 要求用户必须已登录，且角色是 admin
   - 用于管理员接口：
     - /api/admin/users
     - /api/admin/db-connections

为什么这里还要写入 gin context：
- 后续业务接口不仅需要知道“有没有登录”
- 还需要知道：
  - 当前用户ID
  - 当前用户名
  - 当前显示名称
  - 当前角色
- 查询执行 / 元数据查询 / 连接列表过滤 都需要这些上下文信息
*/

// RequireLogin
// ----------------------------------------------------------------------
// 要求用户必须已登录。
//
// 核心流程：
// 1. 从 Cookie 中读取 session_token
// 2. 根据 token 查询当前登录用户
// 3. 若无效则返回 401
// 4. 若有效则把用户信息写入 gin context
func RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Cookie 中读取 session token
		token := auth.ReadSessionCookie(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "未登录或登录已失效",
			})
			return
		}

		// 通过 token 查询当前登录用户
		user, ok, err := auth.GetUserBySessionToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "登录状态校验失败：" + err.Error(),
			})
			return
		}

		// token 不存在或已失效
		if !ok {
			auth.ClearSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "未登录或登录已失效",
			})
			return
		}

		// 把当前用户信息写入 gin context
		// 这样后续接口可以直接读取：
		// - currentUserID
		// - currentUsername
		// - currentDisplayName
		// - currentRole
		c.Set("currentUserID", user.UserID)
		c.Set("currentUsername", user.Username)
		c.Set("currentDisplayName", user.DisplayName)
		c.Set("currentRole", user.RoleName)

		c.Next()
	}
}

// RequireAdmin
// ----------------------------------------------------------------------
// 要求当前用户必须是管理员。
//
// 核心流程：
// 1. 先校验用户是否已登录
// 2. 再判断角色是否为 admin
// 3. 不是 admin 则返回 403
//
// 注意：
// 前端虽然也会隐藏管理员入口，但那只是 UI 层限制。
// 真正安全控制必须在后端中间件这里做。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Cookie 中读取 session token
		token := auth.ReadSessionCookie(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "未登录或登录已失效",
			})
			return
		}

		// 查询当前登录用户
		user, ok, err := auth.GetUserBySessionToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "登录状态校验失败：" + err.Error(),
			})
			return
		}

		// token 无效或已过期
		if !ok {
			auth.ClearSessionCookie(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"ok":      false,
				"message": "未登录或登录已失效",
			})
			return
		}

		// 角色不是 admin，禁止访问
		if user.RoleName != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"ok":      false,
				"message": "仅管理员可访问",
			})
			return
		}

		// 管理员信息写入 context，供后续接口使用
		c.Set("currentUserID", user.UserID)
		c.Set("currentUsername", user.Username)
		c.Set("currentDisplayName", user.DisplayName)
		c.Set("currentRole", user.RoleName)

		c.Next()
	}
}
