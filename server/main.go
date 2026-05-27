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
	"gopkg.in/natefinch/lumberjack.v2" // 新增的日志轮转库
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

	// 初始化访问日志记录器
	accessLogger, accessLogWriter, err := newAccessLogger()
	if err != nil {
		panic(err)
	}
	defer accessLogWriter.Close()

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
// 创建访问日志记录器，支持按天轮转和保留 14 天。
// 日志文件路径：logs/access.log
func newAccessLogger() (*log.Logger, io.WriteCloser, error) {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, err
	}

	logPath := filepath.Join(logDir, "access.log")

	// 引入 lumberjack 实现日志按天轮转与清理
	logWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    100,  // 单个日志文件最大尺寸（MB），达到 100MB 也会触发切割
		MaxAge:     14,   // 保留旧文件的最大天数（保留 14 天）
		MaxBackups: 0,    // 保留的最大旧文件数量（0 表示仅靠 MaxAge 控制，不限制个数）
		LocalTime:  true, // 备份文件名使用本地时间
		Compress:   true, // 压缩旧的日志文件（推荐开启，节省磁盘空间）
	}

	logger := log.New(logWriter, "", 0)
	return logger, logWriter, nil
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
