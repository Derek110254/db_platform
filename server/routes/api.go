package routes

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sql_platform/server/auth"
	"sql_platform/server/config"
	"sql_platform/server/middleware"
	appsql "sql_platform/server/sql"

	"github.com/gin-gonic/gin"
)

/*
api.go
----------------------------------------------------------------------
该文件负责注册并实现所有后端 API 路由。

主要功能模块包括：
1. 用户认证与基础功能：登录、登出、SSO登录、获取当前用户信息、修改密码等。
2. SQL 操作核心功能：SQL/DDL 语法与风险检测、数据库连接查询、SQL 执行、执行计划 (Explain)、导出 Excel、元数据查询。
3. SQL 收藏管理：个人收藏的增删改查。
4. SQL 审核历史：提交审核、查看个人审核记录；管理员审核及记录管理。
5. 数据库变更申请：普通用户提交变更申请；管理员发布验证。
6. 数据库数据同步申请：普通用户提交数据同步申请；管理员(DBA)审核执行。
7. 团队数据库环境管理：普通用户查看环境；管理员维护团队环境信息。
8. 管理员核心配置：用户管理、数据库连接配置管理与连通性测试。

路由权限说明：
- `/api/admin/*` 路由组：要求必须具备 admin 角色权限 (RequireAdmin 拦截器)。
- `/api/*` (除白名单外)：要求必须已登录 (RequireLogin 拦截器)。
- 白名单路由：`/api/hello`, `/api/login`, `/api/sso-login`, `/api/check-sql`, `/api/check-ddl`。
*/

// ----------------------------------------------------------------------
// 一、请求结构
// ----------------------------------------------------------------------

type SQLCheckRequest struct {
	DBType string `json:"dbType"`
	SQL    string `json:"sql"`
}

type QueryExecuteRequest struct {
	ConnectionName string `json:"connectionName"`
	SQL            string `json:"sql"`
}

type QueryPlanRequest struct {
	ConnectionName string `json:"connectionName"`
	SQL            string `json:"sql"`
}

type QueryMetadataRequest struct {
	ConnectionName string `json:"connectionName"`
	Keyword        string `json:"keyword"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminCreateUserRequest struct {
	Username           string   `json:"username"`
	Password           string   `json:"password"`
	DisplayName        string   `json:"displayName"`
	RoleName           string   `json:"roleName"`
	IsEnabled          int      `json:"isEnabled"`
	CanQueryData       int      `json:"canQueryData"`
	CanQueryPlan       int      `json:"canQueryPlan"`
	AllowedConnections []string `json:"allowedConnections"`
}

type AdminUpdateUserRequest struct {
	ID                 int64    `json:"id"`
	Username           string   `json:"username"`
	Password           string   `json:"password"`
	DisplayName        string   `json:"displayName"`
	RoleName           string   `json:"roleName"`
	IsEnabled          int      `json:"isEnabled"`
	CanQueryData       int      `json:"canQueryData"`
	CanQueryPlan       int      `json:"canQueryPlan"`
	AllowedConnections []string `json:"allowedConnections"`
}

type AdminDeleteUserRequest struct {
	ID int64 `json:"id"`
}

type AdminCreateConnectionRequest struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	DBType       string `json:"dbType"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DatabaseName string `json:"databaseName"`
	ServiceName  string `json:"serviceName"`
	IsEnabled    int    `json:"isEnabled"`
}

type AdminUpdateConnectionRequest struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Label        string `json:"label"`
	DBType       string `json:"dbType"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DatabaseName string `json:"databaseName"`
	ServiceName  string `json:"serviceName"`
	IsEnabled    int    `json:"isEnabled"`
}

type AdminDeleteConnectionRequest struct {
	ID int64 `json:"id"`
}

type AdminTestConnectionRequest struct {
	ID           int64  `json:"id"`
	DBType       string `json:"dbType"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DatabaseName string `json:"databaseName"`
	ServiceName  string `json:"serviceName"`
}

// SQLFavoriteCreateRequest
// ----------------------------------------------------------------------
// 新增 SQL 收藏请求
type SQLFavoriteCreateRequest struct {
	FavoriteName   string `json:"favoriteName"`
	SQLText        string `json:"sqlText"`
	DBType         string `json:"dbType"`
	ConnectionName string `json:"connectionName"`
	Remark         string `json:"remark"`
	IsPinned       int    `json:"isPinned"`
}

// SQLFavoriteUpdateRequest
// ----------------------------------------------------------------------
// 编辑 SQL 收藏请求
type SQLFavoriteUpdateRequest struct {
	ID             int64  `json:"id"`
	FavoriteName   string `json:"favoriteName"`
	SQLText        string `json:"sqlText"`
	DBType         string `json:"dbType"`
	ConnectionName string `json:"connectionName"`
	Remark         string `json:"remark"`
	IsPinned       int    `json:"isPinned"`
}

// SQLFavoriteDeleteRequest
// ----------------------------------------------------------------------
// 删除 SQL 收藏请求
type SQLFavoriteDeleteRequest struct {
	ID int64 `json:"id"`
}

// DBChangeRequestCreateReq
// ----------------------------------------------------------------------
// 新增数据库变更申请请求
type DBChangeRequestCreateReq struct {
	ApplicantTeam     string `json:"applicantTeam" binding:"required"`
	PlannedChangeTime string `json:"plannedChangeTime" binding:"required"`
	UrgencyLevel      string `json:"urgencyLevel" binding:"required"`
	ChangeType        string `json:"changeType" binding:"required"`
	ChangeReason      string `json:"changeReason" binding:"required"`
	RequirementUrl    string `json:"requirementUrl"`
	ImpactScope       string `json:"impactScope"`
	DbType            string `json:"dbType" binding:"required"`
	TestDbIp          string `json:"testDbIp" binding:"required"`
	TestDbName        string `json:"testDbName"`
	TestDbSchema      string `json:"testDbSchema"`
	DbIp              string `json:"dbIp" binding:"required"`
	DbName            string `json:"dbName"`
	DbSchema          string `json:"dbSchema"`
	ChangeContent     string `json:"changeContent" binding:"required"`
}

// DBChangeRequestUpdateReq
// ----------------------------------------------------------------------
// 编辑数据库变更申请请求
type DBChangeRequestUpdateReq struct {
	ID                int64  `json:"id" binding:"required"`
	ApplicantTeam     string `json:"applicantTeam" binding:"required"`
	Environment       string `json:"environment"`
	PlannedChangeTime string `json:"plannedChangeTime" binding:"required"`
	UrgencyLevel      string `json:"urgencyLevel" binding:"required"`
	ChangeType        string `json:"changeType" binding:"required"`
	ChangeReason      string `json:"changeReason" binding:"required"`
	RequirementUrl    string `json:"requirementUrl"`
	ImpactScope       string `json:"impactScope"`
	DbType            string `json:"dbType" binding:"required"`
	TestDbIp          string `json:"testDbIp" binding:"required"`
	TestDbName        string `json:"testDbName"`
	TestDbSchema      string `json:"testDbSchema"`
	DbIp              string `json:"dbIp" binding:"required"`
	DbName            string `json:"dbName"`
	DbSchema          string `json:"dbSchema"`
	ChangeContent     string `json:"changeContent" binding:"required"`
}

// DBChangeRequestReleaseReq
// ----------------------------------------------------------------------
// 数据库变更申请发布验证请求
type DBChangeRequestReleaseReq struct {
	ID              int64  `json:"id" binding:"required"`
	TestPublisher   string `json:"testPublisher"`
	ProdPublisher   string `json:"prodPublisher"`
	ReleaseVerifier string `json:"releaseVerifier"`
}

// DBChangeRequestDeleteReq
// ----------------------------------------------------------------------
// 删除数据库变更申请请求
type DBChangeRequestDeleteReq struct {
	ID int64 `json:"id" binding:"required"`
}

// DBDataSyncRequestCreateReq
// ----------------------------------------------------------------------
// 新增数据库数据同步申请请求
type DBDataSyncRequestCreateReq struct {
	ApplicantTeam         string `json:"applicantTeam"`
	Environment           string `json:"environment"`
	ExpectedFinishTime    string `json:"expectedFinishTime" binding:"required"`
	UrgencyLevel          string `json:"urgencyLevel" binding:"required"`
	UrgencyReason         string `json:"urgencyReason"`
	ApplyReason           string `json:"applyReason" binding:"required"`
	OperateType           int    `json:"operateType" binding:"required"`
	SourceDb              string `json:"sourceDb" binding:"required"`
	TargetDbOrPerson      string `json:"targetDbOrPerson" binding:"required"`
	InvolvedDbSchemaTable string `json:"involvedDbSchemaTable" binding:"required"`
	DataFilterCondition   string `json:"dataFilterCondition"`
	EstimatedDataVolume   string `json:"estimatedDataVolume" binding:"required"`
	ContainsSensitiveData int    `json:"containsSensitiveData"`
	DesensitizationRule   string `json:"desensitizationRule"`
}

// DBDataSyncRequestUpdateReq
// ----------------------------------------------------------------------
// 编辑数据库数据同步申请请求
type DBDataSyncRequestUpdateReq struct {
	ID                    int64  `json:"id" binding:"required"`
	ApplicantTeam         string `json:"applicantTeam"`
	Environment           string `json:"environment"`
	ExpectedFinishTime    string `json:"expectedFinishTime" binding:"required"`
	UrgencyLevel          string `json:"urgencyLevel" binding:"required"`
	UrgencyReason         string `json:"urgencyReason"`
	ApplyReason           string `json:"applyReason" binding:"required"`
	OperateType           int    `json:"operateType" binding:"required"`
	SourceDb              string `json:"sourceDb" binding:"required"`
	TargetDbOrPerson      string `json:"targetDbOrPerson" binding:"required"`
	InvolvedDbSchemaTable string `json:"involvedDbSchemaTable" binding:"required"`
	DataFilterCondition   string `json:"dataFilterCondition"`
	EstimatedDataVolume   string `json:"estimatedDataVolume" binding:"required"`
	ContainsSensitiveData int    `json:"containsSensitiveData"`
	DesensitizationRule   string `json:"desensitizationRule"`
}

// DBDataSyncRequestDeleteReq
// ----------------------------------------------------------------------
// 删除数据库数据同步申请请求
type DBDataSyncRequestDeleteReq struct {
	ID int64 `json:"id" binding:"required"`
}

// DBDataSyncRequestDbaReq
// ----------------------------------------------------------------------
// 数据库数据同步申请DBA执行更新请求
type DBDataSyncRequestDbaReq struct {
	ID         int64  `json:"id" binding:"required"`
	ExecuteDba string `json:"executeDba" binding:"required"`
}

// RegisterAPIRoutes
// ----------------------------------------------------------------------
// 注册所有 API 路由
func RegisterAPIRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		// 基础测试接口
		api.GET("/hello", helloHandler)

		// 语法检测接口（不需要登录）
		api.POST("/check-sql", checkSQLHandler) // 检测查询 SQL 的语法及风险
		api.POST("/check-ddl", checkDDLHandler) // 检测 DDL 语句的语法风险

		// 认证相关接口
		api.POST("/login", loginHandler)        // 账密登录
		api.POST("/sso-login", ssoLoginHandler) // SSO 登录
		api.POST("/logout", logoutHandler)      // 注销登录
		api.GET("/auth/me", authMeHandler)      // 获取当前登录用户信息

		queryGroup := api.Group("/")
		queryGroup.Use(middleware.RequireLogin())
		{
			// 个人基础操作
			queryGroup.POST("/user/change-password", changePasswordHandler) // 用户修改个人密码

			// 核心查询操作
			queryGroup.GET("/query-connections", queryConnectionsHandler)   // 获取当前用户有权访问的数据库连接列表
			queryGroup.POST("/query-data", queryDataHandler)                // 执行查询 SQL 获取数据结果
			queryGroup.POST("/query-plan", queryPlanHandler)                // 获取 SQL 的执行计划 (Explain)
			queryGroup.POST("/query-export-excel", queryExportExcelHandler) // 导出 SQL 查询结果为 Excel
			queryGroup.POST("/query-metadata", queryMetadataHandler)        // 查询数据库表及字段元数据

			// SQL 收藏接口：只要求已登录
			queryGroup.GET("/sql-favorites", listSQLFavoritesHandler)     // 获取个人 SQL 收藏列表
			queryGroup.POST("/sql-favorites", createSQLFavoriteHandler)   // 添加新的 SQL 收藏
			queryGroup.PUT("/sql-favorites", updateSQLFavoriteHandler)    // 修改已有的 SQL 收藏
			queryGroup.DELETE("/sql-favorites", deleteSQLFavoriteHandler) // 删除 SQL 收藏

			// SQL 审核历史接口
			queryGroup.GET("/audit-history", listAuditHistoryHandler) // 获取个人的 SQL 审核历史记录
			queryGroup.POST("/audit-submit", submitAuditHandler)      // 提交待审核的 SQL 记录

			// 数据库变更申请接口
			queryGroup.GET("/db-change-requests", listDBChangeRequestsHandler)     // 获取个人提交的数据库变更申请列表
			queryGroup.POST("/db-change-requests", createDBChangeRequestHandler)   // 提交数据库变更申请
			queryGroup.PUT("/db-change-requests", updateDBChangeRequestHandler)    // 修改自己提交的变更申请
			queryGroup.DELETE("/db-change-requests", deleteDBChangeRequestHandler) // 删除自己提交的变更申请

			// 数据库数据同步申请接口
			queryGroup.GET("/db-data-sync-requests", listDBDataSyncRequestsHandler)     // 获取个人提交的数据同步申请列表
			queryGroup.POST("/db-data-sync-requests", createDBDataSyncRequestHandler)   // 提交数据同步申请
			queryGroup.PUT("/db-data-sync-requests", updateDBDataSyncRequestHandler)    // 修改自己提交的数据同步申请
			queryGroup.DELETE("/db-data-sync-requests", deleteDBDataSyncRequestHandler) // 删除自己提交的数据同步申请

			// 团队数据库环境接口（允许普通用户读取）
			queryGroup.GET("/team-db-envs", listAllTeamDbEnvsForUserHandler) // 用户查看所有团队数据库环境信息
		}

		adminGroup := api.Group("/admin")
		adminGroup.Use(middleware.RequireAdmin())
		{
			// 用户管理（管理员功能）
			adminGroup.GET("/users", adminListUsersHandler)     // 获取系统所有用户列表
			adminGroup.POST("/users", adminCreateUserHandler)   // 创建新用户
			adminGroup.PUT("/users", adminUpdateUserHandler)    // 更新用户信息（权限、角色等）
			adminGroup.DELETE("/users", adminDeleteUserHandler) // 删除用户

			// 数据库连接配置管理（管理员功能）
			adminGroup.GET("/db-connections", adminListConnectionsHandler)      // 获取所有系统数据库连接配置
			adminGroup.POST("/db-connections", adminCreateConnectionHandler)    // 创建新的数据库连接
			adminGroup.PUT("/db-connections", adminUpdateConnectionHandler)     // 更新数据库连接配置
			adminGroup.DELETE("/db-connections", adminDeleteConnectionHandler)  // 删除数据库连接配置
			adminGroup.POST("/db-connections/test", adminTestConnectionHandler) // 测试指定数据库连接的连通性

			// SQL 审核管理（管理员功能）
			adminGroup.GET("/audits", adminListSubmittedAuditsHandler) // 获取所有已提交待审核的记录
			adminGroup.PUT("/audits/review", adminReviewAuditHandler)  // 管理员执行审核操作（通过/驳回）

			// 数据库变更发布管理（管理员功能）
			adminGroup.GET("/db-change-requests/release", adminListReleaseDBChangeRequestsHandler) // 获取所有变更申请记录（用于发布验证）
			adminGroup.PUT("/db-change-requests/release", adminReleaseDBChangeRequestHandler)      // 填写发布人及验证信息

			// 数据库数据同步申请管理员(DBA)接口
			adminGroup.GET("/db-data-sync-requests/dba", adminListDBDataSyncRequestsForDbaHandler) // DBA获取所有数据同步申请
			adminGroup.PUT("/db-data-sync-requests/dba", adminUpdateDBDataSyncRequestDbaHandler)   // DBA更新数据同步申请的执行状态

			// 团队数据库环境管理（管理员功能）
			adminGroup.GET("/team-db-envs", adminListTeamDbEnvsHandler)        // 分页获取团队数据库环境配置
			adminGroup.GET("/team-db-envs/all", adminListAllTeamDbEnvsHandler) // 获取全量团队数据库环境配置
			adminGroup.POST("/team-db-envs", adminCreateTeamDbEnvHandler)      // 新增团队数据库环境
			adminGroup.PUT("/team-db-envs", adminUpdateTeamDbEnvHandler)       // 修改团队数据库环境
			adminGroup.DELETE("/team-db-envs", adminDeleteTeamDbEnvHandler)    // 删除团队数据库环境
		}
	}
}

func helloHandler(c *gin.Context) {
	name := c.Query("name")
	if strings.TrimSpace(name) == "" {
		name = "访客"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "你好，" + name + "！欢迎使用SQL管理平台",
	})
}

func checkSQLHandler(c *gin.Context) {
	var req SQLCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":            false,
			"dbType":        req.DBType,
			"syntaxMessage": "请求格式错误",
			"syntaxErrors":  []any{},
			"riskLevel":     "low",
			"riskScore":     0,
			"riskMessage":   "未执行风险检测",
			"riskItems":     []any{},
			"error":         err.Error(),
		})
		return
	}

	result := appsql.CheckSQL(req.DBType, req.SQL)
	c.JSON(http.StatusOK, result)
}

func checkDDLHandler(c *gin.Context) {
	var req SQLCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":         false,
			"dbType":     req.DBType,
			"ddlMessage": "请求格式错误",
			"issueCount": 0,
			"issues":     []any{},
			"error":      err.Error(),
		})
		return
	}

	result := appsql.CheckDDL(req.DBType, req.SQL)
	c.JSON(http.StatusOK, result)
}

func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	user, token, err := auth.Login(strings.TrimSpace(req.Username), req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	auth.WriteSessionCookie(c, token)

	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"message":       "登录成功",
		"username":      user.Username,
		"userId":        user.UserID,
		"role":          user.RoleName,
		"canQueryData":  user.CanQueryData,
		"canQueryPlan":  user.CanQueryPlan,
		"needChangePwd": user.NeedChangePwd,
	})
}

type SSOLoginRequest struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

func ssoLoginHandler(c *gin.Context) {
	var req SSOLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "用户名不能为空",
		})
		return
	}

	// 使用拿到的 username 查 platform_user 获取权限并生成 session
	user, token, err := auth.LoginByUsername(username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	auth.WriteSessionCookie(c, token)

	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"message":       "登录成功",
		"result":        gin.H{"token": token},
		"username":      user.Username,
		"userId":        user.UserID,
		"role":          user.RoleName,
		"canQueryData":  user.CanQueryData,
		"canQueryPlan":  user.CanQueryPlan,
		"needChangePwd": user.NeedChangePwd,
	})
}

func logoutHandler(c *gin.Context) {
	token := auth.ReadSessionCookie(c)
	if token != "" {
		_ = auth.DeleteSessionByToken(token)
	}
	auth.ClearSessionCookie(c)

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "已退出登录",
	})
}

func authMeHandler(c *gin.Context) {
	token := auth.ReadSessionCookie(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": "未登录",
		})
		return
	}

	user, ok, err := auth.GetUserBySessionToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": "登录状态校验失败：" + err.Error(),
		})
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": "未登录或登录已失效",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"message":       "已登录",
		"userId":        user.UserID,
		"username":      user.Username,
		"displayName":   user.DisplayName,
		"role":          user.RoleName,
		"canQueryData":  user.CanQueryData,
		"canQueryPlan":  user.CanQueryPlan,
		"needChangePwd": user.NeedChangePwd,
	})
}

// ChangePasswordRequest
// ----------------------------------------------------------------------
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func changePasswordHandler(c *gin.Context) {
	userID, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": "未登录",
		})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "新旧密码不能为空",
		})
		return
	}

	if req.OldPassword == req.NewPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "新密码不能与原密码一致",
		})
		return
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	var storedPassword string
	err = db.QueryRow(`SELECT fixed_aes_decrypt(password_cipher) FROM platform_user WHERE id = ?`, userID).Scan(&storedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "查询原密码失败：" + err.Error(),
		})
		return
	}

	if storedPassword != req.OldPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "原密码不正确",
		})
		return
	}

	_, err = db.Exec(`UPDATE platform_user SET password_cipher = fixed_aes_encrypt(?), need_change_pwd = 0 WHERE id = ?`, req.NewPassword, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "更新密码失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "密码修改成功",
	})
}

// getCurrentUserContext
// ----------------------------------------------------------------------
// 从 gin context 中读取当前登录用户信息
func getCurrentUserContext(c *gin.Context) (int64, string, bool) {
	userIDVal, ok1 := c.Get("currentUserID")
	roleVal, ok2 := c.Get("currentRole")
	if !ok1 || !ok2 {
		return 0, "", false
	}

	userID, ok := userIDVal.(int64)
	if !ok {
		return 0, "", false
	}

	roleName, ok := roleVal.(string)
	if !ok {
		return 0, "", false
	}

	return userID, roleName, true
}

// ----------------------------------------------------------------------
// 查询相关接口
// ----------------------------------------------------------------------

func queryConnectionsHandler(c *gin.Context) {
	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":          false,
			"message":     "登录状态失效",
			"connections": []any{},
		})
		return
	}

	connections, err := auth.LoadEnabledConnectionsForUser(userID, roleName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":          false,
			"message":     err.Error(),
			"connections": []any{},
		})
		return
	}

	items := make([]gin.H, 0, len(connections))
	for _, item := range connections {
		items = append(items, gin.H{
			"name":        item.Name,
			"label":       item.Label,
			"dbType":      item.DBType,
			"host":        item.Host,
			"port":        item.Port,
			"database":    item.DatabaseName,
			"serviceName": item.ServiceName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"message":     "查询连接列表获取成功",
		"connections": items,
	})
}

func queryDataHandler(c *gin.Context) {
	var req QueryExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":        false,
			"message":   "请求格式错误：" + err.Error(),
			"columns":   []string{},
			"rows":      []any{},
			"rowCount":  0,
			"elapsedMs": 0,
		})
		return
	}

	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":        false,
			"message":   "登录状态失效",
			"columns":   []string{},
			"rows":      []any{},
			"rowCount":  0,
			"elapsedMs": 0,
		})
		return
	}

	result := appsql.ExecuteQueryByConnectionWithContext(
		c.Request.Context(),
		userID,
		roleName,
		req.ConnectionName,
		req.SQL,
	)
	c.JSON(http.StatusOK, result)
}

func queryPlanHandler(c *gin.Context) {
	var req QueryPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":        false,
			"message":   "请求格式错误：" + err.Error(),
			"columns":   []string{},
			"rows":      []any{},
			"rowCount":  0,
			"elapsedMs": 0,
		})
		return
	}

	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":        false,
			"message":   "登录状态失效",
			"columns":   []string{},
			"rows":      []any{},
			"rowCount":  0,
			"elapsedMs": 0,
		})
		return
	}

	result := appsql.ExplainQueryByConnectionWithContext(
		c.Request.Context(),
		userID,
		roleName,
		req.ConnectionName,
		req.SQL,
	)
	c.JSON(http.StatusOK, result)
}

func queryExportExcelHandler(c *gin.Context) {
	var req QueryExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": "登录状态失效",
		})
		return
	}

	fileName, content, err := appsql.ExportQueryResultToExcelWithContext(
		c.Request.Context(),
		userID,
		roleName,
		req.ConnectionName,
		req.SQL,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", `attachment; filename="`+fileName+`"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

func queryMetadataHandler(c *gin.Context) {
	var req QueryMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
			"tables":  []any{},
			"columns": []any{},
		})
		return
	}

	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": "登录状态失效",
			"tables":  []any{},
			"columns": []any{},
		})
		return
	}

	result := appsql.SearchQueryMetadataByConnectionWithUser(
		userID,
		roleName,
		req.ConnectionName,
		req.Keyword,
	)
	c.JSON(http.StatusOK, result)
}

// ----------------------------------------------------------------------
// SQL 收藏接口
// ----------------------------------------------------------------------

// listSQLFavoritesHandler
// ----------------------------------------------------------------------
// 获取当前用户自己的 SQL 收藏列表。
// 支持通过 query 参数筛选：
// 1. dbType
// 2. connectionName
// 3. keyword
func listSQLFavoritesHandler(c *gin.Context) {
	userID, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":        false,
			"message":   "登录状态失效",
			"favorites": []any{},
		})
		return
	}

	dbType := strings.TrimSpace(c.Query("dbType"))
	connectionName := strings.TrimSpace(c.Query("connectionName"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	items, err := appsql.ListSQLFavorites(userID, dbType, connectionName, keyword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":        false,
			"message":   err.Error(),
			"favorites": []any{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"message":   "SQL 收藏列表获取成功",
		"favorites": items,
	})
}

// createSQLFavoriteHandler
// ----------------------------------------------------------------------
// 新增当前用户自己的 SQL 收藏
func createSQLFavoriteHandler(c *gin.Context) {
	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": "登录状态失效",
		})
		return
	}

	var req SQLFavoriteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	err := appsql.CreateSQLFavorite(userID, roleName, appsql.SQLFavorite{
		FavoriteName:   req.FavoriteName,
		SQLText:        req.SQLText,
		DBType:         req.DBType,
		ConnectionName: req.ConnectionName,
		Remark:         req.Remark,
		IsPinned:       req.IsPinned,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "SQL 收藏成功",
	})
}

// updateSQLFavoriteHandler
// ----------------------------------------------------------------------
// 编辑当前用户自己的 SQL 收藏
func updateSQLFavoriteHandler(c *gin.Context) {
	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": "登录状态失效",
		})
		return
	}

	var req SQLFavoriteUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	err := appsql.UpdateSQLFavorite(userID, roleName, appsql.SQLFavorite{
		ID:             req.ID,
		FavoriteName:   req.FavoriteName,
		SQLText:        req.SQLText,
		DBType:         req.DBType,
		ConnectionName: req.ConnectionName,
		Remark:         req.Remark,
		IsPinned:       req.IsPinned,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "SQL 收藏更新成功",
	})
}

// deleteSQLFavoriteHandler
// ----------------------------------------------------------------------
// 删除当前用户自己的 SQL 收藏
func deleteSQLFavoriteHandler(c *gin.Context) {
	userID, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"ok":      false,
			"message": "登录状态失效",
		})
		return
	}

	var req SQLFavoriteDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	if err := appsql.DeleteSQLFavorite(userID, req.ID); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "SQL 收藏删除成功",
	})
}

// ----------------------------------------------------------------------
// 管理员：用户管理
// ----------------------------------------------------------------------

func adminListUsersHandler(c *gin.Context) {
	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	rows, err := db.Query(`
SELECT id, username, display_name, role_name, is_enabled, can_query_data, can_query_plan, create_time, update_time
FROM platform_user
WHERE is_deleted = 0
ORDER BY id DESC
`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "查询用户列表失败：" + err.Error(),
		})
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var username, displayName, roleName string
		var isEnabled, canQueryData, canQueryPlan int
		var createTime, updateTime string

		if err := rows.Scan(&id, &username, &displayName, &roleName, &isEnabled, &canQueryData, &canQueryPlan, &createTime, &updateTime); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":      false,
				"message": "读取用户数据失败：" + err.Error(),
			})
			return
		}

		allowedConnections, err := auth.ListUserAllowedConnectionNames(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":      false,
				"message": "读取用户连接权限失败：" + err.Error(),
			})
			return
		}

		items = append(items, gin.H{
			"id":                 id,
			"username":           username,
			"displayName":        displayName,
			"roleName":           roleName,
			"isEnabled":          isEnabled,
			"canQueryData":       canQueryData,
			"canQueryPlan":       canQueryPlan,
			"allowedConnections": allowedConnections,
			"createTime":         createTime,
			"updateTime":         updateTime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":    true,
		"users": items,
	})
}

func adminCreateUserHandler(c *gin.Context) {
	var req AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.RoleName = strings.TrimSpace(req.RoleName)
	req.AllowedConnections = auth.NormalizeConnectionNames(req.AllowedConnections)

	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "用户名和密码不能为空",
		})
		return
	}

	if req.RoleName == "" {
		req.RoleName = "user"
	}
	if req.RoleName != "admin" && req.RoleName != "user" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "角色只能是 admin 或 user",
		})
		return
	}

	if req.RoleName == "user" && len(req.AllowedConnections) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "普通用户必须分配至少一个可查询数据库连接",
		})
		return
	}

	if err := auth.ValidateEnabledConnectionNames(req.AllowedConnections); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	if req.IsEnabled != 0 {
		req.IsEnabled = 1
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM platform_user WHERE username = ? AND is_deleted = 0`, req.Username).Scan(&exists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "校验用户名失败：" + err.Error(),
		})
		return
	}
	if exists > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "用户名已存在",
		})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "开启事务失败：" + err.Error(),
		})
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()

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
`, req.Username, req.Password, req.DisplayName, req.RoleName, req.IsEnabled, req.CanQueryData, req.CanQueryPlan)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "创建用户失败：" + err.Error(),
		})
		return
	}

	userID, err := result.LastInsertId()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "获取新用户ID失败：" + err.Error(),
		})
		return
	}

	if req.RoleName == "user" {
		if err := auth.SaveUserAllowedConnectionsTx(tx, userID, req.AllowedConnections); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":      false,
				"message": "保存用户连接权限失败：" + err.Error(),
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "提交事务失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "创建用户成功",
	})
}

func adminUpdateUserHandler(c *gin.Context) {
	var req AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.RoleName = strings.TrimSpace(req.RoleName)
	req.AllowedConnections = auth.NormalizeConnectionNames(req.AllowedConnections)

	if req.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "用户ID不能为空",
		})
		return
	}
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "用户名不能为空",
		})
		return
	}
	if req.RoleName != "admin" && req.RoleName != "user" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "角色只能是 admin 或 user",
		})
		return
	}
	if req.RoleName == "user" && len(req.AllowedConnections) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "普通用户必须分配至少一个可查询数据库连接",
		})
		return
	}
	if err := auth.ValidateEnabledConnectionNames(req.AllowedConnections); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}
	if req.IsEnabled != 0 {
		req.IsEnabled = 1
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM platform_user WHERE username = ? AND id <> ? AND is_deleted = 0`, req.Username, req.ID).Scan(&exists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "校验用户名失败：" + err.Error(),
		})
		return
	}
	if exists > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "用户名已存在",
		})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "开启事务失败：" + err.Error(),
		})
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if req.Password == "" {
		_, err = tx.Exec(`
UPDATE platform_user
SET username = ?, display_name = ?, role_name = ?, is_enabled = ?, can_query_data = ?, can_query_plan = ?
WHERE id = ?
`, req.Username, req.DisplayName, req.RoleName, req.IsEnabled, req.CanQueryData, req.CanQueryPlan, req.ID)
	} else {
		_, err = tx.Exec(`
UPDATE platform_user
SET username = ?, password_cipher = fixed_aes_encrypt(?), display_name = ?, role_name = ?, is_enabled = ?, can_query_data = ?, can_query_plan = ?
WHERE id = ?
`, req.Username, req.Password, req.DisplayName, req.RoleName, req.IsEnabled, req.CanQueryData, req.CanQueryPlan, req.ID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "更新用户失败：" + err.Error(),
		})
		return
	}

	if req.RoleName == "user" {
		if err := auth.SaveUserAllowedConnectionsTx(tx, req.ID, req.AllowedConnections); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":      false,
				"message": "更新用户连接权限失败：" + err.Error(),
			})
			return
		}
	} else {
		if _, err := tx.Exec(`DELETE FROM platform_user_db_connection WHERE user_id = ?`, req.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":      false,
				"message": "清理管理员连接权限关系失败：" + err.Error(),
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "提交事务失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "编辑用户成功",
	})
}

func adminDeleteUserHandler(c *gin.Context) {
	var req AdminDeleteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}
	if req.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "用户ID不能为空",
		})
		return
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "开启事务失败：" + err.Error(),
		})
		return
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`DELETE FROM platform_user_db_connection WHERE user_id = ?`, req.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "删除用户连接权限失败：" + err.Error(),
		})
		return
	}

	// 删除该用户 SQL 收藏
	if _, err := tx.Exec(`DELETE FROM platform_sql_favorite WHERE user_id = ?`, req.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "删除用户 SQL 收藏失败：" + err.Error(),
		})
		return
	}

	if _, err := tx.Exec(`DELETE FROM platform_session WHERE user_id = ?`, req.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "删除用户会话失败：" + err.Error(),
		})
		return
	}

	res, err := tx.Exec(`UPDATE platform_user SET is_deleted = 1 WHERE id = ?`, req.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "删除用户失败：" + err.Error(),
		})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "用户不存在或已被删除",
		})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "提交事务失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "删除用户成功",
	})
}

// ----------------------------------------------------------------------
// 管理员：查询数据库管理
// ----------------------------------------------------------------------

func adminListConnectionsHandler(c *gin.Context) {
	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	rows, err := db.Query(`
SELECT id, name, label, db_type, host, port, username, database_name, service_name, is_enabled, create_time, update_time
FROM platform_db_connection
ORDER BY id DESC
`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "查询连接配置失败：" + err.Error(),
		})
		return
	}
	defer rows.Close()

	items := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var name, label, dbType, host, username, databaseName, serviceName string
		var port, isEnabled int
		var createTime, updateTime string

		if err := rows.Scan(
			&id,
			&name,
			&label,
			&dbType,
			&host,
			&port,
			&username,
			&databaseName,
			&serviceName,
			&isEnabled,
			&createTime,
			&updateTime,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":      false,
				"message": "读取连接配置失败：" + err.Error(),
			})
			return
		}

		items = append(items, gin.H{
			"id":           id,
			"name":         name,
			"label":        label,
			"dbType":       dbType,
			"host":         host,
			"port":         port,
			"username":     username,
			"databaseName": databaseName,
			"serviceName":  serviceName,
			"isEnabled":    isEnabled,
			"createTime":   createTime,
			"updateTime":   updateTime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"connections": items,
	})
}

func adminCreateConnectionHandler(c *gin.Context) {
	var req AdminCreateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Label = strings.TrimSpace(req.Label)
	req.DBType = strings.ToLower(strings.TrimSpace(req.DBType))
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.DatabaseName = strings.TrimSpace(req.DatabaseName)
	req.ServiceName = strings.TrimSpace(req.ServiceName)

	if req.Name == "" || req.Label == "" || req.DBType == "" || req.Host == "" || req.Port <= 0 || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "连接名称、展示名称、数据库类型、主机、端口、用户名、密码不能为空",
		})
		return
	}

	if req.DBType != "mysql" && req.DBType != "oracle" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "数据库类型只能是 mysql 或 oracle",
		})
		return
	}

	if req.DBType == "mysql" && req.DatabaseName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "MySQL 连接必须填写数据库名",
		})
		return
	}

	if req.DBType == "oracle" && req.ServiceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "Oracle 连接必须填写服务名",
		})
		return
	}

	if req.IsEnabled != 0 {
		req.IsEnabled = 1
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	var exists int
	if err := db.QueryRow(`SELECT COUNT(1) FROM platform_db_connection WHERE name = ?`, req.Name).Scan(&exists); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "校验连接名称失败：" + err.Error(),
		})
		return
	}
	if exists > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "连接名称已存在",
		})
		return
	}

	_, err = db.Exec(`
INSERT INTO platform_db_connection (
    name,
    label,
    db_type,
    host,
    port,
    username,
    password_cipher,
    database_name,
    service_name,
    is_enabled
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    fixed_aes_encrypt(?),
    ?,
    ?,
    ?
)
`, req.Name, req.Label, req.DBType, req.Host, req.Port, req.Username, req.Password, req.DatabaseName, req.ServiceName, req.IsEnabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "创建连接配置失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "创建连接配置成功",
	})
}

func adminUpdateConnectionHandler(c *gin.Context) {
	var req AdminUpdateConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Label = strings.TrimSpace(req.Label)
	req.DBType = strings.ToLower(strings.TrimSpace(req.DBType))
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.DatabaseName = strings.TrimSpace(req.DatabaseName)
	req.ServiceName = strings.TrimSpace(req.ServiceName)

	if req.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "连接ID不能为空",
		})
		return
	}
	if req.Name == "" || req.Label == "" || req.DBType == "" || req.Host == "" || req.Port <= 0 || req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "连接名称、展示名称、数据库类型、主机、端口、用户名不能为空",
		})
		return
	}
	if req.DBType != "mysql" && req.DBType != "oracle" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "数据库类型只能是 mysql 或 oracle",
		})
		return
	}
	if req.DBType == "mysql" && req.DatabaseName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "MySQL 连接必须填写数据库名",
		})
		return
	}
	if req.DBType == "oracle" && req.ServiceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "Oracle 连接必须填写服务名",
		})
		return
	}
	if req.IsEnabled != 0 {
		req.IsEnabled = 1
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	var currentName string
	if err := db.QueryRow(`SELECT name FROM platform_db_connection WHERE id = ?`, req.ID).Scan(&currentName); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":      false,
				"message": "连接不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "读取连接失败：" + err.Error(),
		})
		return
	}
	if currentName != req.Name {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "当前版本不支持修改连接名称",
		})
		return
	}

	if req.Password == "" {
		_, err = db.Exec(`
UPDATE platform_db_connection
SET label = ?, db_type = ?, host = ?, port = ?, username = ?, database_name = ?, service_name = ?, is_enabled = ?
WHERE id = ?
`, req.Label, req.DBType, req.Host, req.Port, req.Username, req.DatabaseName, req.ServiceName, req.IsEnabled, req.ID)
	} else {
		_, err = db.Exec(`
UPDATE platform_db_connection
SET label = ?, db_type = ?, host = ?, port = ?, username = ?, password_cipher = fixed_aes_encrypt(?), database_name = ?, service_name = ?, is_enabled = ?
WHERE id = ?
`, req.Label, req.DBType, req.Host, req.Port, req.Username, req.Password, req.DatabaseName, req.ServiceName, req.IsEnabled, req.ID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "编辑连接配置失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "编辑连接配置成功",
	})
}

func adminDeleteConnectionHandler(c *gin.Context) {
	var req AdminDeleteConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}
	if req.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "连接ID不能为空",
		})
		return
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	var name string
	if err := db.QueryRow(`SELECT name FROM platform_db_connection WHERE id = ?`, req.ID).Scan(&name); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":      false,
				"message": "连接不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "读取连接失败：" + err.Error(),
		})
		return
	}

	var refCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM platform_user_db_connection WHERE connection_name = ?`, name).Scan(&refCount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "校验连接引用关系失败：" + err.Error(),
		})
		return
	}
	if refCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "该连接仍被用户权限引用，无法删除。请先取消用户分配后再删除",
		})
		return
	}

	// SQL 收藏里如果仍有该 connection_name，不阻止删除，但会把它清空，避免脏引用
	if _, err := db.Exec(`
UPDATE platform_sql_favorite
SET connection_name = ''
WHERE connection_name = ?
`, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "清理 SQL 收藏关联连接失败：" + err.Error(),
		})
		return
	}

	res, err := db.Exec(`DELETE FROM platform_db_connection WHERE id = ?`, req.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "删除连接配置失败：" + err.Error(),
		})
		return
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "连接不存在或已被删除",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "删除连接配置成功",
	})
}

func adminTestConnectionHandler(c *gin.Context) {
	var req AdminTestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "请求格式错误：" + err.Error(),
		})
		return
	}

	req.DBType = strings.ToLower(strings.TrimSpace(req.DBType))
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.DatabaseName = strings.TrimSpace(req.DatabaseName)
	req.ServiceName = strings.TrimSpace(req.ServiceName)

	if req.Host == "" || req.Port <= 0 || req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":      false,
			"message": "主机、端口、用户名不能为空",
		})
		return
	}

	password := req.Password
	if password == "" && req.ID > 0 {
		db, err := config.GetPlatformDB()
		if err == nil {
			var plain string
			err = db.QueryRow("SELECT fixed_aes_decrypt(password_cipher) FROM platform_db_connection WHERE id = ?", req.ID).Scan(&plain)
			if err == nil {
				password = plain
			} else if err != sql.ErrNoRows {
				c.JSON(http.StatusInternalServerError, gin.H{
					"ok":      false,
					"message": "获取密码失败：" + err.Error(),
				})
				return
			}
		}
	}

	var dsn string
	switch req.DBType {
	case "mysql":
		if req.DatabaseName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "MySQL 连接必须填写数据库名"})
			return
		}
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			req.Username, password, req.Host, req.Port, req.DatabaseName)
	case "oracle":
		if req.ServiceName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "Oracle 连接必须填写服务名"})
			return
		}
		dsn = fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
			req.Username, password, req.Host, req.Port, req.ServiceName)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "不支持的数据库类型：" + req.DBType})
		return
	}

	testDB, err := sql.Open(req.DBType, dsn)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"message": "初始化连接失败：" + err.Error(),
		})
		return
	}
	defer testDB.Close()

	testDB.SetConnMaxLifetime(5 * time.Second)

	err = testDB.Ping()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"message": "连接失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "连接测试成功",
	})
}

// listAuditHistoryHandler
// ----------------------------------------------------------------------
// 获取当前用户的 SQL 审核历史记录
func listAuditHistoryHandler(c *gin.Context) {
	userID, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(401, gin.H{"error": "获取当前用户失败"})
		return
	}

	sqlText := c.Query("sql")
	var digest string
	if sqlText != "" {
		digest = auth.GenerateSQLDigest(sqlText)
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if pageSize < 1 {
		pageSize = 10
	}

	total, records, err := auth.GetSqlAuditHistoryByUserID(userID, digest, page, pageSize)
	if err != nil {
		log.Printf("[listAuditHistoryHandler] error: %v", err)
		c.JSON(500, gin.H{"error": "获取审核历史失败"})
		return
	}

	if records == nil {
		records = []auth.SqlAuditHistoryRecord{}
	}

	c.JSON(200, gin.H{
		"total":   total,
		"history": records,
	})
}

// 请求体: { "auditId": 123, "submitAudit": 1, "remark": "..." }
func submitAuditHandler(c *gin.Context) {
	userIDVal, exists := c.Get("currentUserID")
	if !exists {
		c.JSON(401, gin.H{"error": "未登录"})
		return
	}
	userID := userIDVal.(int64)

	var req struct {
		AuditID     int64  `json:"auditId"`
		SubmitAudit int    `json:"submitAudit"`
		Remark      string `json:"remark"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}

	if req.AuditID <= 0 {
		c.JSON(400, gin.H{"error": "非法的 auditId"})
		return
	}

	if req.SubmitAudit < 1 || req.SubmitAudit > 5 {
		c.JSON(400, gin.H{"error": "非法的提交审核类型"})
		return
	}

	if req.SubmitAudit == 5 && strings.TrimSpace(req.Remark) == "" {
		c.JSON(400, gin.H{"error": "选择“其他”时备注不能为空"})
		return
	}

	if err := auth.SubmitSqlAudit(req.AuditID, userID, req.SubmitAudit, req.Remark); err != nil {
		log.Printf("[submitAuditHandler] error: %v", err)
		c.JSON(500, gin.H{"error": "提交审核失败"})
		return
	}

	c.JSON(200, gin.H{
		"message": "提交成功",
	})
}

// adminListSubmittedAuditsHandler
// ----------------------------------------------------------------------
func adminListSubmittedAuditsHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if pageSize < 1 {
		pageSize = 10
	}

	status := c.Query("status")
	submitter := c.Query("submitter")
	connection := c.Query("connection")

	total, records, err := auth.AdminListSubmittedAudits(page, pageSize, status, submitter, connection)
	if err != nil {
		log.Printf("[adminListSubmittedAuditsHandler] error: %v", err)
		c.JSON(500, gin.H{"error": "获取提交审核记录失败"})
		return
	}

	if records == nil {
		records = []auth.AdminSqlAuditRecord{}
	}

	c.JSON(200, gin.H{
		"total":   total,
		"history": records,
	})
}

// adminReviewAuditHandler
// ----------------------------------------------------------------------
func adminReviewAuditHandler(c *gin.Context) {
	var req struct {
		AuditID     int64 `json:"auditId"`
		AuditPassed int   `json:"auditPassed"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}

	if req.AuditID <= 0 {
		c.JSON(400, gin.H{"error": "非法的 auditId"})
		return
	}

	if req.AuditPassed != 1 && req.AuditPassed != -1 {
		c.JSON(400, gin.H{"error": "非法的审核状态"})
		return
	}

	reviewer := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		reviewer = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		reviewer = username.(string)
	}

	if err := auth.AdminReviewAudit(req.AuditID, req.AuditPassed, reviewer); err != nil {
		log.Printf("[adminReviewAuditHandler] error: %v", err)
		c.JSON(500, gin.H{"error": "审核操作失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message": "审核操作成功",
	})
}

// ----------------------------------------------------------------------
// 数据库变更申请接口
// ----------------------------------------------------------------------

func listDBChangeRequestsHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	applicant := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		applicant = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		applicant = username.(string)
	}

	applicantTeam := c.Query("applicantTeam")
	urgencyLevel := c.Query("urgencyLevel")
	dbType := c.Query("dbType")

	total, records, err := appsql.QueryDBChangeRequests(page, pageSize, applicant, applicantTeam, urgencyLevel, dbType)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "total": 0, "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.DBChangeRequestRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"total":   total,
		"records": records,
	})
}

func listAllTeamDbEnvsForUserHandler(c *gin.Context) {
	records, err := appsql.ListAllTeamDbEnvs()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.TeamDbEnvRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"records": records,
	})
}

func createDBChangeRequestHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	applicant := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		applicant = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		applicant = username.(string)
	}

	var req DBChangeRequestCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	err := appsql.CreateDBChangeRequest(appsql.DBChangeRequestRecord{
		Applicant:         applicant,
		ApplicantTeam:     req.ApplicantTeam,
		PlannedChangeTime: req.PlannedChangeTime,
		UrgencyLevel:      req.UrgencyLevel,
		ChangeType:        req.ChangeType,
		ChangeReason:      req.ChangeReason,
		RequirementUrl:    req.RequirementUrl,
		ImpactScope:       req.ImpactScope,
		DbType:            req.DbType,
		TestDbIp:          req.TestDbIp,
		TestDbName:        req.TestDbName,
		TestDbSchema:      req.TestDbSchema,
		DbIp:              req.DbIp,
		DbName:            req.DbName,
		DbSchema:          req.DbSchema,
		ChangeContent:     req.ChangeContent,
	})

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "申请提交成功"})
}

func updateDBChangeRequestHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	applicant := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		applicant = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		applicant = username.(string)
	}

	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}

	var req DBChangeRequestUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	err := appsql.UpdateDBChangeRequest(appsql.DBChangeRequestRecord{
		ID:                req.ID,
		Applicant:         applicant,
		ApplicantTeam:     req.ApplicantTeam,
		Environment:       req.Environment,
		PlannedChangeTime: req.PlannedChangeTime,
		UrgencyLevel:      req.UrgencyLevel,
		ChangeType:        req.ChangeType,
		ChangeReason:      req.ChangeReason,
		RequirementUrl:    req.RequirementUrl,
		ImpactScope:       req.ImpactScope,
		DbType:            req.DbType,
		TestDbIp:          req.TestDbIp,
		TestDbName:        req.TestDbName,
		TestDbSchema:      req.TestDbSchema,
		DbIp:              req.DbIp,
		DbName:            req.DbName,
		DbSchema:          req.DbSchema,
		ChangeContent:     req.ChangeContent,
	}, roleName)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "申请更新成功"})
}

func deleteDBChangeRequestHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	applicant := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		applicant = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		applicant = username.(string)
	}

	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}

	var req DBChangeRequestDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.DeleteDBChangeRequest(req.ID, applicant, roleName); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "申请删除成功"})
}

func adminReleaseDBChangeRequestHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	var req DBChangeRequestReleaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	err := appsql.UpdateDBChangeRequestRelease(req.ID, req.TestPublisher, req.ProdPublisher, req.ReleaseVerifier)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "发布验证信息更新成功"})
}

func adminListReleaseDBChangeRequestsHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	applicantTeam := c.Query("applicantTeam")
	urgencyLevel := c.Query("urgencyLevel")
	dbType := c.Query("dbType")
	isVerified := c.Query("isVerified")

	total, records, err := appsql.QueryDBChangeRequestsForRelease(page, pageSize, applicantTeam, urgencyLevel, dbType, isVerified)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "total": 0, "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.DBChangeRequestRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"total":   total,
		"records": records,
	})
}

// ----------------------------------------------------------------------
// 团队数据库环境配置接口
// ----------------------------------------------------------------------

func adminListTeamDbEnvsHandler(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	teamName := c.Query("teamName")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	total, records, err := appsql.ListTeamDbEnvs(page, pageSize, teamName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "total": 0, "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.TeamDbEnvRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"total":   total,
		"records": records,
	})
}

func adminListAllTeamDbEnvsHandler(c *gin.Context) {
	records, err := appsql.ListAllTeamDbEnvs()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.TeamDbEnvRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"records": records,
	})
}

func adminCreateTeamDbEnvHandler(c *gin.Context) {
	var req appsql.TeamDbEnvRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.CreateTeamDbEnv(req); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "创建失败：" + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "创建成功"})
}

func adminUpdateTeamDbEnvHandler(c *gin.Context) {
	var req appsql.TeamDbEnvRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.UpdateTeamDbEnv(req); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新失败：" + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "更新成功"})
}

func adminDeleteTeamDbEnvHandler(c *gin.Context) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.DeleteTeamDbEnv(req.ID); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "删除失败：" + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "删除成功"})
}

// ----------------------------------------------------------------------
// 数据库数据同步申请接口
// ----------------------------------------------------------------------

func listDBDataSyncRequestsHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	applicant := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		applicant = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		applicant = username.(string)
	}

	urgencyLevel := c.Query("urgencyLevel")
	operateType, _ := strconv.Atoi(c.Query("operateType"))

	total, records, err := appsql.QueryDBDataSyncRequests(page, pageSize, applicant, urgencyLevel, operateType)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "total": 0, "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.DBDataSyncRequestRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"total":   total,
		"records": records,
	})
}

func createDBDataSyncRequestHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	applicant := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		applicant = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		applicant = username.(string)
	}

	var req DBDataSyncRequestCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	err := appsql.CreateDBDataSyncRequest(appsql.DBDataSyncRequestRecord{
		Applicant:             applicant,
		ApplicantTeam:         req.ApplicantTeam,
		Environment:           req.Environment,
		ExpectedFinishTime:    req.ExpectedFinishTime,
		UrgencyLevel:          req.UrgencyLevel,
		UrgencyReason:         req.UrgencyReason,
		ApplyReason:           req.ApplyReason,
		OperateType:           req.OperateType,
		SourceDb:              req.SourceDb,
		TargetDbOrPerson:      req.TargetDbOrPerson,
		InvolvedDbSchemaTable: req.InvolvedDbSchemaTable,
		DataFilterCondition:   req.DataFilterCondition,
		EstimatedDataVolume:   req.EstimatedDataVolume,
		ContainsSensitiveData: req.ContainsSensitiveData,
		DesensitizationRule:   req.DesensitizationRule,
	})

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "申请提交成功"})
}

func updateDBDataSyncRequestHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	applicant := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		applicant = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		applicant = username.(string)
	}

	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}

	var req DBDataSyncRequestUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	err := appsql.UpdateDBDataSyncRequest(appsql.DBDataSyncRequestRecord{
		ID:                    req.ID,
		Applicant:             applicant,
		ApplicantTeam:         req.ApplicantTeam,
		Environment:           req.Environment,
		ExpectedFinishTime:    req.ExpectedFinishTime,
		UrgencyLevel:          req.UrgencyLevel,
		UrgencyReason:         req.UrgencyReason,
		ApplyReason:           req.ApplyReason,
		OperateType:           req.OperateType,
		SourceDb:              req.SourceDb,
		TargetDbOrPerson:      req.TargetDbOrPerson,
		InvolvedDbSchemaTable: req.InvolvedDbSchemaTable,
		DataFilterCondition:   req.DataFilterCondition,
		EstimatedDataVolume:   req.EstimatedDataVolume,
		ContainsSensitiveData: req.ContainsSensitiveData,
		DesensitizationRule:   req.DesensitizationRule,
	}, roleName)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "申请更新成功"})
}

func deleteDBDataSyncRequestHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	applicant := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		applicant = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		applicant = username.(string)
	}

	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}

	var req DBDataSyncRequestDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.DeleteDBDataSyncRequest(req.ID, applicant, roleName); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "申请删除成功"})
}

func adminListDBDataSyncRequestsForDbaHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	applicant := c.Query("applicant")
	urgencyLevel := c.Query("urgencyLevel")
	operateType, _ := strconv.Atoi(c.Query("operateType"))

	total, records, err := appsql.QueryDBDataSyncRequests(page, pageSize, applicant, urgencyLevel, operateType)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "total": 0, "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.DBDataSyncRequestRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"total":   total,
		"records": records,
	})
}

func adminUpdateDBDataSyncRequestDbaHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	var req DBDataSyncRequestDbaReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.UpdateDBDataSyncRequestDBA(req.ID, req.ExecuteDba); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "执行DBA信息更新成功"})
}
