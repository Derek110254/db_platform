package config

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/goccy/go-yaml"
)

// PlatformDBConfig 描述平台管控库的 MySQL 连接参数。
type PlatformDBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

// SessionConfig 描述登录会话 Cookie 和有效期。
type SessionConfig struct {
	CookieName  string `yaml:"cookie_name"`
	ExpireHours int    `yaml:"expire_hours"`
}

// AppConfig 是服务启动时从 YAML 文件加载的完整配置。
type AppConfig struct {
	PlatformDB PlatformDBConfig `yaml:"platform_db"`
	Session    SessionConfig    `yaml:"session"`
}

var (
	configMu      sync.RWMutex
	currentConfig AppConfig
	configLoaded  bool

	platformDB     *sql.DB
	platformDBOnce sync.Once
	platformDBErr  error
)

// LoadConfig 读取并校验启动配置。配置只应在服务启动阶段加载一次。
func LoadConfig(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("必须通过 --config_file 指定配置文件")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败 %q: %w", path, err)
	}

	var loaded AppConfig
	if err := yaml.Unmarshal(content, &loaded); err != nil {
		return fmt.Errorf("解析配置文件失败 %q: %w", path, err)
	}
	applyConfigDefaults(&loaded)
	if err := validateConfig(loaded); err != nil {
		return fmt.Errorf("配置文件无效 %q: %w", path, err)
	}

	configMu.Lock()
	currentConfig = loaded
	configLoaded = true
	configMu.Unlock()
	return nil
}

func applyConfigDefaults(appConfig *AppConfig) {
	if appConfig.PlatformDB.Port == 0 {
		appConfig.PlatformDB.Port = 3306
	}
	if strings.TrimSpace(appConfig.Session.CookieName) == "" {
		appConfig.Session.CookieName = "db_platform_session_token"
	}
	if appConfig.Session.ExpireHours == 0 {
		appConfig.Session.ExpireHours = 8
	}
}

func validateConfig(appConfig AppConfig) error {
	dbConfig := appConfig.PlatformDB
	if strings.TrimSpace(dbConfig.Host) == "" {
		return errors.New("platform_db.host 不能为空")
	}
	if dbConfig.Port < 1 || dbConfig.Port > 65535 {
		return errors.New("platform_db.port 必须在 1 到 65535 之间")
	}
	if strings.TrimSpace(dbConfig.User) == "" {
		return errors.New("platform_db.user 不能为空")
	}
	if dbConfig.Password == "" {
		return errors.New("platform_db.password 不能为空")
	}
	if strings.TrimSpace(dbConfig.Name) == "" {
		return errors.New("platform_db.name 不能为空")
	}
	if appConfig.Session.ExpireHours < 1 {
		return errors.New("session.expire_hours 必须大于 0")
	}
	return nil
}

// GetSessionConfig 返回当前会话配置。
func GetSessionConfig() SessionConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	return currentConfig.Session
}

func getPlatformDBConfig() (PlatformDBConfig, error) {
	configMu.RLock()
	defer configMu.RUnlock()
	if !configLoaded {
		return PlatformDBConfig{}, errors.New("服务配置尚未加载")
	}
	return currentConfig.PlatformDB, nil
}

// GetPlatformDB 返回全局共享的管控库连接池。
func GetPlatformDB() (*sql.DB, error) {
	platformDBOnce.Do(func() {
		dbConfig, err := getPlatformDBConfig()
		if err != nil {
			platformDBErr = err
			return
		}

		db, err := sql.Open("mysql", buildPlatformDBDSN(dbConfig))
		if err != nil {
			platformDBErr = err
			return
		}

		db.SetMaxOpenConns(20)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(2 * time.Hour)

		if err := db.Ping(); err != nil {
			_ = db.Close()
			platformDBErr = err
			return
		}
		platformDB = db
	})

	return platformDB, platformDBErr
}

// buildPlatformDBDSN 在驱动默认配置上设置连接参数，保留其默认认证兼容策略。
func buildPlatformDBDSN(dbConfig PlatformDBConfig) string {
	dsnConfig := mysql.NewConfig()
	dsnConfig.User = dbConfig.User
	dsnConfig.Passwd = dbConfig.Password
	dsnConfig.Net = "tcp"
	dsnConfig.Addr = net.JoinHostPort(dbConfig.Host, strconv.Itoa(dbConfig.Port))
	dsnConfig.DBName = dbConfig.Name
	dsnConfig.Params = map[string]string{"charset": "utf8mb4"}
	dsnConfig.ParseTime = true
	dsnConfig.Loc = time.Local
	return dsnConfig.FormatDSN()
}
