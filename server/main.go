package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"sql_platform/server/auth"
	"sql_platform/server/routes"

	"github.com/gin-gonic/gin"
)

//go:embed web/dist/**
var dist embed.FS

// main
// ------------------------------------------------------------
// 程序入口：
// 1. 初始化平台认证库表结构；
// 2. 初始化 Gin；
// 3. 注册日志；
// 4. 注册 API 路由；
// 5. 注册前端静态页面。
func main() {
	// 初始化认证与连接配置表。
	// 因为这次采用“代码中固定配置”的方式，所以这里不依赖环境变量。
	if err := auth.EnsureSchema(); err != nil {
		panic(fmt.Sprintf("初始化认证数据库失败: %v", err))
	}

	accessLogger, accessLogFile, err := newAccessLogger()
	if err != nil {
		panic(err)
	}
	defer accessLogFile.Close()

	r := gin.New()

	// 恢复中间件，防止 panic 直接导致进程退出
	r.Use(gin.Recovery())

	// 控制台日志
	r.Use(gin.Logger())

	// 写 access.log 的访问日志中间件
	r.Use(accessLogMiddleware(accessLogger))

	// 注册后端 API
	routes.RegisterAPIRoutes(r)

	// 注册前端页面
	distFS, err := fs.Sub(dist, "web/dist")
	if err != nil {
		panic(err)
	}
	routes.RegisterWebRoutes(r, distFS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "2345"
	}

	if err := r.Run(":" + port); err != nil {
		panic(err)
	}
}

// newAccessLogger
// ------------------------------------------------------------
// 创建访问日志记录器。
// 日志文件路径：logs/access.log
func newAccessLogger() (*log.Logger, *os.File, error) {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, err
	}

	logPath := filepath.Join(logDir, "access.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, err
	}

	logger := log.New(io.Writer(logFile), "", 0)
	return logger, logFile, nil
}

// accessLogMiddleware
// ------------------------------------------------------------
// 记录接口访问日志。
// 只记录 /api/ 开头的访问，便于排查接口调用问题。
func accessLogMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery
		statusCode := c.Writer.Status()
		userAgent := c.Request.UserAgent()

		if rawQuery != "" {
			path = path + "?" + rawQuery
		}

		if len(path) >= 5 && path[:5] == "/api/" {
			logger.Printf(
				"[%s] ip=%s method=%s path=%s status=%d latency=%s ua=%q",
				time.Now().Format("2006-01-02 15:04:05"),
				clientIP,
				method,
				path,
				statusCode,
				latency.String(),
				userAgent,
			)
		}
	}
}
