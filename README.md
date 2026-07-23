# sql_platform SQL管理平台

基于 Go (Gin) 和 Vue 3 构建的轻量级、安全且易于使用的 SQL 管理与审计平台。提供 SQL 风险检测、查询执行、执行计划分析、数据库变更审批、运维变更记录、工作看板统计等功能，内置 SSO 单点登录和细粒度的访问控制。

## 核心功能

### SQL 检测与格式化

- **DML 风险静态检测：** 在执行前对 SQL 语句进行风险规则匹配分析（缺少 WHERE、全表扫描等），对高风险操作预警。
- **DDL 规范检查：** 对 DDL 语句进行规范性检测，确保建表、修改表等操作符合标准。
- **SQL 格式化：** 内置 SQL 格式化工具，一键美化。
- **SQL/DDL 文件上传分析：** 支持上传 SQL 文件进行批量检测。

### 安全查询

- **多数据库支持：** MySQL 和 Oracle。
- **严格只读控制：** 仅允许 `SELECT`/`WITH`，代码层面拦截所有写操作。
- **行数限制：** 数据库执行层限制最多返回 500 行。
- **执行计划解读 (EXPLAIN)：** 集成 EXPLAIN 命令，支持 AI 智能解读（DML/DQL 均可检测，DDL 不支持）。
- **手动提交执行计划：** 对于网络不可达的数据库（`can_connect=0`），用户可手动粘贴 EXPLAIN 输出进行 AI 分析。
- **表/列元数据浏览：** 实时关键字过滤表和字段，点击快捷插入 SQL 编辑器。
- **查询历史：** 侧边栏展示历史记录，支持回填和删除。
- **数据导出：** 导出为 Excel (.xlsx)，解决长数字科学计数法失真。
- **SQL 收藏夹：** 保存常用 SQL，支持新增（仅表单）和查看列表（使用/编辑/删除）两种模式。

### 数据库变更管理

- **变更申请：** 用户提交变更申请（团队、环境、变更类型、变更内容、备份表、回滚方案等）。
- **变更审批流程：** 创建 → 待复核 → 复核人复核 → 待变更 → 确认变更结果(成功/失败) → 失败则确认回滚 → 彻底完结。
- **已完结保护：** 变更结果确认后不可编辑、不可删除。
- **SQL 审核提交：** 用户提交 SQL 进行 AI 评分，管理员人工二次复核，审核记录包含执行计划。
- **审核历史：** 用户查看自己的审核记录及审批状态。

### 数据同步管理

- **同步申请：** 用户提交数据同步申请（源/目标库、过滤条件、预估数据量、敏感数据规则等）。
- **同步审批：** 管理员审批，指定 DBA 执行人。

### 运维变更记录

- **变更登记：** 记录运维变更的完整信息（标题、类型、等级、内容、影响范围、变更IP、回滚方案等）。
- **流程化管理：** 待复核 → 复核人确认 → 待变更 → 确认变更结果 → 失败则确认回滚 → 完结。
- **9 种变更类型：** 安装部署、配置变更、服务重启、版本升级、数据修复、性能优化、容量变更、应急变更、其他。

### 数据库告警处理

- **告警登记：** 记录告警内容、影响范围、处理过程与结果。
- **7 种告警分类：** SQL性能、空间扩容、配置优化、可用性故障、锁与阻塞、备份恢复、硬件不足。
- **三态列表：** 列表/查看/登记三个视图，双击行直接进入查看。

### 工作看板与统计

- **年度工作看板：** 展示选定年度的统计卡片（SQL 审核、变更申请、数据同步、告警处理、运维变更等数量）、月度趋势、各分类分布和工作量排行榜。
- **个人看板（管理员）：** 展示当前用户的工作量卡片（作为申请人/处理人/操作人/审核人）和待办事项，双击待办卡片可快速跳转到对应管理页面。

### 团队数据库环境

- **环境配置：** 按团队配置数据库环境映射（测试线/生产线 IP、库名、Schema）。
- **快速引用：** 用户提交变更申请时可直接选择环境自动填入。

### 认证与安全

- **SSO 工作证认证：** 集成企业 SSO，RSA 加密 + SSO API 验证全在后端完成，公钥和 API 地址不暴露给前端。
- **自动注册：** SSO 首次登录自动创建用户（role=user，默认开启查询权限）。
- **首次登录强制改密：** 账密登录的新用户首次登录时强制修改密码。
- **操作审计日志：** 记录每一次查询的执行者、目标数据库、SQL 指纹及执行耗时。
- **连接测试：** 保存连接配置时先测 TCP 端口连通性，端口不通可强制保存；密码错误不可保存。修改连接名称时事务级联更新所有关联表。

### 权限与管理

- **角色隔离：** admin / user 角色隔离。
- **连接权限：** 管理员配置连接，按用户授权，普通用户只看自己的。
- **可连接标记：** 连接配置 `can_connect` 字段，不可连接的库在查询页和检测页显示提示、禁用操作按钮。
- **用户管理：** 创建/编辑/删除用户，分配角色和权限。
- **团队环境管理：** 配置团队数据库环境，支持实时搜索。

## 技术栈

### 后端 (Server)
- **语言：** Go 1.20+
- **框架：** Gin
- **数据库驱动：** `go-sql-driver/mysql` (MySQL)、`sijms/go-ora/v2` (Oracle)
- **AI 接口：** OpenAI 兼容的 chat/completions 格式
- **加密：** `crypto/rsa` (SSO token 加密)、MySQL `fixed_aes_encrypt/decrypt` (密码存储)
- **前端嵌入：** Go `embed` 嵌入前端静态资源，编译为单一二进制

### 前端 (Client)
- **框架：** Vue 3 (Composition API, `<script setup>`)
- **语言：** TypeScript
- **构建工具：** Vite (自动清空输出目录)
- **代码编辑器：** CodeMirror 6 (SQL 语法高亮与自动补全)
- **字体：** 全局使用 Consolas/Monaco 等宽字体
- **样式：** 原生 CSS（`global.css` 统一公共样式 + Vue scoped 组件样式），Zabbix 5 风格侧边栏 + 面包屑布局

## 数据库表

| 表名 | 说明 |
|------|------|
| `platform_user` | 平台用户表 |
| `platform_session` | 登录会话表 |
| `platform_db_connection` | 数据库连接配置表 |
| `platform_user_db_connection` | 用户-连接权限关系表 |
| `platform_sql_favorite` | SQL 收藏表 |
| `platform_sql_audit` | SQL AI 审核记录表（含执行计划） |
| `platform_db_change_request` | 数据库变更申请表 |
| `platform_team_db_env` | 团队数据库环境配置表 |
| `platform_db_data_sync_request` | 数据库数据同步申请表 |
| `platform_db_alert_handle` | 数据库告警处理表 |
| `platform_ops_change_record` | 运维变更记录表 |

## 目录结构

```text
sql_platform/
├── client/                          # Vue 3 前端工程
│   ├── src/
│   │   ├── App.vue                  # 主入口（路由/导航/权限控制）
│   │   └── components/
│   │       ├── HomePanel.vue             # 首页（DML/DDL 检测、格式化）
│   │       ├── DashboardPanel.vue       # 工作看板（年度统计）
│   │       ├── QueryPanel.vue            # 查询工作台
│   │       ├── QueryPlanPanel.vue        # 执行计划查询与 AI 解读
│   │       ├── MetadataPanel.vue         # 表/列元数据侧边栏
│   │       ├── QueryHistoryPanel.vue     # 查询历史侧边栏
│   │       ├── SqlFavoritePanel.vue      # SQL 收藏（新增/列表双模式）
│   │       ├── LoginDialog.vue           # SSO 登录（后端代理验证）
│   │       ├── ChangePasswordDialog.vue  # 修改密码
│   │       ├── DbChangeRequestPanel.vue  # 数据库变更申请
│   │       ├── DbDataSyncRequestPanel.vue# 数据同步申请
│   │       ├── AuditHistoryPanel.vue     # 审核历史
│   │       ├── AdminUserPanel.vue        # [管理] 用户管理
│   │       ├── PersonalDashboardPanel.vue # [管理] 个人看板
│   │       ├── AdminConnectionPanel.vue  # [管理] 连接配置
│   │       ├── AdminAuditPanel.vue       # [管理] SQL 审核
│   │       ├── AdminDbChangeReleasePanel.vue # [管理] 变更发布
│   │       ├── AdminDbDataSyncRequestPanel.vue # [管理] 数据同步审批
│   │       ├── AdminTeamDbEnvPanel.vue   # [管理] 团队数据库环境
│   │       ├── AdminDbAlertHandlePanel.vue # [管理] 数据库告警处理
│   │       └── AdminOpsChangePanel.vue   # [管理] 运维变更记录
│   ├── package.json
│   └── vite.config.ts
├── server/                          # Go 后端工程
│   ├── auth/                        # 认证、鉴权、表结构初始化
│   │   ├── session.go               # 登录/会话/用户连接权限映射/表初始化
│   │   └── sso.go                   # SSO 验证（RSA 加密 + API 调用）
│   ├── config/
│   │   └── platform_db.go           # 全局配置（数据库、AI Key、SSO 配置）
│   ├── middleware/
│   │   └── auth.go                  # RequireLogin / RequireAdmin 中间件
│   ├── routes/
│   │   ├── api.go                   # 所有 API 路由与 handler
│   │   └── web.go                   # 前端静态资源托管 (SPA fallback)
│   ├── sql/                         # SQL 核心引擎（按业务域拆分，HTTP handler 在 routes/api.go 委托调用）
│   │   ├── connection.go            # 数据库连接配置管理 CRUD（含改名级联）
│   │   ├── dml_check.go            # SQL 风险检测
│   │   ├── dashboard.go             # 年度工作量看板统计
│   │   ├── db_alert_handle.go       # 告警处理 CRUD
│   │   ├── db_change_request.go     # 变更申请 CRUD + 验证流程
│   │   ├── db_data_sync_request.go  # 数据同步 CRUD
│   │   ├── ddl_checker.go           # DDL 规范检查
│   │   ├── ops_change_record.go     # 运维变更记录 CRUD + 流程
│   │   ├── personal_dashboard.go    # 个人看板统计
│   │   ├── query_executor.go        # 查询执行（行数限制）
│   │   ├── query_metadata.go        # 元数据查询
│   │   ├── query_plan.go            # 执行计划 + AI 解读 + 手动提交
│   │   ├── sql_favorite.go          # SQL 收藏 CRUD
│   │   ├── team_db_env.go           # 团队数据库环境 CRUD
│   │   └── user.go                  # 用户管理 CRUD（密码/连接权限）
│   ├── web/dist/                    # 编译后的前端（embed 嵌入）
│   ├── main.go
│   └── go.mod
├── init.sql                         # 数据库初始化脚本
└── README.md
```

## 本地运行指南

### 环境依赖

- Go 1.20+
- Node.js v20+
- MySQL 5.7+

### 1. 初始化数据库

```bash
mysql -u root -p < init.sql
```

### 2. 配置后端

修改 `server/config/platform_db.go` 中的配置：

```go
const (
    PlatformDBHost = "你的数据库IP"
    PlatformDBPort = 3306
    PlatformDBUser = "db_platform"
    PlatformDBPassword = "你的密码"
    PlatformDBName = "db_platform"

    QwenAPIKey = "你的AI API Key"  // AI 性能分析用

    // SSO 配置
    SSOGzzApiUrl = "http://SSO接口地址/qhgzz/authority/isLogin"
    SSOLoginUrl  = "http://SSO登录页地址"
    SSOPublicKey = "RSA公钥"
    SSOUrlId     = "1577"
)
```

### 3. 编译前端

```bash
cd client
npm install
npm run build
```

Vite 会自动清空输出目录（`emptyOutDir`），编译结果输出到 `server/web/dist`。

### 4. 运行后端

```bash
cd server
go mod tidy
go run main.go
```

默认端口 `2345`，访问 `http://localhost:2345`。

### 默认管理员

- 用户名：`admin`
- 密码：`Admin@123`

## 安全性说明

- **SQL 注入防范：** 正则过滤 DML，安全嵌套策略封堵注释绕过。
- **连接测试：** 保存前先测 TCP 端口（3 秒超时），端口不通可强制保存，密码错误不可保存。
- **敏感信息保护：** 数据库密码加密存储，SSO 公钥/API 地址仅在后端，不暴露给前端。
- **可追溯性：** 所有查询操作记录审计日志，`access.log` 记录 HTTP 请求。
- **已完结保护：** 运维变更记录完结后不可编辑/删除。
