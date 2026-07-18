package auth

import (
	"crypto/md5"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"time"

	"sql_platform/server/config"

	"github.com/gin-gonic/gin"
)

/*
session.go
----------------------------------------------------------------------
该文件负责平台登录、会话管理、数据库连接配置读取、用户与连接权限关系维护。

本版本在原有基础上新增：
1. platform_sql_favorite 表结构初始化
2. 为 SQL 收藏功能提供数据表支持

当前职责：
1. 平台用户登录 / 登出 / 会话管理
2. 数据库连接配置读取
3. 用户可访问数据库连接关系维护
4. 平台表结构初始化
*/

// ----------------------------------------------------------------------
// 一、固定配置
// ----------------------------------------------------------------------

const sessionCookieName = "platform_session_token"
const sessionExpireHours = 8

// 保留固定密钥常量，便于和数据库 fixed_aes_encrypt / fixed_aes_decrypt 语义保持一致。
const fixedSecretKey = "db_platform_fixed_secret_key_32"

// SessionUser
// ----------------------------------------------------------------------
// 当前登录用户信息
type SessionUser struct {
	UserID        int64  `json:"userId"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	RoleName      string `json:"roleName"`
	CanQueryData  int    `json:"canQueryData"`
	CanQueryPlan  int    `json:"canQueryPlan"`
	NeedChangePwd int    `json:"needChangePwd"`
}

// DBConnectionRecord
// ----------------------------------------------------------------------
// 数据库连接配置记录
type DBConnectionRecord struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	DBType         string `json:"dbType"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	PasswordCipher string `json:"passwordCipher"`
	DatabaseName   string `json:"databaseName"`
	ServiceName    string `json:"serviceName"`
	IsEnabled      int    `json:"isEnabled"`
	CanConnect     int    `json:"canConnect"`
}

// getAuthDB
// ----------------------------------------------------------------------
// 获取平台管控库连接
func getAuthDB() (*sql.DB, error) {
	return config.GetPlatformDB()
}

// EnsureSchema
// ----------------------------------------------------------------------
// 初始化平台相关表结构。
//
// 本次包含：
// 1. platform_user                   平台登录用户表
// 2. platform_session                登录会话表
// 3. platform_db_connection          数据库连接配置表
// 4. platform_user_db_connection     用户可访问数据库连接关系表
// 5. platform_sql_favorite           SQL 收藏表
// 6. platform_sql_audit              SQL AI 审核记录表
// 7. platform_db_change_request      数据库变更申请表
// 8. platform_team_db_env            团队数据库环境配置表
// 9. platform_db_data_sync_request   数据库数据同步申请表
// 10. platform_db_alert_handle       数据库告警处理表
func EnsureSchema() error {
	db, err := getAuthDB()
	if err != nil {
		return err
	}

	createUserTable := `
CREATE TABLE IF NOT EXISTS platform_user (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    username VARCHAR(64) NOT NULL DEFAULT '' COMMENT '登录用户名，唯一',
    password_cipher VARCHAR(512) NOT NULL DEFAULT '' COMMENT '使用 fixed_aes_encrypt 加密后的密码密文',
    display_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '显示名称',
    role_name VARCHAR(64) NOT NULL DEFAULT 'user' COMMENT '角色名称：admin/user',
    is_enabled TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：1启用，0禁用',
    can_query_data TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许访问查询页面：1是，0否',
    can_query_plan TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许访问执行计划页面：1是，0否',
    need_change_pwd TINYINT NOT NULL DEFAULT 1 COMMENT '首次登录是否需要修改密码：1是，0否',
    is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '是否删除：1是，0否',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_platform_user_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台登录用户表';
`

	createSessionTable := `
CREATE TABLE IF NOT EXISTS platform_session (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    session_token VARCHAR(128) NOT NULL DEFAULT '' COMMENT '登录会话令牌',
    user_id BIGINT NOT NULL DEFAULT 0 COMMENT '用户ID',
    username VARCHAR(64) NOT NULL DEFAULT '' COMMENT '用户名冗余',
    expire_time DATETIME NOT NULL COMMENT '会话过期时间',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_platform_session_token (session_token),
    KEY idx_platform_session_user_id (user_id),
    KEY idx_platform_session_expire_time (expire_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台登录会话表';
`

	createConnectionTable := `
CREATE TABLE IF NOT EXISTS platform_db_connection (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '连接唯一名称，例如 mysql-dev',
    db_type VARCHAR(16) NOT NULL DEFAULT '' COMMENT '数据库类型：mysql/oracle',
    host VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库主机',
    port INT NOT NULL DEFAULT 0 COMMENT '数据库端口',
    username VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库账号',
    password_cipher VARCHAR(512) NOT NULL DEFAULT '' COMMENT '使用 fixed_aes_encrypt 加密后的数据库密码密文',
    database_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'MySQL 数据库名',
    service_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Oracle 服务名',
    is_enabled TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：1启用，0禁用',
    can_connect TINYINT NOT NULL DEFAULT 0 COMMENT '是否可连接：1可连接，0不可连接',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_platform_db_connection_name (name),
    KEY idx_platform_db_connection_db_type (db_type),
    KEY idx_platform_db_connection_is_enabled (is_enabled),
    KEY idx_platform_db_connection_can_connect (can_connect)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台数据库连接配置表';
`

	createUserConnectionTable := `
CREATE TABLE IF NOT EXISTS platform_user_db_connection (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    user_id BIGINT NOT NULL COMMENT '用户ID',
    connection_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '连接名称，对应 platform_db_connection.name',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_platform_user_db_connection (user_id, connection_name),
    KEY idx_platform_user_db_connection_user_id (user_id),
    KEY idx_platform_user_db_connection_connection_name (connection_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户可访问数据库连接关系表';
`

	// 新增：SQL 收藏表
	createSQLFavoriteTable := `
CREATE TABLE IF NOT EXISTS platform_sql_favorite (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    user_id BIGINT NOT NULL COMMENT '所属用户ID',
    favorite_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '收藏名称',
    sql_text LONGTEXT NOT NULL COMMENT 'SQL内容',
    db_type VARCHAR(16) NOT NULL DEFAULT '' COMMENT '数据库类型：mysql/oracle',
    connection_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '关联连接名称，可为空',
    remark VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注',
    is_pinned TINYINT NOT NULL DEFAULT 0 COMMENT '是否置顶：1是，0否',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_platform_sql_favorite_user_id (user_id),
    KEY idx_platform_sql_favorite_connection_name (connection_name),
    KEY idx_platform_sql_favorite_db_type (db_type),
    KEY idx_platform_sql_favorite_is_pinned (is_pinned)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL收藏表';
`

	// 新增：SQL AI 审核记录表 (使用 username 记录)
	createSqlAuditTable := `
CREATE TABLE IF NOT EXISTS platform_sql_audit (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    user_id BIGINT NOT NULL COMMENT '提交用户ID',
    connection_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '数据库连接名称',
    sql_text LONGTEXT NOT NULL COMMENT '提交审核的SQL内容',
	sql_digest VARCHAR(64) NOT NULL COMMENT 'SQL结构指纹哈希',
    execution_plan LONGTEXT COMMENT 'SQL执行计划',
    ai_suggestion LONGTEXT COMMENT 'AI审核建议文本',
    ai_score INT DEFAULT 0 COMMENT 'AI审核评分(0-100)',
    submit_audit TINYINT NOT NULL DEFAULT 0 COMMENT '提交审核（默认0未提交，1面向交易，2面向用户，3后台配置，4报表生成，5其他）',
    audit_passed TINYINT NOT NULL DEFAULT 0 COMMENT '审核通过（默认0未通过，1审核通过，-1审核驳回）',
    reviewer VARCHAR(64) NOT NULL DEFAULT 'admin' COMMENT '审核人员',
    remark VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注',
    create_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '审核时间',
    update_time datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_platform_sql_audit_user_id (user_id),
    KEY idx_platform_sql_audit_connection_name (connection_name),
    KEY idx_platform_sql_audit_create_time (create_time),
	KEY idx_sql_digest (sql_digest)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL AI审核记录表';
`

	// 新增：数据库变更申请表
	createDbChangeRequestTable := `
CREATE TABLE IF NOT EXISTS platform_db_change_request (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    applicant VARCHAR(64) NOT NULL DEFAULT '' COMMENT '申请人',
    applicant_team VARCHAR(128) NOT NULL DEFAULT '' COMMENT '申请团队',
    environment VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库环境',
    planned_change_time DATETIME NOT NULL COMMENT '计划变更时间',
    urgency_level VARCHAR(16) NOT NULL DEFAULT '常规' COMMENT '紧急程度（常规，紧急）',
    test_publisher VARCHAR(64) NOT NULL DEFAULT '' COMMENT '测试线发布人',
    prod_publisher VARCHAR(64) NOT NULL DEFAULT '' COMMENT '生产线发布人',
    change_type VARCHAR(64) NOT NULL DEFAULT '' COMMENT '变更类型（新建表，修改表结构，数据修改，数据同步，其他）',
    change_reason VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '变更原因',
    requirement_url VARCHAR(512) NOT NULL DEFAULT '' COMMENT '需求url',
    impact_scope VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '影响范围',
    db_type VARCHAR(32) NOT NULL DEFAULT '' COMMENT '数据库类型（oracle mysql redis 其他）',
    test_db_ip VARCHAR(128) NOT NULL DEFAULT '' COMMENT '测试线数据库IP',
    test_db_name VARCHAR(32) NOT NULL DEFAULT '' COMMENT '测试线数据库名',
    test_db_schema VARCHAR(128) NOT NULL DEFAULT '' COMMENT '测试线数据库schema',
    db_ip VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库IP',
    db_name VARCHAR(32) NOT NULL DEFAULT '' COMMENT '数据库名',
    db_schema VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库schema',
    change_content LONGTEXT COMMENT '变更内容',
    backup_table VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '备份表名，方便后续清理备份数据',
    release_verifier VARCHAR(64) NOT NULL DEFAULT '' COMMENT '发布验证人',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_platform_db_change_request_applicant (applicant),
    KEY idx_platform_db_change_request_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库变更申请表';
`

	// 新增：团队数据库环境配置表
	createTeamDbEnvTable := `
CREATE TABLE IF NOT EXISTS platform_team_db_env (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    team_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '团队名称，如：交易开发',
    env_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '环境名称，如：交易核心库、营销活动库',
    db_type VARCHAR(32) NOT NULL DEFAULT '' COMMENT '数据库类型：Oracle, MySQL, redis, 其他',
    test_db_ip VARCHAR(128) NOT NULL DEFAULT '' COMMENT '测试线数据库IP',
    test_db_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '测试线实例/数据库名',
    test_db_schema VARCHAR(128) NOT NULL DEFAULT '' COMMENT '测试线数据库schema',
    prod_db_ip VARCHAR(128) NOT NULL DEFAULT '' COMMENT '生产线数据库IP',
    prod_db_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '生产线实例/数据库名',
    prod_db_schema VARCHAR(128) NOT NULL DEFAULT '' COMMENT '生产线数据库schema',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_team_db_env_team_name (team_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='团队数据库环境配置表';
`
	// 新增：数据库数据同步申请表
	createDbDataSyncRequestTable := `
CREATE TABLE IF NOT EXISTS platform_db_data_sync_request (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    applicant VARCHAR(64) NOT NULL DEFAULT '' COMMENT '申请人',
    applicant_team VARCHAR(128) NOT NULL DEFAULT '' COMMENT '申请团队',
    environment VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库环境',
    expected_finish_time DATETIME NOT NULL COMMENT '期望完成时间',
    urgency_level VARCHAR(16) NOT NULL DEFAULT '常规' COMMENT '紧急程度（常规，重要，紧急）',
    urgency_reason VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '紧急原因',
    execute_dba VARCHAR(64) NOT NULL DEFAULT '' COMMENT '执行DBA',
    apply_reason VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '申请原因与目的',
    operate_type TINYINT NOT NULL DEFAULT 1 COMMENT '操作类型（1:迁移到其他数据库 2:迁移到测试库 3:导出为文件）',
    source_db VARCHAR(128) NOT NULL DEFAULT '' COMMENT '源数据库',
    target_db_or_person VARCHAR(128) NOT NULL DEFAULT '' COMMENT '目标库/目标人',
    involved_db_schema_table VARCHAR(512) NOT NULL DEFAULT '' COMMENT '涉及库名/schema名/表名',
    data_filter_condition LONGTEXT COMMENT '数据过滤条件',
    estimated_data_volume VARCHAR(64) NOT NULL DEFAULT '' COMMENT '预估数据量',
    contains_sensitive_data TINYINT NOT NULL DEFAULT 0 COMMENT '是否包含敏感信息（1:是 0:否）',
    desensitization_rule LONGTEXT COMMENT '脱敏规则说明',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_db_data_sync_req_applicant (applicant),
    KEY idx_db_data_sync_req_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库数据同步申请表';
`

	// 新增：数据库告警处理表
	createDbAlertHandleTable := `
CREATE TABLE IF NOT EXISTS platform_db_alert_handle (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    db_type VARCHAR(32) NOT NULL DEFAULT '' COMMENT '数据库类型（oracle mysql redis 其他）',
    alert_level VARCHAR(16) NOT NULL DEFAULT '一般' COMMENT '告警等级（一般，重要，紧急）',
    alert_category VARCHAR(32) NOT NULL DEFAULT '' COMMENT '告警分类（SQL性能,空间扩容,配置优化,可用性故障,锁与阻塞,备份恢复,硬件不足）',
    alert_content LONGTEXT NOT NULL COMMENT '告警内容',
    impact_scope VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '影响范围',
    alert_time DATETIME NOT NULL COMMENT '告警时间',
    handler VARCHAR(64) NOT NULL DEFAULT '' COMMENT '处理人',
    handle_start_time DATETIME COMMENT '处理开始时间',
    handle_end_time DATETIME COMMENT '处理结束时间',
    handle_result VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '处理结果',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_db_alert_handle_db_type (db_type),
    KEY idx_db_alert_handle_alert_level (alert_level),
    KEY idx_db_alert_handle_handler (handler),
    KEY idx_db_alert_handle_alert_time (alert_time),
    KEY idx_db_alert_handle_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库告警处理表';
`

	// 新增：运维变更记录表
	createOpsChangeRecordTable := `
CREATE TABLE IF NOT EXISTS platform_ops_change_record (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    change_title VARCHAR(256) NOT NULL DEFAULT '' COMMENT '变更标题（简述变更内容）',
    change_type VARCHAR(32) NOT NULL DEFAULT '' COMMENT '变更类型（安装部署,配置变更,服务重启,版本升级,数据修复,性能优化,容量变更,应急变更,其他）',
    change_level VARCHAR(16) NOT NULL DEFAULT '常规' COMMENT '变更等级（常规,重要,紧急）',
    change_content LONGTEXT NOT NULL COMMENT '变更内容（详细描述具体操作）',
    impact_scope VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '影响范围（受影响的系统/服务）',
    change_ip_list VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '变更IP列表',
    change_time DATETIME NOT NULL COMMENT '变更执行时间',
    operator VARCHAR(64) NOT NULL DEFAULT '' COMMENT '操作人',
    reviewer VARCHAR(64) NOT NULL DEFAULT '' COMMENT '复核人',
    change_result VARCHAR(16) NOT NULL DEFAULT '待确认' COMMENT '变更结果（待确认,成功,失败,部分成功）',
    rollback_plan LONGTEXT COMMENT '回滚方案',
    rollback_status VARCHAR(16) NOT NULL DEFAULT '待确认' COMMENT '回滚状态（待确认,无需回滚,已回滚,已失败）',
    remark VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_ops_change_record_operator (operator),
    KEY idx_ops_change_record_change_type (change_type),
    KEY idx_ops_change_record_change_level (change_level),
    KEY idx_ops_change_record_change_time (change_time),
    KEY idx_ops_change_record_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='运维变更记录表';
`

	for _, stmt := range []string{
		createUserTable,
		createSessionTable,
		createConnectionTable,
		createUserConnectionTable,
		createSQLFavoriteTable,
		createSqlAuditTable,
		createDbChangeRequestTable,
		createTeamDbEnvTable,
		createDbDataSyncRequestTable,
		createDbAlertHandleTable,
		createOpsChangeRecordTable,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	_ = DeleteExpiredSessions()
	return nil
}

// ReadSessionCookie
// ----------------------------------------------------------------------
// 从 Cookie 中读取 session token
func ReadSessionCookie(c *gin.Context) string {
	val, err := c.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

// WriteSessionCookie
// ----------------------------------------------------------------------
// 把 session token 写入浏览器 Cookie
func WriteSessionCookie(c *gin.Context, token string) {
	maxAge := sessionExpireHours * 3600
	c.SetCookie(sessionCookieName, token, maxAge, "/", "", false, true)
}

// ClearSessionCookie
// ----------------------------------------------------------------------
// 清理浏览器 Cookie
func ClearSessionCookie(c *gin.Context) {
	c.SetCookie(sessionCookieName, "", -1, "/", "", false, true)
}

// EncryptText
// ----------------------------------------------------------------------
// 保留旧函数名，兼容旧代码。
// 当前真实密码加解密由 MySQL fixed_aes_encrypt / fixed_aes_decrypt 处理。
func EncryptText(plainText string) (string, error) {
	if strings.TrimSpace(plainText) == "" {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString([]byte(plainText)), nil
}

// DecryptText
// ----------------------------------------------------------------------
// 保留旧函数名，兼容旧代码。
func DecryptText(cipherText string) (string, error) {
	if strings.TrimSpace(cipherText) == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// Login
// ----------------------------------------------------------------------
// 用户登录。
//
// 流程：
// 1. 查询平台用户
// 2. 使用 fixed_aes_decrypt 解密数据库中的密码密文
// 3. 校验密码
// 4. 写入 session
func Login(username string, password string) (SessionUser, string, error) {
	db, err := getAuthDB()
	if err != nil {
		return SessionUser{}, "", err
	}

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if username == "" || password == "" {
		return SessionUser{}, "", errors.New("用户名和密码不能为空")
	}

	var user SessionUser
	var storedPassword string
	var passwordCipher string
	var isEnabled int

	query := `
SELECT
    id,
    username,
    display_name,
    role_name,
    password_cipher,
    fixed_aes_decrypt(password_cipher) AS plain_password,
    is_enabled,
    can_query_data,
    can_query_plan,
    need_change_pwd
FROM platform_user
WHERE username = ? AND is_deleted = 0
LIMIT 1
`

	err = db.QueryRow(query, username).Scan(
		&user.UserID,
		&user.Username,
		&user.DisplayName,
		&user.RoleName,
		&passwordCipher,
		&storedPassword,
		&isEnabled,
		&user.CanQueryData,
		&user.CanQueryPlan,
		&user.NeedChangePwd,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SessionUser{}, "", errors.New("用户名或密码错误")
		}
		return SessionUser{}, "", err
	}

	if isEnabled != 1 {
		return SessionUser{}, "", errors.New("当前用户已被禁用")
	}

	log.Printf("[login] username=%s", username)
	log.Printf("[login] password_cipher=%q", passwordCipher)
	log.Printf("[login] password_plain=%q", storedPassword)

	if strings.TrimSpace(storedPassword) == "" {
		return SessionUser{}, "", errors.New("用户密码密文解密失败")
	}

	if storedPassword != password {
		return SessionUser{}, "", errors.New("用户名或密码错误")
	}

	token, err := generateSessionToken()
	if err != nil {
		return SessionUser{}, "", err
	}

	expireTime := time.Now().Add(time.Duration(sessionExpireHours) * time.Hour)

	_, err = db.Exec(`
INSERT INTO platform_session (session_token, user_id, username, expire_time)
VALUES (?, ?, ?, ?)
`, token, user.UserID, user.Username, expireTime)
	if err != nil {
		return SessionUser{}, "", err
	}

	_ = DeleteExpiredSessions()
	return user, token, nil
}

// LoginByUsername
// ----------------------------------------------------------------------
// 仅通过用户名登录（用于 SSO/第三方认证）
func LoginByUsername(username string) (SessionUser, string, error) {
	db, err := getAuthDB()
	if err != nil {
		return SessionUser{}, "", err
	}

	username = strings.TrimSpace(username)
	if username == "" {
		return SessionUser{}, "", errors.New("用户名不能为空")
	}

	var user SessionUser
	var isEnabled int

	query := `
SELECT
    id,
    username,
    display_name,
    role_name,
    is_enabled,
    can_query_data,
    can_query_plan,
    need_change_pwd
FROM platform_user
WHERE username = ? AND is_deleted = 0
LIMIT 1
`

	err = db.QueryRow(query, username).Scan(
		&user.UserID,
		&user.Username,
		&user.DisplayName,
		&user.RoleName,
		&isEnabled,
		&user.CanQueryData,
		&user.CanQueryPlan,
		&user.NeedChangePwd,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// 用户不存在，自动创建（SSO 首次登录自动注册）
			log.Printf("[SSO] 用户 %s 不存在，自动创建", username)
			_, createErr := db.Exec(`
INSERT INTO platform_user (username, password_cipher, display_name, role_name, is_enabled, can_query_data, can_query_plan, need_change_pwd, is_deleted)
VALUES (?, '', ?, 'user', 1, 1, 1, 0, 0)
`, username, username)
			if createErr != nil {
				return SessionUser{}, "", fmt.Errorf("自动创建用户失败：%v", createErr)
			}

			// 重新查询刚创建的用户
			err = db.QueryRow(query, username).Scan(
				&user.UserID,
				&user.Username,
				&user.DisplayName,
				&user.RoleName,
				&isEnabled,
				&user.CanQueryData,
				&user.CanQueryPlan,
				&user.NeedChangePwd,
			)
			if err != nil {
				return SessionUser{}, "", fmt.Errorf("创建用户后查询失败：%v", err)
			}
		} else {
			return SessionUser{}, "", err
		}
	}

	if isEnabled != 1 {
		return SessionUser{}, "", errors.New("当前用户已被禁用")
	}

	token, err := generateSessionToken()
	if err != nil {
		return SessionUser{}, "", err
	}

	expireTime := time.Now().Add(time.Duration(sessionExpireHours) * time.Hour)

	_, err = db.Exec(`
INSERT INTO platform_session (session_token, user_id, username, expire_time)
VALUES (?, ?, ?, ?)
`, token, user.UserID, user.Username, expireTime)
	if err != nil {
		return SessionUser{}, "", err
	}

	_ = DeleteExpiredSessions()
	return user, token, nil
}

// GetUserBySessionToken
// ----------------------------------------------------------------------
// 根据 session token 获取当前登录用户
func GetUserBySessionToken(token string) (SessionUser, bool, error) {
	db, err := getAuthDB()
	if err != nil {
		return SessionUser{}, false, err
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return SessionUser{}, false, nil
	}

	query := `
SELECT u.id, u.username, u.display_name, u.role_name, u.can_query_data, u.can_query_plan, u.need_change_pwd
FROM platform_session s
INNER JOIN platform_user u ON s.user_id = u.id
WHERE s.session_token = ?
  AND s.expire_time > NOW()
  AND u.is_enabled = 1
  AND u.is_deleted = 0
LIMIT 1
`

	var user SessionUser
	err = db.QueryRow(query, token).Scan(
		&user.UserID,
		&user.Username,
		&user.DisplayName,
		&user.RoleName,
		&user.CanQueryData,
		&user.CanQueryPlan,
		&user.NeedChangePwd,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SessionUser{}, false, nil
		}
		return SessionUser{}, false, err
	}

	return user, true, nil
}

// DeleteSessionByToken
// ----------------------------------------------------------------------
// 删除指定 session
func DeleteSessionByToken(token string) error {
	db, err := getAuthDB()
	if err != nil {
		return err
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}

	_, err = db.Exec(`DELETE FROM platform_session WHERE session_token = ?`, token)
	return err
}

// DeleteExpiredSessions
// ----------------------------------------------------------------------
// 删除所有过期 session
func DeleteExpiredSessions() error {
	db, err := getAuthDB()
	if err != nil {
		return err
	}

	_, err = db.Exec(`DELETE FROM platform_session WHERE expire_time <= NOW()`)
	return err
}

// LoadEnabledConnections
// ----------------------------------------------------------------------
// 获取全部启用的数据库连接。
// admin 默认看到全部启用连接。
func LoadEnabledConnections() ([]DBConnectionRecord, error) {
	db, err := getAuthDB()
	if err != nil {
		return nil, err
	}

	query := `
SELECT id, name, db_type, host, port, username, password_cipher, database_name, service_name, is_enabled, can_connect
FROM platform_db_connection
WHERE is_enabled = 1
ORDER BY db_type, name
`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DBConnectionRecord, 0)
	for rows.Next() {
		var item DBConnectionRecord
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.DBType,
			&item.Host,
			&item.Port,
			&item.Username,
			&item.PasswordCipher,
			&item.DatabaseName,
			&item.ServiceName,
			&item.IsEnabled,
			&item.CanConnect,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// LoadEnabledConnectionsForUser
// ----------------------------------------------------------------------
// 根据用户权限返回其可见连接列表。
//
// 规则：
// 1. admin 返回全部启用连接
// 2. user 只返回分配给自己的连接
func LoadEnabledConnectionsForUser(userID int64, roleName string) ([]DBConnectionRecord, error) {
	if strings.EqualFold(strings.TrimSpace(roleName), "admin") {
		return LoadEnabledConnections()
	}

	db, err := getAuthDB()
	if err != nil {
		return nil, err
	}

	query := `
SELECT c.id, c.name, c.db_type, c.host, c.port, c.username, c.password_cipher, c.database_name, c.service_name, c.is_enabled
FROM platform_db_connection c
INNER JOIN platform_user_db_connection uc
        ON c.name = uc.connection_name
WHERE uc.user_id = ?
  AND c.is_enabled = 1
ORDER BY c.db_type, c.name
`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DBConnectionRecord, 0)
	for rows.Next() {
		var item DBConnectionRecord
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.DBType,
			&item.Host,
			&item.Port,
			&item.Username,
			&item.PasswordCipher,
			&item.DatabaseName,
			&item.ServiceName,
			&item.IsEnabled,
			&item.CanConnect,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// LoadConnectionByName
// ----------------------------------------------------------------------
// 根据连接名称读取单个启用连接
func LoadConnectionByName(name string) (DBConnectionRecord, error) {
	db, err := getAuthDB()
	if err != nil {
		return DBConnectionRecord{}, err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return DBConnectionRecord{}, errors.New("连接名称不能为空")
	}

	query := `
SELECT id, name, db_type, host, port, username, password_cipher, database_name, service_name, is_enabled, can_connect
FROM platform_db_connection
WHERE name = ? AND is_enabled = 1
LIMIT 1
`

	var item DBConnectionRecord
	err = db.QueryRow(query, name).Scan(
		&item.ID,
		&item.Name,
		&item.DBType,
		&item.Host,
		&item.Port,
		&item.Username,
		&item.PasswordCipher,
		&item.DatabaseName,
		&item.ServiceName,
		&item.IsEnabled,
		&item.CanConnect,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DBConnectionRecord{}, errors.New("未找到连接配置：" + name)
		}
		return DBConnectionRecord{}, err
	}

	return item, nil
}

// GetConnectionPlainPassword
// ----------------------------------------------------------------------
// 解密数据库连接密码
func GetConnectionPlainPassword(record DBConnectionRecord) (string, error) {
	db, err := getAuthDB()
	if err != nil {
		return "", err
	}

	var plainPassword sql.NullString
	err = db.QueryRow(`SELECT fixed_aes_decrypt(?)`, record.PasswordCipher).Scan(&plainPassword)
	if err != nil {
		return "", err
	}

	if !plainPassword.Valid {
		return "", errors.New("数据库连接密码密文解密失败")
	}

	return plainPassword.String, nil
}

// NormalizeConnectionNames
// ----------------------------------------------------------------------
// 归一化连接名称数组：去空格、去重、去空值
func NormalizeConnectionNames(names []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)

	for _, name := range names {
		v := strings.TrimSpace(name)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}

// ValidateEnabledConnectionNames
// ----------------------------------------------------------------------
// 校验一批连接名称是否都存在且启用
func ValidateEnabledConnectionNames(names []string) error {
	normalized := NormalizeConnectionNames(names)
	if len(normalized) == 0 {
		return nil
	}

	db, err := getAuthDB()
	if err != nil {
		return err
	}

	for _, name := range normalized {
		var count int
		err := db.QueryRow(`
SELECT COUNT(1)
FROM platform_db_connection
WHERE name = ? AND is_enabled = 1
`, name).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			return errors.New("连接不存在或未启用：" + name)
		}
	}

	return nil
}

// DeleteUserAllowedConnections
// ----------------------------------------------------------------------
// 删除某用户全部连接权限
func DeleteUserAllowedConnections(userID int64) error {
	db, err := getAuthDB()
	if err != nil {
		return err
	}

	_, err = db.Exec(`DELETE FROM platform_user_db_connection WHERE user_id = ?`, userID)
	return err
}

// SaveUserAllowedConnections
// ----------------------------------------------------------------------
// 保存用户可访问连接（先删后插）
func SaveUserAllowedConnections(userID int64, connectionNames []string) error {
	db, err := getAuthDB()
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := SaveUserAllowedConnectionsTx(tx, userID, connectionNames); err != nil {
		return err
	}

	return tx.Commit()
}

// SaveUserAllowedConnectionsTx
// ----------------------------------------------------------------------
// 在事务中保存用户连接权限
func SaveUserAllowedConnectionsTx(tx *sql.Tx, userID int64, connectionNames []string) error {
	normalized := NormalizeConnectionNames(connectionNames)

	if _, err := tx.Exec(`DELETE FROM platform_user_db_connection WHERE user_id = ?`, userID); err != nil {
		return err
	}

	for _, name := range normalized {
		if _, err := tx.Exec(`
INSERT INTO platform_user_db_connection (user_id, connection_name)
VALUES (?, ?)
`, userID, name); err != nil {
			return err
		}
	}

	return nil
}

// ListUserAllowedConnectionNames
// ----------------------------------------------------------------------
// 获取某用户被分配的连接名称列表
func ListUserAllowedConnectionNames(userID int64) ([]string, error) {
	db, err := getAuthDB()
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
SELECT connection_name
FROM platform_user_db_connection
WHERE user_id = ?
ORDER BY connection_name
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		items = append(items, name)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// UserCanAccessConnection
// ----------------------------------------------------------------------
// 判断用户是否有权访问某个连接
func UserCanAccessConnection(userID int64, roleName string, connectionName string) (bool, error) {
	connName := strings.TrimSpace(connectionName)
	if connName == "" {
		return false, errors.New("连接名称不能为空")
	}

	if _, err := LoadConnectionByName(connName); err != nil {
		return false, err
	}

	if strings.EqualFold(strings.TrimSpace(roleName), "admin") {
		return true, nil
	}

	db, err := getAuthDB()
	if err != nil {
		return false, err
	}

	var count int
	err = db.QueryRow(`
SELECT COUNT(1)
FROM platform_user_db_connection
WHERE user_id = ? AND connection_name = ?
`, userID, connName).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GenerateSQLDigest 生成 SQL 的结构化摘要哈希
func GenerateSQLDigest(sql string) string {
	// 1. 转小写并去除首尾空格
	s := strings.ToLower(strings.TrimSpace(sql))

	// 2. 替换字符串常量 (例如 'abc' -> ?)
	reStr := regexp.MustCompile(`'(?:''|[^'])*'`)
	s = reStr.ReplaceAllString(s, "?")

	// 3. 替换数值常量 (例如 123, 0.45 -> ?)
	reNum := regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	s = reNum.ReplaceAllString(s, "?")

	// 4. 合并多个连续空格
	reSpace := regexp.MustCompile(`\s+`)
	s = reSpace.ReplaceAllString(s, " ")

	// 5. 计算 MD5 哈希值，方便索引查询
	hash := md5.Sum([]byte(s))
	return hex.EncodeToString(hash[:])
}

// SQL 审核记录模型
type SqlAuditRecord struct {
	UserID         int64  `json:"userId"`
	ConnectionName string `json:"connectionName"`
	SqlText        string `json:"sqlText"`
	SqlDigest      string `json:"sqlDigest"`
	ExecutionPlan  string `json:"executionPlan"`
	AiSuggestion   string `json:"aiSuggestion"`
	AiScore        int    `json:"aiScore"`
	SubmitAudit    int    `json:"submitAudit"`
	AuditPassed    int    `json:"auditPassed"`
	Remark         string `json:"remark"`
}

// SaveSqlAuditRecord
// ----------------------------------------------------------------------
// 将 AI 审核记录持久化到平台库
func SaveSqlAuditRecord(record SqlAuditRecord) (int64, error) {
	db, err := getAuthDB()
	if err != nil {
		return 0, err
	}

	query := `
INSERT INTO platform_sql_audit (user_id, connection_name, sql_text, sql_digest, execution_plan, ai_suggestion, ai_score, submit_audit, audit_passed, remark)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
	res, err := db.Exec(query,
		record.UserID,
		record.ConnectionName,
		record.SqlText,
		record.SqlDigest,
		record.ExecutionPlan,
		record.AiSuggestion,
		record.AiScore,
		record.SubmitAudit,
		record.AuditPassed,
		record.Remark,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

type SqlAuditHistoryRecord struct {
	ID             int64  `json:"id"`
	ConnectionName string `json:"connectionName"`
	SqlText        string `json:"sqlText"`
	ExecutionPlan  string `json:"executionPlan"`
	AiSuggestion   string `json:"aiSuggestion"`
	AiScore        int    `json:"aiScore"`
	SubmitAudit    int    `json:"submitAudit"`
	AuditPassed    int    `json:"auditPassed"`
	Reviewer       string `json:"reviewer"`
	Remark         string `json:"remark"`
	CreateTime     string `json:"createTime"`
}

// GetSqlAuditHistoryByUserID
// ----------------------------------------------------------------------
// 获取当前用户的所有审核历史记录，按时间倒序
func GetSqlAuditHistoryByUserID(userID int64, digest string, page int, pageSize int) (int64, []SqlAuditHistoryRecord, error) {
	db, err := getAuthDB()
	if err != nil {
		return 0, nil, err
	}

	countQuery := `SELECT COUNT(*) FROM platform_sql_audit WHERE user_id = ?`
	query := `
SELECT id, connection_name, sql_text, execution_plan, ai_suggestion, ai_score, submit_audit, audit_passed, reviewer, remark, create_time
FROM platform_sql_audit
WHERE user_id = ?
`
	args := []interface{}{userID}

	if digest != "" {
		countQuery += " AND sql_digest = ?"
		query += " AND sql_digest = ?"
		args = append(args, digest)
	}

	var total int64
	err = db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return 0, nil, err
	}

	query += " ORDER BY create_time DESC LIMIT ? OFFSET ?"
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var records []SqlAuditHistoryRecord
	for rows.Next() {
		var rec SqlAuditHistoryRecord
		var executionPlan sql.NullString
		if err := rows.Scan(&rec.ID, &rec.ConnectionName, &rec.SqlText, &executionPlan, &rec.AiSuggestion, &rec.AiScore, &rec.SubmitAudit, &rec.AuditPassed, &rec.Reviewer, &rec.Remark, &rec.CreateTime); err != nil {
			return 0, nil, err
		}
		if executionPlan.Valid {
			rec.ExecutionPlan = executionPlan.String
		}
		records = append(records, rec)
	}

	return total, records, nil
}

// SubmitSqlAudit
// ----------------------------------------------------------------------
// 提交 SQL 审核
func SubmitSqlAudit(auditID int64, userID int64, submitAudit int, remark string) error {
	db, err := getAuthDB()
	if err != nil {
		return err
	}

	query := `
UPDATE platform_sql_audit 
SET submit_audit = ?, remark = ?
WHERE id = ? AND user_id = ?
`
	result, err := db.Exec(query, submitAudit, remark, auditID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("审核记录不存在或无权限修改")
	}

	return nil
}

type AdminSqlAuditRecord struct {
	ID             int64  `json:"id"`
	UserID         int64  `json:"userId"`
	Username       string `json:"username"`
	DisplayName    string `json:"displayName"`
	ConnectionName string `json:"connectionName"`
	SqlText        string `json:"sqlText"`
	ExecutionPlan  string `json:"executionPlan"`
	AiSuggestion   string `json:"aiSuggestion"`
	AiScore        int    `json:"aiScore"`
	SubmitAudit    int    `json:"submitAudit"`
	AuditPassed    int    `json:"auditPassed"`
	Reviewer       string `json:"reviewer"`
	Remark         string `json:"remark"`
	CreateTime     string `json:"createTime"`
}

// AdminListSubmittedAudits
// ----------------------------------------------------------------------
// 管理员获取所有已提交的审核记录
func AdminListSubmittedAudits(page int, pageSize int, status string, submitter string, connection string) (int64, []AdminSqlAuditRecord, error) {
	db, err := getAuthDB()
	if err != nil {
		return 0, nil, err
	}

	baseWhere := "WHERE a.submit_audit > 0"
	var args []interface{}

	switch status {
	case "pending":
		baseWhere += " AND a.audit_passed = 0"
	case "passed":
		baseWhere += " AND a.audit_passed = 1"
	case "rejected":
		baseWhere += " AND a.audit_passed = -1"
	}

	if submitter != "" {
		baseWhere += " AND (u.username LIKE ? OR u.display_name LIKE ?)"
		likeSubmitter := "%" + submitter + "%"
		args = append(args, likeSubmitter, likeSubmitter)
	}

	if connection != "" {
		baseWhere += " AND a.connection_name LIKE ?"
		args = append(args, "%"+connection+"%")
	}

	countQuery := `
SELECT COUNT(*) 
FROM platform_sql_audit a
LEFT JOIN platform_user u ON a.user_id = u.id
` + baseWhere

	var total int64
	err = db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return 0, nil, err
	}

	query := `
SELECT a.id, a.user_id, u.username, u.display_name, a.connection_name, a.sql_text,
       a.execution_plan, a.ai_suggestion, a.ai_score, a.submit_audit, a.audit_passed, a.reviewer, a.remark, a.create_time
FROM platform_sql_audit a
LEFT JOIN platform_user u ON a.user_id = u.id
` + baseWhere + `
ORDER BY a.create_time DESC LIMIT ? OFFSET ?
`
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var records []AdminSqlAuditRecord
	for rows.Next() {
		var rec AdminSqlAuditRecord
		var displayName, executionPlan sql.NullString
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.Username, &displayName, &rec.ConnectionName, &rec.SqlText,
			&executionPlan, &rec.AiSuggestion, &rec.AiScore, &rec.SubmitAudit, &rec.AuditPassed, &rec.Reviewer, &rec.Remark, &rec.CreateTime); err != nil {
			return 0, nil, err
		}
		if displayName.Valid {
			rec.DisplayName = displayName.String
		}
		if executionPlan.Valid {
			rec.ExecutionPlan = executionPlan.String
		}
		records = append(records, rec)
	}

	return total, records, nil
}

// AdminReviewAudit
// ----------------------------------------------------------------------
// 管理员审核通过/驳回
func AdminReviewAudit(auditID int64, auditPassed int, reviewer string) error {
	db, err := getAuthDB()
	if err != nil {
		return err
	}

	query := `UPDATE platform_sql_audit SET audit_passed = ?, reviewer = ? WHERE id = ? AND submit_audit > 0`
	result, err := db.Exec(query, auditPassed, reviewer, auditID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("审核记录不存在或未提交审核")
	}

	return nil
}

// BuildPasswordCipher
// ----------------------------------------------------------------------
// 保留旧函数名，兼容旧代码
func BuildPasswordCipher(plainText string) (string, error) {
	return EncryptText(plainText)
}

// generateSessionToken
// ----------------------------------------------------------------------
// 生成随机 session token
func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
