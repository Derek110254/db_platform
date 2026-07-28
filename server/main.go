package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"db_platform/server/auth"
	"db_platform/server/config"
	"db_platform/server/routes"

	"github.com/gin-gonic/gin"
)

//go:embed web/dist/**
var dist embed.FS

// main 是服务端入口。
// 启动流程：清理过期会话、初始化 Gin、注册访问日志、注册 API、挂载前端静态资源。
func main() {
	flag.Usage = func() {
		writeUsage(flag.CommandLine.Output(), filepath.Base(os.Args[0]))
	}
	configFile := flag.String("config_file", "", "YAML 配置文件路径")
	flag.Parse()
	if err := config.LoadConfig(*configFile); err != nil {
		panic(err)
	}

	if err := auth.DeleteExpiredSessions(); err != nil {
		panic(fmt.Sprintf("清理过期会话失败: %v", err))
	}

	accessLogger, accessLogWriter, err := newAccessLogger()
	if err != nil {
		panic(err)
	}
	defer accessLogWriter.Close()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(accessLogMiddleware(accessLogger))

	routes.RegisterAPIRoutes(r)

	distFS, err := fs.Sub(dist, "web/dist")
	if err != nil {
		panic(err)
	}
	routes.RegisterWebRoutes(r, distFS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "1520"
	}

	if err := r.Run(":" + port); err != nil {
		panic(err)
	}
}

// writeUsage 输出与 README 一致的双横线启动参数说明。
func writeUsage(output io.Writer, command string) {
	fmt.Fprintf(output, "用法: %s --config_file <配置文件路径>\n\n", command)
	fmt.Fprintln(output, "参数:")
	fmt.Fprintln(output, "  --config_file string")
	fmt.Fprintln(output, "        YAML 配置文件路径（必填）")
}

const (
	accessLogDateLayout    = "2006-01-02"
	accessLogRetentionDays = 14
)

// dailyAccessLogWriter 按本地日期将日志写入 access-YYYY-MM-DD.log。
// 每次写入前都会检查日期，因此服务跨过午夜后无需重启即可切换文件。
type dailyAccessLogWriter struct {
	mu      sync.Mutex
	logDir  string
	date    string
	file    *os.File
	nowFunc func() time.Time
}

// newAccessLogger 创建 API 访问日志记录器。
// 启动时会自动创建日志目录和当天文件，并提前验证目标路径是否可写。
func newAccessLogger() (*log.Logger, io.WriteCloser, error) {
	logDir := strings.TrimSpace(os.Getenv("LOG_DIR"))
	if logDir == "" {
		logDir = "logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	logWriter, err := newDailyAccessLogWriter(logDir, time.Now)
	if err != nil {
		return nil, nil, err
	}

	logger := log.New(logWriter, "", 0)
	return logger, logWriter, nil
}

func newDailyAccessLogWriter(logDir string, nowFunc func() time.Time) (*dailyAccessLogWriter, error) {
	writer := &dailyAccessLogWriter{logDir: logDir, nowFunc: nowFunc}
	if err := writer.rotateIfNeededLocked(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (writer *dailyAccessLogWriter) Write(content []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	if err := writer.rotateIfNeededLocked(); err != nil {
		return 0, err
	}
	return writer.file.Write(content)
}

func (writer *dailyAccessLogWriter) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	if writer.file == nil {
		return nil
	}
	err := writer.file.Close()
	writer.file = nil
	return err
}

func (writer *dailyAccessLogWriter) rotateIfNeededLocked() error {
	now := writer.nowFunc()
	date := now.Format(accessLogDateLayout)
	if writer.file != nil && writer.date == date {
		return nil
	}

	logPath := filepath.Join(writer.logDir, "access-"+date+".log")
	newFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建当天访问日志失败: %w", err)
	}
	if writer.file != nil {
		_ = writer.file.Close()
	}
	writer.file = newFile
	writer.date = date
	cleanupExpiredAccessLogs(writer.logDir, now)
	return nil
}

// cleanupExpiredAccessLogs 清理超过保留期的每日访问日志；清理失败不影响当天日志继续写入。
func cleanupExpiredAccessLogs(logDir string, now time.Time) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := dayStart.AddDate(0, 0, -(accessLogRetentionDays - 1))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "access-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		dateText := strings.TrimSuffix(strings.TrimPrefix(name, "access-"), ".log")
		logDate, err := time.ParseInLocation(accessLogDateLayout, dateText, now.Location())
		if err == nil && logDate.Before(cutoff) {
			_ = os.Remove(filepath.Join(logDir, name))
		}
	}
}

// accessLogMiddleware 记录全部 /api/ 请求的来源、状态码和耗时，不记录请求体与密码。
func accessLogMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		path := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			path += "?" + c.Request.URL.RawQuery
		}
		if len(path) < 5 || path[:5] != "/api/" {
			return
		}

		logger.Printf(
			"[%s] ip=%s method=%s path=%s status=%d latency=%s ua=%q",
			time.Now().Format("2006-01-02 15:04:05"),
			c.ClientIP(),
			c.Request.Method,
			path,
			c.Writer.Status(),
			time.Since(start).String(),
			c.Request.UserAgent(),
		)
	}
}
