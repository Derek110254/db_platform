-- =========================================================
-- 1. 创建数据库
-- =========================================================
CREATE DATABASE IF NOT EXISTS db_platform
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_general_ci;

USE db_platform;


-- =========================================================
-- 2. 删除旧函数（如果已存在）
-- =========================================================
DROP FUNCTION IF EXISTS fixed_aes_encrypt;
DROP FUNCTION IF EXISTS fixed_aes_decrypt;


-- =========================================================
-- 3. 创建固定密钥加密函数
-- ---------------------------------------------------------
-- 说明：
-- 1. 你的 Go 代码里调用方式是：
--      fixed_aes_encrypt(?)
--      fixed_aes_decrypt(?)
--    只有 1 个参数，所以这里函数也必须只有 1 个参数。
-- 2. 固定密钥与代码中的 fixedSecretKey 保持一致：
--      db_platform_fixed_secret_key_32
-- 3. 存储格式：
--      加密后统一转成 HEX 字符串保存，避免二进制乱码问题
-- =========================================================
DELIMITER $$

CREATE FUNCTION fixed_aes_encrypt(p_plain_text TEXT)
RETURNS TEXT
DETERMINISTIC
BEGIN
    DECLARE v_key VARCHAR(64) DEFAULT 'db_platform_fixed_secret_key_32';

    IF p_plain_text IS NULL OR CHAR_LENGTH(TRIM(p_plain_text)) = 0 THEN
        RETURN '';
    END IF;

    RETURN HEX(AES_ENCRYPT(p_plain_text, v_key));
END$$

CREATE FUNCTION fixed_aes_decrypt(p_cipher_hex TEXT)
RETURNS TEXT
DETERMINISTIC
BEGIN
    DECLARE v_key VARCHAR(64) DEFAULT 'db_platform_fixed_secret_key_32';
    DECLARE v_result TEXT;

    IF p_cipher_hex IS NULL OR CHAR_LENGTH(TRIM(p_cipher_hex)) = 0 THEN
        RETURN '';
    END IF;

    SET v_result = CAST(AES_DECRYPT(UNHEX(p_cipher_hex), v_key) AS CHAR(255) CHARACTER SET utf8mb4);

    IF v_result IS NULL THEN
        RETURN '';
    END IF;

    RETURN v_result;
END$$

DELIMITER ;


-- =========================================================
-- 4. 创建平台登录用户表
-- ---------------------------------------------------------
-- 对应代码中的 platform_user 表
-- =========================================================
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


-- =========================================================
-- 5. 创建登录会话表
-- ---------------------------------------------------------
-- 对应代码中的 platform_session 表
-- =========================================================
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


-- =========================================================
-- 6. 创建数据库连接配置表
-- ---------------------------------------------------------
-- 对应代码中的 platform_db_connection 表
-- 密码字段保存 fixed_aes_encrypt 后的密文
-- =========================================================
CREATE TABLE IF NOT EXISTS platform_db_connection (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '连接唯一名称，例如 mysql-dev',
    label VARCHAR(128) NOT NULL DEFAULT '' COMMENT '前端展示名称',
    db_type VARCHAR(16) NOT NULL DEFAULT '' COMMENT '数据库类型：mysql/oracle',
    host VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库主机',
    port INT NOT NULL DEFAULT 0 COMMENT '数据库端口',
    username VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库账号',
    password_cipher VARCHAR(512) NOT NULL DEFAULT '' COMMENT '使用 fixed_aes_encrypt 加密后的数据库密码密文',
    database_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'MySQL 数据库名',
    service_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Oracle 服务名',
    is_enabled TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：1启用，0禁用',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_platform_db_connection_name (name),
    KEY idx_platform_db_connection_db_type (db_type),
    KEY idx_platform_db_connection_is_enabled (is_enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台数据库连接配置表';


-- =========================================================
-- 7. 创建用户可访问数据库连接关系表
-- ---------------------------------------------------------
-- 对应代码中的 platform_user_db_connection 表
-- 普通用户必须通过这张表分配连接权限
-- admin 默认不依赖这张表
-- =========================================================
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


-- =========================================================
-- 8. 创建 SQL 收藏表
-- ---------------------------------------------------------
-- 对应代码中的 platform_sql_favorite 表
-- =========================================================
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


-- =========================================================
-- 8. 创建 SQL AI审核记录表
-- ---------------------------------------------------------
-- 对应代码中的 platform_sql_audit 表
-- =========================================================
CREATE TABLE IF NOT EXISTS platform_sql_audit (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    user_id BIGINT NOT NULL COMMENT '提交用户ID',
    connection_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '数据库连接名称',
    sql_text LONGTEXT NOT NULL COMMENT '提交审核的SQL内容',
	sql_digest VARCHAR(64) NOT NULL COMMENT 'SQL结构指纹哈希',
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

-- =========================================================
-- 9. 插入管理员用户
-- ---------------------------------------------------------
-- 管理员账号：
--   admin
-- 管理员密码：
--   Admin@123
-- =========================================================
INSERT INTO platform_user (
    username,
    password_cipher,
    display_name,
    role_name,
    is_enabled
)
SELECT
    'admin',
    fixed_aes_encrypt('Admin@123'),
    '系统管理员',
    'admin',
    1
FROM DUAL
WHERE NOT EXISTS (
    SELECT 1 FROM platform_user WHERE username = 'admin' AND is_deleted = 0
);

-- =========================================================
-- 10. 创建数据库变更申请表
-- ---------------------------------------------------------
-- =========================================================
CREATE TABLE IF NOT EXISTS platform_db_change_request (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    applicant VARCHAR(64) NOT NULL DEFAULT '' COMMENT '申请人',
    applicant_team VARCHAR(128) NOT NULL DEFAULT '' COMMENT '申请团队',
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
    db_ip VARCHAR(128) NOT NULL DEFAULT '' COMMENT '生产线数据库IP',
    db_name VARCHAR(32) NOT NULL DEFAULT '' COMMENT '生产线数据库名',
    db_schema VARCHAR(128) NOT NULL DEFAULT '' COMMENT '生产线数据库schema',
    change_content LONGTEXT COMMENT '变更内容',
    release_verifier VARCHAR(64) NOT NULL DEFAULT '' COMMENT '发布验证人',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_platform_db_change_request_applicant (applicant),
    KEY idx_platform_db_change_request_create_time (create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库变更申请表';