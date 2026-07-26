package config

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

/*
platform_db.go
----------------------------------------------------------------------
集中管理数据库查询平台的管控库连接。

管控库保存以下数据：
1. 用户账号与角色信息。
2. 登录会话。
3. 可查询数据库连接配置。
4. 用户与数据库连接的授权关系。
5. SQL 收藏夹。

当前配置仍然写在代码常量中，便于单机部署
*/

const (
	PlatformDBHost     = "127.0.0.1"       // 管控库主机
	PlatformDBPort     = 3306              // 管控库端口
	PlatformDBUser     = "db_platform"     // 管控库用户名
	PlatformDBPassword = "db_platform"     // 管控库密码
	PlatformDBName     = "db_platform"     // 管控库名称

	SessionCookieName  = "db_platform_session_token" // 登录会话 Cookie 名称
	SessionExpireHours = 8                           // 会话有效期，单位：小时
)

var (
	platformDB     *sql.DB
	platformDBOnce sync.Once
	platformDBErr  error
)

// GetPlatformDB 返回全局共享的管控库连接池。
// 连接池在进程内只初始化一次，所有认证、授权和配置管理模块复用同一个 *sql.DB。
func GetPlatformDB() (*sql.DB, error) {
	platformDBOnce.Do(func() {
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			PlatformDBUser,
			PlatformDBPassword,
			PlatformDBHost,
			PlatformDBPort,
			PlatformDBName,
		)

		db, err := sql.Open("mysql", dsn)
		if err != nil {
			platformDBErr = err
			return
		}

		// 默认连接池规模适合小型内部平台，可按并发量继续调整。
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
