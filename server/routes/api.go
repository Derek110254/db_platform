package routes

import (
	"database/sql"
	"fmt"
	"log"
	"net"
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

type ManualQueryPlanRequest struct {
	ConnectionName string `json:"connectionName"`
	SQL            string `json:"sql"`
	ExecutionPlan  string `json:"executionPlan"`
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
	DBType       string `json:"dbType"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DatabaseName string `json:"databaseName"`
	ServiceName  string `json:"serviceName"`
	IsEnabled    int    `json:"isEnabled"`
	CanConnect   int    `json:"canConnect"`
}

type AdminUpdateConnectionRequest struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	DBType       string `json:"dbType"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DatabaseName string `json:"databaseName"`
	ServiceName  string `json:"serviceName"`
	IsEnabled    int    `json:"isEnabled"`
	CanConnect   int    `json:"canConnect"`
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
	BackupTable       string `json:"backupTable"`
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
	BackupTable       string `json:"backupTable"`
}

// DBChangeRequestReleaseReq
// ----------------------------------------------------------------------
// 数据库变更申请发布验证请求
type DBChangeRequestReleaseReq struct {
	ID            int64  `json:"id" binding:"required"`
	TestPublisher string `json:"testPublisher"`
	ProdPublisher string `json:"prodPublisher"`
}

// DBChangeRequestDeleteReq
// ----------------------------------------------------------------------
// 删除数据库变更申请请求
type DBChangeRequestDeleteReq struct {
	ID int64 `json:"id" binding:"required"`
}

// DBChangeRequestVerifyReq
// ----------------------------------------------------------------------
// 申请人验证发布结果请求
type DBChangeRequestVerifyReq struct {
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

// DBAlertHandleCreateReq
// ----------------------------------------------------------------------
// 新增数据库告警处理记录请求
type DBAlertHandleCreateReq struct {
	DBType          string `json:"dbType" binding:"required"`
	AlertLevel      string `json:"alertLevel" binding:"required"`
	AlertCategory   string `json:"alertCategory" binding:"required"`
	AlertContent    string `json:"alertContent" binding:"required"`
	ImpactScope     string `json:"impactScope"`
	AlertTime       string `json:"alertTime" binding:"required"`
	HandleStartTime string `json:"handleStartTime"`
	HandleEndTime   string `json:"handleEndTime"`
	HandleResult    string `json:"handleResult"`
}

// DBAlertHandleUpdateReq
// ----------------------------------------------------------------------
// 编辑数据库告警处理记录请求
type DBAlertHandleUpdateReq struct {
	ID              int64  `json:"id" binding:"required"`
	DBType          string `json:"dbType" binding:"required"`
	AlertLevel      string `json:"alertLevel" binding:"required"`
	AlertCategory   string `json:"alertCategory" binding:"required"`
	AlertContent    string `json:"alertContent" binding:"required"`
	ImpactScope     string `json:"impactScope"`
	AlertTime       string `json:"alertTime" binding:"required"`
	HandleStartTime string `json:"handleStartTime"`
	HandleEndTime   string `json:"handleEndTime"`
	HandleResult    string `json:"handleResult"`
}

// DBAlertHandleDeleteReq
// ----------------------------------------------------------------------
// 删除数据库告警处理记录请求
type DBAlertHandleDeleteReq struct {
	ID int64 `json:"id" binding:"required"`
}

// DBAlertHandleResultReq
// ----------------------------------------------------------------------
// 管理员更新告警处理结果请求
type DBAlertHandleResultReq struct {
	ID              int64  `json:"id" binding:"required"`
	Handler         string `json:"handler" binding:"required"`
	HandleStartTime string `json:"handleStartTime"`
	HandleEndTime   string `json:"handleEndTime"`
	HandleResult    string `json:"handleResult" binding:"required"`
}

// OpsChangeCreateReq
// ----------------------------------------------------------------------
// 新增运维变更记录请求
type OpsChangeCreateReq struct {
	ChangeTitle   string `json:"changeTitle" binding:"required"`
	ChangeType    string `json:"changeType" binding:"required"`
	ChangeLevel   string `json:"changeLevel" binding:"required"`
	ChangeContent string `json:"changeContent" binding:"required"`
	ImpactScope   string `json:"impactScope"`
	ChangeIPList  string `json:"changeIpList"`
	ChangeTime    string `json:"changeTime" binding:"required"`
	RollbackPlan  string `json:"rollbackPlan"`
	Remark        string `json:"remark"`
}

// OpsChangeUpdateReq
// ----------------------------------------------------------------------
// 编辑运维变更记录请求
type OpsChangeUpdateReq struct {
	ID            int64  `json:"id" binding:"required"`
	ChangeTitle   string `json:"changeTitle" binding:"required"`
	ChangeType    string `json:"changeType" binding:"required"`
	ChangeLevel   string `json:"changeLevel" binding:"required"`
	ChangeContent string `json:"changeContent" binding:"required"`
	ImpactScope   string `json:"impactScope"`
	ChangeIPList  string `json:"changeIpList"`
	ChangeTime    string `json:"changeTime" binding:"required"`
	RollbackPlan  string `json:"rollbackPlan"`
	Remark        string `json:"remark"`
}

// OpsChangeDeleteReq
// ----------------------------------------------------------------------
// 删除运维变更记录请求
type OpsChangeDeleteReq struct {
	ID int64 `json:"id" binding:"required"`
}

// OpsChangeResultReq
// ----------------------------------------------------------------------
// 管理员确认变更结果请求
type OpsChangeResultReq struct {
	ID           int64  `json:"id" binding:"required"`
	ChangeResult string `json:"changeResult" binding:"required"`
}

// OpsChangeRollbackReq
// ----------------------------------------------------------------------
// 管理员确认回滚状态请求
type OpsChangeRollbackReq struct {
	ID             int64  `json:"id" binding:"required"`
	RollbackStatus string `json:"rollbackStatus" binding:"required"`
}

// OpsChangeReviewerReq
// ----------------------------------------------------------------------
// 管理员更新复核人请求
type OpsChangeReviewerReq struct {
	ID int64 `json:"id" binding:"required"`
}

// RegisterAPIRoutes
// ----------------------------------------------------------------------
// 注册所有 API 路由
func RegisterAPIRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		// 基础测试接口
		api.GET("/hello", helloHandler)
		api.GET("/dashboard", dashboardHandler) // 首屏看板（白名单）

		// 语法检测接口（不需要登录）
		api.POST("/check-sql", checkSQLHandler) // 检测查询 SQL 的语法及风险
		api.POST("/check-ddl", checkDDLHandler) // 检测 DDL 语句的语法风险

		// 认证相关接口
		api.POST("/login", loginHandler)         // 账密登录
		api.POST("/sso-login", ssoLoginHandler)  // SSO 登录
		api.GET("/sso-config", ssoConfigHandler) // 获取 SSO 配置（白名单，无需登录）
		api.POST("/logout", logoutHandler)       // 注销登录
		api.GET("/auth/me", authMeHandler)       // 获取当前登录用户信息

		queryGroup := api.Group("/")
		queryGroup.Use(middleware.RequireLogin())
		{
			// 看板（需登录）
			queryGroup.GET("/dashboard/yearly", yearlyDashboardHandler)     // 今年工作量统计
			queryGroup.GET("/dashboard/personal", personalDashboardHandler) // 个人看板

			// 个人基础操作
			queryGroup.POST("/user/change-password", changePasswordHandler) // 用户修改个人密码

			// 核心查询操作
			queryGroup.GET("/query-connections", queryConnectionsHandler)   // 获取当前用户有权访问的数据库连接列表
			queryGroup.POST("/query-data", queryDataHandler)                // 执行查询 SQL 获取数据结果
			queryGroup.POST("/query-plan", queryPlanHandler)                // 获取 SQL 的执行计划 (Explain)
			queryGroup.POST("/query-plan/manual", manualQueryPlanHandler)   // 手动提交执行计划进行性能检测
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
			queryGroup.GET("/db-change-requests", listDBChangeRequestsHandler)         // 获取个人提交的数据库变更申请列表
			queryGroup.POST("/db-change-requests", createDBChangeRequestHandler)       // 提交数据库变更申请
			queryGroup.PUT("/db-change-requests", updateDBChangeRequestHandler)        // 修改自己提交的变更申请
			queryGroup.DELETE("/db-change-requests", deleteDBChangeRequestHandler)     // 删除自己提交的变更申请
			queryGroup.PUT("/db-change-requests/verify", verifyDBChangeRequestHandler) // 申请人验证发布结果

			// 数据库数据同步申请接口
			queryGroup.GET("/db-data-sync-requests", listDBDataSyncRequestsHandler)     // 获取个人提交的数据同步申请列表
			queryGroup.POST("/db-data-sync-requests", createDBDataSyncRequestHandler)   // 提交数据同步申请
			queryGroup.PUT("/db-data-sync-requests", updateDBDataSyncRequestHandler)    // 修改自己提交的数据同步申请
			queryGroup.DELETE("/db-data-sync-requests", deleteDBDataSyncRequestHandler) // 删除自己提交的数据同步申请

			// 团队数据库环境接口（允许普通用户读取）
			queryGroup.GET("/team-db-envs", listAllTeamDbEnvsForUserHandler) // 用户查看所有团队数据库环境信息

			// 数据库告警处理接口
			queryGroup.GET("/db-alert-handles", listDBAlertHandlesHandler)     // 获取个人提交的告警处理记录列表
			queryGroup.POST("/db-alert-handles", createDBAlertHandleHandler)   // 新增告警处理记录
			queryGroup.PUT("/db-alert-handles", updateDBAlertHandleHandler)    // 修改自己的告警处理记录
			queryGroup.DELETE("/db-alert-handles", deleteDBAlertHandleHandler) // 删除自己的告警处理记录

			// 运维变更记录接口
			queryGroup.GET("/ops-change-records", listOpsChangeRecordsHandler)     // 获取个人提交的运维变更记录列表
			queryGroup.POST("/ops-change-records", createOpsChangeRecordHandler)   // 新增运维变更记录
			queryGroup.PUT("/ops-change-records", updateOpsChangeRecordHandler)    // 修改自己的运维变更记录
			queryGroup.DELETE("/ops-change-records", deleteOpsChangeRecordHandler) // 删除自己的运维变更记录
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

			// 数据库告警处理管理（管理员功能）
			adminGroup.GET("/db-alert-handles", adminListDBAlertHandlesHandler)               // 获取所有告警处理记录
			adminGroup.PUT("/db-alert-handles/result", adminUpdateDBAlertHandleResultHandler) // 更新告警处理结果

			// 运维变更记录管理（管理员功能）
			adminGroup.GET("/ops-change-records", adminListOpsChangeRecordsHandler)             // 获取所有运维变更记录
			adminGroup.PUT("/ops-change-records/result", adminUpdateOpsChangeResultHandler)     // 确认变更结果
			adminGroup.PUT("/ops-change-records/rollback", adminUpdateOpsChangeRollbackHandler) // 确认回滚状态
			adminGroup.PUT("/ops-change-records/reviewer", adminUpdateOpsChangeReviewerHandler) // 更新复核人
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

func dashboardHandler(c *gin.Context) {
	result := appsql.GetDashboardOverview()
	c.JSON(http.StatusOK, result)
}

func yearlyDashboardHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}
	result := appsql.GetYearlyDashboard()
	c.JSON(http.StatusOK, result)
}

func personalDashboardHandler(c *gin.Context) {
	userID, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	// 获取当前用户名（优先用 displayName）
	username := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		username = displayName.(string)
	} else if u, ok := c.Get("currentUsername"); ok && u.(string) != "" {
		username = u.(string)
	}

	result := appsql.GetPersonalDashboard(userID, username)
	c.JSON(http.StatusOK, result)
}

func ssoConfigHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"loginUrl": config.SSOLoginUrl,
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
	Token string `json:"token" binding:"required"`
	UrlId string `json:"urlId"`
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

	// 修复：浏览器可能将 + 转为空格，还原回来
	rawToken := strings.ReplaceAll(req.Token, " ", "+")

	// urlId 默认用后端配置值
	urlId := strings.TrimSpace(req.UrlId)
	if urlId == "" {
		urlId = config.SSOUrlId
	}

	// 后端完成 RSA 加密 + SSO API 调用 + 用户名解析
	ssoResult, err := auth.VerifySSOToken(rawToken, urlId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"message": "SSO验证异常：" + err.Error(),
		})
		return
	}
	if !ssoResult.Success {
		c.JSON(http.StatusOK, gin.H{
			"ok":      false,
			"message": ssoResult.Message,
		})
		return
	}

	// 使用 username 查 platform_user 获取权限并生成 session
	user, token, err := auth.LoginByUsername(ssoResult.Username)
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
			"dbType":      item.DBType,
			"host":        item.Host,
			"port":        item.Port,
			"database":    item.DatabaseName,
			"serviceName": item.ServiceName,
			"canConnect":  item.CanConnect,
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

func manualQueryPlanHandler(c *gin.Context) {
	var req ManualQueryPlanRequest
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

	result := appsql.ManualExplainQuery(
		userID,
		roleName,
		req.ConnectionName,
		req.SQL,
		req.ExecutionPlan,
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
SELECT id, name, db_type, host, port, username, database_name, service_name, is_enabled, can_connect, create_time, update_time
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
		var name, dbType, host, username, databaseName, serviceName string
		var port, isEnabled, canConnect int
		var createTime, updateTime string

		if err := rows.Scan(
			&id,
			&name,
			&dbType,
			&host,
			&port,
			&username,
			&databaseName,
			&serviceName,
			&isEnabled,
			&canConnect,
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
			"dbType":       dbType,
			"host":         host,
			"port":         port,
			"username":     username,
			"databaseName": databaseName,
			"serviceName":  serviceName,
			"isEnabled":    isEnabled,
			"canConnect":   canConnect,
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
	req.DBType = strings.ToLower(strings.TrimSpace(req.DBType))
	req.Host = strings.TrimSpace(req.Host)
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.DatabaseName = strings.TrimSpace(req.DatabaseName)
	req.ServiceName = strings.TrimSpace(req.ServiceName)

	if req.Name == "" || req.DBType == "" || req.Host == "" || req.Port <= 0 || req.Username == "" || req.Password == "" {
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
`, req.Name, req.DBType, req.Host, req.Port, req.Username, req.Password, req.DatabaseName, req.ServiceName, req.IsEnabled, req.CanConnect)
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
	if req.Name == "" || req.DBType == "" || req.Host == "" || req.Port <= 0 || req.Username == "" {
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
	if req.CanConnect != 0 {
		req.CanConnect = 1
	}

	db, err := config.GetPlatformDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "连接平台库失败：" + err.Error(),
		})
		return
	}

	// 读取当前连接名称，用于判断是否需要级联更新
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

	// 如果修改了连接名称，检查新名称是否已被占用
	nameChanged := currentName != req.Name
	if nameChanged {
		var count int
		if err := db.QueryRow(`SELECT COUNT(1) FROM platform_db_connection WHERE name = ? AND id != ?`, req.Name, req.ID).Scan(&count); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":      false,
				"message": "检查连接名称失败：" + err.Error(),
			})
			return
		}
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"ok":      false,
				"message": "连接名称已存在",
			})
			return
		}
	}

	// 使用事务，确保连接名称修改和级联更新原子性
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
UPDATE platform_db_connection
SET name = ?, db_type = ?, host = ?, port = ?, username = ?, database_name = ?, service_name = ?, is_enabled = ?, can_connect = ?
WHERE id = ?
`, req.Name, req.DBType, req.Host, req.Port, req.Username, req.DatabaseName, req.ServiceName, req.IsEnabled, req.CanConnect, req.ID)
	} else {
		_, err = tx.Exec(`
UPDATE platform_db_connection
SET name = ?, db_type = ?, host = ?, port = ?, username = ?, password_cipher = fixed_aes_encrypt(?), database_name = ?, service_name = ?, is_enabled = ?, can_connect = ?
WHERE id = ?
`, req.Name, req.DBType, req.Host, req.Port, req.Username, req.Password, req.DatabaseName, req.ServiceName, req.IsEnabled, req.CanConnect, req.ID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"ok":      false,
			"message": "编辑连接配置失败：" + err.Error(),
		})
		return
	}

	// 如果连接名称变更，级联更新所有引用 connection_name 的关联表
	if nameChanged {
		tables := []string{
			"platform_user_db_connection",
			"platform_sql_favorite",
			"platform_sql_audit",
		}
		for _, table := range tables {
			_, err = tx.Exec(fmt.Sprintf(`UPDATE %s SET connection_name = ? WHERE connection_name = ?`, table), req.Name, currentName)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"ok":      false,
					"message": fmt.Sprintf("更新关联表 %s 失败：%s", table, err.Error()),
				})
				return
			}
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
			"ok":        false,
			"errorType": "port",
			"message":   "主机、端口、用户名不能为空",
		})
		return
	}

	// 第一步：测试端口连通性（3 秒超时）
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", req.Host, req.Port), 3*time.Second)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok":        false,
			"errorType": "port",
			"message":   "端口连接失败：" + err.Error() + "（主机/端口无法连通）",
		})
		return
	}
	_ = conn.Close()

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
			"ok":        false,
			"errorType": "auth",
			"message":   "连接失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"errorType": "",
		"message":   "连接测试成功",
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
		BackupTable:       req.BackupTable,
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
		BackupTable:       req.BackupTable,
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

func verifyDBChangeRequestHandler(c *gin.Context) {
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

	var req DBChangeRequestVerifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.VerifyDBChangeRequest(req.ID, applicant, roleName); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "验证失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "验证成功，该变更申请已完结"})
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

	err := appsql.UpdateDBChangeRequestRelease(req.ID, req.TestPublisher, req.ProdPublisher)
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

// ----------------------------------------------------------------------
// 数据库告警处理接口
// ----------------------------------------------------------------------

func listDBAlertHandlesHandler(c *gin.Context) {
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

	// 普通用户只能看到 handler 为自己的记录，admin 看全部
	handler := ""
	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}
	if roleName != "admin" {
		if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
			handler = displayName.(string)
		} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
			handler = username.(string)
		}
	}

	dbType := c.Query("dbType")
	alertLevel := c.Query("alertLevel")
	alertCategory := c.Query("alertCategory")

	total, records, err := appsql.QueryDBAlertHandles(page, pageSize, handler, dbType, alertLevel, alertCategory)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "total": 0, "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.DBAlertHandleRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"total":   total,
		"records": records,
	})
}

func createDBAlertHandleHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	// 处理人默认为当前创建人
	handler := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		handler = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		handler = username.(string)
	}

	var req DBAlertHandleCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	err := appsql.CreateDBAlertHandle(appsql.DBAlertHandleRecord{
		DBType:          req.DBType,
		AlertLevel:      req.AlertLevel,
		AlertCategory:   req.AlertCategory,
		AlertContent:    req.AlertContent,
		ImpactScope:     req.ImpactScope,
		AlertTime:       req.AlertTime,
		Handler:         handler,
		HandleStartTime: req.HandleStartTime,
		HandleEndTime:   req.HandleEndTime,
		HandleResult:    req.HandleResult,
	})

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "告警处理记录创建成功"})
}

func updateDBAlertHandleHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}

	// 用于归属校验（非 admin 只能改自己的记录）
	handler := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		handler = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		handler = username.(string)
	}

	var req DBAlertHandleUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	err := appsql.UpdateDBAlertHandle(appsql.DBAlertHandleRecord{
		ID:              req.ID,
		DBType:          req.DBType,
		AlertLevel:      req.AlertLevel,
		AlertCategory:   req.AlertCategory,
		AlertContent:    req.AlertContent,
		ImpactScope:     req.ImpactScope,
		AlertTime:       req.AlertTime,
		Handler:         handler,
		HandleStartTime: req.HandleStartTime,
		HandleEndTime:   req.HandleEndTime,
		HandleResult:    req.HandleResult,
	}, roleName)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "告警处理记录更新成功"})
}

func deleteDBAlertHandleHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	handler := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		handler = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		handler = username.(string)
	}

	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}

	var req DBAlertHandleDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.DeleteDBAlertHandle(req.ID, handler, roleName); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "告警处理记录删除成功"})
}

func adminListDBAlertHandlesHandler(c *gin.Context) {
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

	handler := c.Query("handler")
	dbType := c.Query("dbType")
	alertLevel := c.Query("alertLevel")
	alertCategory := c.Query("alertCategory")

	total, records, err := appsql.QueryDBAlertHandles(page, pageSize, handler, dbType, alertLevel, alertCategory)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "total": 0, "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.DBAlertHandleRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"total":   total,
		"records": records,
	})
}

func adminUpdateDBAlertHandleResultHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	var req DBAlertHandleResultReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.UpdateDBAlertHandleResult(req.ID, req.Handler, req.HandleStartTime, req.HandleEndTime, req.HandleResult); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "告警处理结果更新成功"})
}

// ----------------------------------------------------------------------
// 运维变更记录接口
// ----------------------------------------------------------------------

func listOpsChangeRecordsHandler(c *gin.Context) {
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

	operator := ""
	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}
	if roleName != "admin" {
		if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
			operator = displayName.(string)
		} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
			operator = username.(string)
		}
	}

	changeType := c.Query("changeType")
	changeLevel := c.Query("changeLevel")

	total, records, err := appsql.QueryOpsChangeRecords(page, pageSize, operator, changeType, changeLevel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "total": 0, "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.OpsChangeRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"total":   total,
		"records": records,
	})
}

func createOpsChangeRecordHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	operator := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		operator = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		operator = username.(string)
	}

	var req OpsChangeCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	err := appsql.CreateOpsChangeRecord(appsql.OpsChangeRecord{
		ChangeTitle:   req.ChangeTitle,
		ChangeType:    req.ChangeType,
		ChangeLevel:   req.ChangeLevel,
		ChangeContent: req.ChangeContent,
		ImpactScope:   req.ImpactScope,
		ChangeIPList:  req.ChangeIPList,
		ChangeTime:    req.ChangeTime,
		Operator:      operator,
		RollbackPlan:  req.RollbackPlan,
		Remark:        req.Remark,
	})

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "运维变更记录创建成功"})
}

func updateOpsChangeRecordHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	operator := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		operator = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		operator = username.(string)
	}

	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}

	var req OpsChangeUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	err := appsql.UpdateOpsChangeRecord(appsql.OpsChangeRecord{
		ID:            req.ID,
		ChangeTitle:   req.ChangeTitle,
		ChangeType:    req.ChangeType,
		ChangeLevel:   req.ChangeLevel,
		ChangeContent: req.ChangeContent,
		ImpactScope:   req.ImpactScope,
		ChangeIPList:  req.ChangeIPList,
		ChangeTime:    req.ChangeTime,
		Operator:      operator,
		RollbackPlan:  req.RollbackPlan,
		Remark:        req.Remark,
	}, roleName)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "运维变更记录更新成功"})
}

func deleteOpsChangeRecordHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	operator := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		operator = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		operator = username.(string)
	}

	roleName := "user"
	if r, ok := c.Get("currentRole"); ok {
		roleName = r.(string)
	}

	var req OpsChangeDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.DeleteOpsChangeRecord(req.ID, operator, roleName); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "运维变更记录删除成功"})
}

func adminListOpsChangeRecordsHandler(c *gin.Context) {
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

	operator := c.Query("operator")
	changeType := c.Query("changeType")
	changeLevel := c.Query("changeLevel")

	total, records, err := appsql.QueryOpsChangeRecords(page, pageSize, operator, changeType, changeLevel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "total": 0, "records": []any{}})
		return
	}

	if records == nil {
		records = []appsql.OpsChangeRecord{}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "获取成功",
		"total":   total,
		"records": records,
	})
}

func adminUpdateOpsChangeReviewerHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	// 复核人自动取当前登录用户
	reviewer := "admin"
	if displayName, ok := c.Get("currentDisplayName"); ok && displayName.(string) != "" {
		reviewer = displayName.(string)
	} else if username, ok := c.Get("currentUsername"); ok && username.(string) != "" {
		reviewer = username.(string)
	}

	var req OpsChangeReviewerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.UpdateOpsChangeRecordReviewer(req.ID, reviewer); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "复核成功"})
}

func adminUpdateOpsChangeResultHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	var req OpsChangeResultReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.UpdateOpsChangeRecordResult(req.ID, req.ChangeResult); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "确认失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "变更结果确认成功"})
}

func adminUpdateOpsChangeRollbackHandler(c *gin.Context) {
	_, _, ok := getCurrentUserContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
		return
	}

	var req OpsChangeRollbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误：" + err.Error()})
		return
	}

	if err := appsql.UpdateOpsChangeRecordRollback(req.ID, req.RollbackStatus); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "确认失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "回滚状态确认成功"})
}
