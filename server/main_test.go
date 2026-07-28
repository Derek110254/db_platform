package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteUsageShowsDoubleDashConfigFlag(t *testing.T) {
	var output bytes.Buffer
	writeUsage(&output, "db_platform")

	usage := output.String()
	if !strings.Contains(usage, "db_platform --config_file <配置文件路径>") {
		t.Fatalf("usage does not show the expected flag: %q", usage)
	}
	if strings.Contains(usage, "\n  -config_file") {
		t.Fatalf("usage unexpectedly shows the single-dash flag: %q", usage)
	}
}

// TestNewAccessLoggerCreatesMissingDirectory 验证编译后的程序首次启动时可自动创建日志目录和文件。
func TestNewAccessLoggerCreatesMissingDirectory(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "nested", "logs")
	t.Setenv("LOG_DIR", logDir)

	logger, writer, err := newAccessLogger()
	if err != nil {
		t.Fatalf("newAccessLogger returned error: %v", err)
	}
	logPath := filepath.Join(logDir, "access-"+time.Now().Format(accessLogDateLayout)+".log")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("daily access log was not created during initialization: %v", err)
	}
	logger.Print("access log test")
	if err := writer.Close(); err != nil {
		t.Fatalf("close log writer: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read daily access log: %v", err)
	}
	if !strings.Contains(string(content), "access log test") {
		t.Fatalf("daily access log does not contain test entry: %q", string(content))
	}
}

// TestDailyAccessLogWriterRotatesAtMidnight 验证服务持续运行跨过午夜时自动切换到新文件。
func TestDailyAccessLogWriterRotatesAtMidnight(t *testing.T) {
	logDir := t.TempDir()
	currentTime := time.Date(2026, 7, 26, 23, 59, 59, 0, time.Local)
	writer, err := newDailyAccessLogWriter(logDir, func() time.Time { return currentTime })
	if err != nil {
		t.Fatalf("create daily writer: %v", err)
	}

	logger := log.New(writer, "", 0)
	logger.Print("day one")
	currentTime = currentTime.Add(2 * time.Second)
	logger.Print("day two")
	if err := writer.Close(); err != nil {
		t.Fatalf("close daily writer: %v", err)
	}

	dayOne, err := os.ReadFile(filepath.Join(logDir, "access-2026-07-26.log"))
	if err != nil {
		t.Fatalf("read first daily log: %v", err)
	}
	dayTwo, err := os.ReadFile(filepath.Join(logDir, "access-2026-07-27.log"))
	if err != nil {
		t.Fatalf("read second daily log: %v", err)
	}
	if !strings.Contains(string(dayOne), "day one") || strings.Contains(string(dayOne), "day two") {
		t.Fatalf("unexpected first daily log: %q", string(dayOne))
	}
	if !strings.Contains(string(dayTwo), "day two") {
		t.Fatalf("unexpected second daily log: %q", string(dayTwo))
	}
}
