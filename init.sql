CREATE DATABASE IF NOT EXISTS db_platform
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_general_ci;

USE db_platform;

DELIMITER $$

CREATE FUNCTION fixed_aes_encrypt(p_plain_text TEXT)
RETURNS TEXT
DETERMINISTIC
BEGIN
    DECLARE v_key VARCHAR(64) DEFAULT 'UyvmefM8WRt6RdLlZUtg1IO9FOreTcT';

    IF p_plain_text IS NULL OR CHAR_LENGTH(TRIM(p_plain_text)) = 0 THEN
        RETURN '';
    END IF;

    RETURN HEX(AES_ENCRYPT(p_plain_text, v_key));
END$$

CREATE FUNCTION fixed_aes_decrypt(p_cipher_hex TEXT)
RETURNS TEXT
DETERMINISTIC
BEGIN
    DECLARE v_key VARCHAR(64) DEFAULT 'UyvmefM8WRt6RdLlZUtg1IO9FOreTcT';
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

CREATE TABLE IF NOT EXISTS `user` (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    username VARCHAR(64) NOT NULL DEFAULT '' COMMENT '登录用户名，唯一',
    password_cipher VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'fixed_aes_encrypt 加密后的密码',
    display_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '显示名称',
    role_name VARCHAR(64) NOT NULL DEFAULT 'user' COMMENT '角色：admin/user',
    is_enabled TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：1启用，0禁用',
    can_query_data TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许访问查询功能：1允许，0禁止',
    need_change_pwd TINYINT NOT NULL DEFAULT 1 COMMENT '首次登录是否需要修改密码：1是，0否',
    is_deleted TINYINT NOT NULL DEFAULT 0 COMMENT '是否删除：1删除，0正常',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

CREATE TABLE IF NOT EXISTS `session` (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    session_token VARCHAR(128) NOT NULL DEFAULT '' COMMENT '登录会话令牌',
    user_id BIGINT NOT NULL DEFAULT 0 COMMENT '用户ID',
    username VARCHAR(64) NOT NULL DEFAULT '' COMMENT '用户名',
    expire_time DATETIME NOT NULL COMMENT '会话过期时间',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_session_token (session_token),
    KEY idx_session_user_id (user_id),
    KEY idx_session_expire_time (expire_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='登录会话表';

CREATE TABLE IF NOT EXISTS db_connection (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '连接唯一名称',
    db_type VARCHAR(16) NOT NULL DEFAULT '' COMMENT '数据库类型：mysql/oracle/postgres/mssql',
    host VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库主机',
    port INT NOT NULL DEFAULT 0 COMMENT '数据库端口',
    username VARCHAR(128) NOT NULL DEFAULT '' COMMENT '数据库账号',
    password_cipher VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'fixed_aes_encrypt 加密后的数据库密码',
    database_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'MySQL/PostgreSQL/MSSQL 数据库名 / Oracle schema 名',
    service_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Oracle 服务名 / PostgreSQL schema 名',
    is_enabled TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：1启用，0禁用',
    can_connect TINYINT NOT NULL DEFAULT 0 COMMENT '是否可连接：1可连接，0不可连接',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_db_connection_name (name),
    KEY idx_db_connection_db_type (db_type),
    KEY idx_db_connection_is_enabled (is_enabled),
    KEY idx_db_connection_can_connect (can_connect)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库连接配置表';

CREATE TABLE IF NOT EXISTS user_db_connection (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    user_id BIGINT NOT NULL COMMENT '用户ID',
    connection_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '连接名称，对应 db_connection.name',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_db_connection (user_id, connection_name),
    KEY idx_user_db_connection_user_id (user_id),
    KEY idx_user_db_connection_connection_name (connection_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户可访问数据库连接关系表';

CREATE TABLE IF NOT EXISTS sql_favorite (
    id BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
    user_id BIGINT NOT NULL COMMENT '所属用户ID',
    favorite_name VARCHAR(128) NOT NULL DEFAULT '' COMMENT '收藏名称',
    sql_text LONGTEXT NOT NULL COMMENT 'SQL 内容',
    db_type VARCHAR(16) NOT NULL DEFAULT '' COMMENT '数据库类型：mysql/oracle/postgres/mssql',
    connection_name VARCHAR(64) NOT NULL DEFAULT '' COMMENT '关联连接名称，可为空',
    remark VARCHAR(500) NOT NULL DEFAULT '' COMMENT '备注',
    is_pinned TINYINT NOT NULL DEFAULT 0 COMMENT '是否置顶：1是，0否',
    create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_sql_favorite_user_id (user_id),
    KEY idx_sql_favorite_connection_name (connection_name),
    KEY idx_sql_favorite_db_type (db_type),
    KEY idx_sql_favorite_is_pinned (is_pinned)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='SQL 收藏表';

INSERT INTO `user` (
    username,
    password_cipher,
    display_name,
    role_name,
    is_enabled,
    can_query_data,
    need_change_pwd,
    is_deleted
)
SELECT
    'admin',
    fixed_aes_encrypt('Admin'),
    '系统管理员',
    'admin',
    1,
    1,
    0,
    0
FROM DUAL
WHERE NOT EXISTS (
    SELECT 1 FROM `user` WHERE username = 'admin' AND is_deleted = 0
);
