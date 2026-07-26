package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"db_platform/server/auth"
	appsql "db_platform/server/sql"

	_ "github.com/denisenkom/go-mssqldb"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/sijms/go-ora/v2"
)

/*
api.go
----------------------------------------------------------------------
集中定义数据库查询平台的 JSON API、请求模型和处理函数。

路由边界：
1. 登录、退出、当前用户和健康检查为公开接口。
2. 查询、导出、元数据、收藏和改密必须通过登录校验。
3. 用户与数据库连接维护必须通过管理员校验。
4. API 响应禁止浏览器缓存，避免退出后显示旧的认证或业务数据。
5. 查询执行、连接权限和库/schema 范围限制由 sql 包继续校验。
*/

// LoginRequest 是登录接口的请求体。
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ChangePasswordRequest 是当前用户修改密码的请求体。
type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// QueryExecuteRequest 承载查询执行和 Excel 导出的 SQL 参数。
type QueryExecuteRequest struct {
	ConnectionName string `json:"connectionName"`
	SQL            string `json:"sql"`
}

// QueryMetadataRequest 用于按连接和关键字检索表、字段元数据。
type QueryMetadataRequest struct {
	ConnectionName string `json:"connectionName"`
	Keyword        string `json:"keyword"`
}

// SQLFavoriteCreateRequest 是新增 SQL 收藏的请求体。
type SQLFavoriteCreateRequest struct {
	FavoriteName   string `json:"favoriteName"`
	SQLText        string `json:"sqlText"`
	DBType         string `json:"dbType"`
	ConnectionName string `json:"connectionName"`
	Remark         string `json:"remark"`
	IsPinned       int    `json:"isPinned"`
}

// SQLFavoriteUpdateRequest 是更新 SQL 收藏的请求体。
type SQLFavoriteUpdateRequest struct {
	ID             int64  `json:"id"`
	FavoriteName   string `json:"favoriteName"`
	SQLText        string `json:"sqlText"`
	DBType         string `json:"dbType"`
	ConnectionName string `json:"connectionName"`
	Remark         string `json:"remark"`
	IsPinned       int    `json:"isPinned"`
}

// SQLFavoriteDeleteRequest 是删除 SQL 收藏的请求体。
type SQLFavoriteDeleteRequest struct {
	ID int64 `json:"id"`
}

// AdminCreateUserRequest 是管理员新增用户时提交的用户和连接授权信息。
type AdminCreateUserRequest struct {
	Username           string   `json:"username"`
	Password           string   `json:"password"`
	DisplayName        string   `json:"displayName"`
	RoleName           string   `json:"roleName"`
	IsEnabled          int      `json:"isEnabled"`
	CanQueryData       int      `json:"canQueryData"`
	AllowedConnections []string `json:"allowedConnections"`
}

// AdminUpdateUserRequest 是管理员更新用户时提交的用户和连接授权信息。
type AdminUpdateUserRequest struct {
	ID                 int64    `json:"id"`
	Username           string   `json:"username"`
	Password           string   `json:"password"`
	DisplayName        string   `json:"displayName"`
	RoleName           string   `json:"roleName"`
	IsEnabled          int      `json:"isEnabled"`
	CanQueryData       int      `json:"canQueryData"`
	AllowedConnections []string `json:"allowedConnections"`
}

// AdminDeleteUserRequest 是管理员删除用户的请求体。
type AdminDeleteUserRequest struct {
	ID int64 `json:"id"`
}

// AdminConnectionRequest 是管理员维护数据库连接时使用的统一请求体。
type AdminConnectionRequest struct {
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

// AdminDeleteConnectionRequest 是管理员删除数据库连接的请求体。
type AdminDeleteConnectionRequest struct {
	ID int64 `json:"id"`
}

// RegisterAPIRoutes 注册公开、登录态和管理员三组 API，并统一关闭响应缓存。
func RegisterAPIRoutes(r *gin.Engine) {
	api := r.Group("/api")
	api.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		c.Next()
	})
	{
		api.GET("/hello", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true, "message": "ok"})
		})
		api.POST("/login", loginHandler)
		api.POST("/logout", logoutHandler)
		api.GET("/auth/me", authMeHandler)

		// 登录用户可访问查询、元数据、导出、收藏和个人密码接口。
		authed := api.Group("/")
		authed.Use(auth.RequireLogin())
		{
			authed.POST("/user/change-password", changePasswordHandler)
			authed.GET("/query-connections", queryConnectionsHandler)
			authed.POST("/query-data", queryDataHandler)
			authed.POST("/query-export-excel", queryExportExcelHandler)
			authed.POST("/query-metadata", queryMetadataHandler)
			authed.GET("/sql-favorites", listSQLFavoritesHandler)
			authed.POST("/sql-favorites", createSQLFavoriteHandler)
			authed.PUT("/sql-favorites", updateSQLFavoriteHandler)
			authed.DELETE("/sql-favorites", deleteSQLFavoriteHandler)
		}

		// 管理员接口只保留用户权限和数据库连接管理。
		admin := api.Group("/admin")
		admin.Use(auth.RequireAdmin())
		{
			admin.GET("/users", adminListUsersHandler)
			admin.POST("/users", adminCreateUserHandler)
			admin.PUT("/users", adminUpdateUserHandler)
			admin.DELETE("/users", adminDeleteUserHandler)
			admin.GET("/db-connections", adminListConnectionsHandler)
			admin.POST("/db-connections", adminCreateConnectionHandler)
			admin.PUT("/db-connections", adminUpdateConnectionHandler)
			admin.DELETE("/db-connections", adminDeleteConnectionHandler)
			admin.POST("/db-connections/test", adminTestConnectionHandler)
		}
	}
}

func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	user, token, err := auth.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}

	auth.WriteSessionCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "登录成功", "user": sessionUserPayload(user)})
}

func logoutHandler(c *gin.Context) {
	token := auth.ReadSessionCookie(c)
	_ = auth.DeleteSessionByToken(token)
	auth.ClearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已退出登录"})
}

func authMeHandler(c *gin.Context) {
	token := auth.ReadSessionCookie(c)
	if token == "" {
		respondUnauthorized(c)
		return
	}

	user, ok, err := auth.GetUserBySessionToken(token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	if !ok {
		auth.ClearSessionCookie(c)
		respondUnauthorized(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "user": sessionUserPayload(user)})
}

func sessionUserPayload(user auth.SessionUser) gin.H {
	return gin.H{
		"userId":        user.UserID,
		"username":      user.Username,
		"displayName":   user.DisplayName,
		"role":          user.RoleName,
		"roleName":      user.RoleName,
		"canQueryData":  user.CanQueryData,
		"needChangePwd": user.NeedChangePwd,
	}
}

func changePasswordHandler(c *gin.Context) {
	userID, _, ok := getCurrentUserContext(c)
	if !ok {
		respondUnauthorized(c)
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "新密码不能为空"})
		return
	}

	if err := appsql.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "密码修改成功"})
}

func getCurrentUserContext(c *gin.Context) (int64, string, bool) {
	userIDValue, ok := c.Get("currentUserID")
	if !ok {
		return 0, "", false
	}

	var userID int64
	switch v := userIDValue.(type) {
	case int64:
		userID = v
	case int:
		userID = int64(v)
	case float64:
		userID = int64(v)
	default:
		return 0, "", false
	}

	roleName := "user"
	if v, ok := c.Get("currentRole"); ok {
		if s, ok := v.(string); ok && s != "" {
			roleName = s
		}
	}
	return userID, roleName, true
}

func queryConnectionsHandler(c *gin.Context) {
	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		respondUnauthorized(c)
		return
	}

	items, err := appsql.LoadEnabledConnectionsForUser(userID, roleName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "connections": []any{}})
		return
	}

	connections := make([]gin.H, 0, len(items))
	for _, item := range items {
		connections = append(connections, gin.H{
			"name":        item.Name,
			"dbType":      item.DBType,
			"host":        item.Host,
			"port":        item.Port,
			"database":    item.DatabaseName,
			"serviceName": item.ServiceName,
			"canConnect":  item.CanConnect,
		})
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "获取成功", "connections": connections})
}

func queryDataHandler(c *gin.Context) {
	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		respondUnauthorized(c)
		return
	}

	var req QueryExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	result := appsql.ExecuteQueryByConnectionWithContext(c.Request.Context(), userID, roleName, req.ConnectionName, req.SQL)
	c.JSON(http.StatusOK, result)
}

func queryExportExcelHandler(c *gin.Context) {
	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		respondUnauthorized(c)
		return
	}

	var req QueryExecuteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	fileName, bytes, err := appsql.ExportQueryResultToExcelWithContext(c.Request.Context(), userID, roleName, req.ConnectionName, req.SQL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}

	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	c.Data(http.StatusOK, contentType, bytes)
}

func queryMetadataHandler(c *gin.Context) {
	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		respondUnauthorized(c)
		return
	}

	var req QueryMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	result := appsql.SearchQueryMetadataByConnectionWithUser(userID, roleName, req.ConnectionName, req.Keyword)
	c.JSON(http.StatusOK, result)
}

func listSQLFavoritesHandler(c *gin.Context) {
	userID, _, ok := getCurrentUserContext(c)
	if !ok {
		respondUnauthorized(c)
		return
	}

	items, err := appsql.ListSQLFavorites(userID, c.Query("dbType"), c.Query("connectionName"), c.Query("keyword"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "favorites": []any{}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "获取成功", "favorites": items})
}

func createSQLFavoriteHandler(c *gin.Context) {
	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		respondUnauthorized(c)
		return
	}

	var req SQLFavoriteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
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
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "收藏保存成功"})
}

func updateSQLFavoriteHandler(c *gin.Context) {
	userID, roleName, ok := getCurrentUserContext(c)
	if !ok {
		respondUnauthorized(c)
		return
	}

	var req SQLFavoriteUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
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
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "收藏更新成功"})
}

func deleteSQLFavoriteHandler(c *gin.Context) {
	userID, _, ok := getCurrentUserContext(c)
	if !ok {
		respondUnauthorized(c)
		return
	}

	var req SQLFavoriteDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	if err := appsql.DeleteSQLFavorite(userID, req.ID); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "收藏删除成功"})
}

func adminListUsersHandler(c *gin.Context) {
	items, err := appsql.ListUsers()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "users": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "获取成功", "users": items})
}

func adminCreateUserHandler(c *gin.Context) {
	var req AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	if err := appsql.ValidateEnabledConnectionNames(req.AllowedConnections); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}

	_, err := appsql.CreateUser(req.Username, req.Password, req.DisplayName, req.RoleName, req.IsEnabled, req.CanQueryData, req.AllowedConnections)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "用户创建成功"})
}

func adminUpdateUserHandler(c *gin.Context) {
	var req AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	if err := appsql.ValidateEnabledConnectionNames(req.AllowedConnections); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}

	err := appsql.UpdateUser(req.ID, req.Username, req.Password, req.DisplayName, req.RoleName, req.IsEnabled, req.CanQueryData, req.AllowedConnections)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "用户更新成功"})
}

func adminDeleteUserHandler(c *gin.Context) {
	var req AdminDeleteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	if err := appsql.DeleteUser(req.ID); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "用户删除成功"})
}

func adminListConnectionsHandler(c *gin.Context) {
	items, err := appsql.ListConnections()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error(), "connections": []any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "获取成功", "connections": items})
}

func adminCreateConnectionHandler(c *gin.Context) {
	var req AdminConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	if err := appsql.CreateConnection(connectionRecordFromRequest(req)); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "连接创建成功"})
}

func adminUpdateConnectionHandler(c *gin.Context) {
	var req AdminConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	if err := appsql.UpdateConnection(req.ID, connectionRecordFromRequest(req)); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "连接更新成功"})
}

func adminDeleteConnectionHandler(c *gin.Context) {
	var req AdminDeleteConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	if err := appsql.DeleteConnection(req.ID); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "连接删除成功"})
}

func adminTestConnectionHandler(c *gin.Context) {
	var req AdminConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	if err := testPort(req.Host, req.Port); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "errorType": "port", "message": "端口连接失败: " + err.Error()})
		return
	}

	password := req.Password
	if password == "" && req.ID > 0 {
		plain, err := appsql.GetConnectionPasswordPlain(req.ID)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "errorType": "auth", "message": err.Error()})
			return
		}
		password = plain
	}

	if err := testDatabaseLogin(req, password); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "errorType": "auth", "message": "数据库登录失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "连接测试成功"})
}

func connectionRecordFromRequest(req AdminConnectionRequest) appsql.ConnectionRecord {
	return appsql.ConnectionRecord{
		ID:           req.ID,
		Name:         strings.TrimSpace(req.Name),
		DBType:       strings.ToLower(strings.TrimSpace(req.DBType)),
		Host:         strings.TrimSpace(req.Host),
		Port:         req.Port,
		Username:     strings.TrimSpace(req.Username),
		Password:     req.Password,
		DatabaseName: strings.TrimSpace(req.DatabaseName),
		ServiceName:  strings.TrimSpace(req.ServiceName),
		IsEnabled:    req.IsEnabled,
		CanConnect:   req.CanConnect,
	}
}

func testPort(host string, port int) error {
	if strings.TrimSpace(host) == "" || port <= 0 {
		return errors.New("主机和端口不能为空")
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 3*time.Second)
	if err != nil {
		return err
	}
	return conn.Close()
}

func testDatabaseLogin(req AdminConnectionRequest, password string) error {
	dbType := strings.ToLower(strings.TrimSpace(req.DBType))
	driverName := dbType
	var dsn string
	switch dbType {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local", req.Username, password, req.Host, req.Port, req.DatabaseName)
	case "oracle":
		dsn = fmt.Sprintf("oracle://%s:%s@%s:%d/%s", req.Username, password, req.Host, req.Port, req.ServiceName)
	case "postgres":
		dsn = buildPostgresTestDSN(req, password)
	case "mssql":
		driverName = "sqlserver"
		dsn = buildMSSQLTestDSN(req, password)
	default:
		return fmt.Errorf("不支持的数据库类型: %s", req.DBType)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

// buildPostgresTestDSN 连接指定数据库；填写 schema 时测试同样会校验 search_path。
func buildPostgresTestDSN(req AdminConnectionRequest, password string) string {
	values := url.Values{}
	values.Set("sslmode", "disable")
	if strings.TrimSpace(req.ServiceName) != "" {
		values.Set("search_path", strings.TrimSpace(req.ServiceName))
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?%s",
		url.QueryEscape(req.Username),
		url.QueryEscape(password),
		req.Host,
		req.Port,
		url.PathEscape(strings.TrimSpace(req.DatabaseName)),
		values.Encode(),
	)
}

// buildMSSQLTestDSN 连接指定 SQL Server 数据库，连接参数与 MySQL 一样不包含 schema。
func buildMSSQLTestDSN(req AdminConnectionRequest, password string) string {
	values := url.Values{}
	values.Set("database", strings.TrimSpace(req.DatabaseName))
	values.Set("encrypt", "disable")

	return fmt.Sprintf(
		"sqlserver://%s:%s@%s:%d?%s",
		url.QueryEscape(req.Username),
		url.QueryEscape(password),
		req.Host,
		req.Port,
		values.Encode(),
	)
}

func respondBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "请求格式错误: " + err.Error()})
}

func respondUnauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "message": "未登录"})
}
