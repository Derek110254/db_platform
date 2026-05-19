# sql_platform 数据库审计管理平台

基于 Go (Gin) 和 Vue 3 构建的轻量级、安全且易于使用的 SQL 数据库管理与审计平台。它提供了一个 Web 界面，用于执行查询、审查执行计划、进行 SQL 风险分析以及将结果导出为 Excel，并内置了严格的访问控制和操作审计功能。

## 核心功能

- **多数据库支持：** 支持连接并查询 MySQL 和 Oracle 数据库。
- **安全的查询执行：**
  - 严格限制仅允许执行 `SELECT` 和 `WITH` 查询语句，从代码层面拦截 `INSERT`、`UPDATE`、`DELETE`、`DROP` 等写操作。
  - 在数据库执行层内置了最大返回行数限制（最多 500 行），防止恶意大数据量查询导致资源耗尽或大规模数据泄露。
- **SQL 风险静态检测：** 在执行前，对 SQL 语句进行风险规则匹配分析（例如：缺少 WHERE 条件、全表扫描等），并对高风险操作进行预警。
- **执行计划解读 (EXPLAIN)：** 集成了数据库的 EXPLAIN 命令，展示查询执行计划，并可通过 AI 接口提供执行计划的智能解读建议。
- **操作审计日志：** 详细记录每一次查询的审计信息，包括执行者、目标数据库、SQL 指纹及执行耗时。
- **数据导出：** 一键将查询结果导出为 Excel (.xlsx) 文件，解决长数字科学计数法失真问题。
- **连接与权限管理：** 管理员可统一配置数据库连接，并基于用户进行细粒度的连接授权，普通用户仅能看到和操作分配给自己的数据库。
- **SQL 收藏夹：** 用户可以保存常用的 SQL 片段，方便随时调用。
- **角色权限隔离：** 区分普通用户和管理员角色，保障平台管理安全。

## 技术栈

### 后端 (Server)
- **开发语言：** Go (1.20+)
- **Web 框架：** Gin
- **数据库驱动：**
  - `github.com/go-sql-driver/mysql` (MySQL)
  - `github.com/sijms/go-ora/v2` (Oracle)
- **Excel 导出：** `github.com/xuri/excelize/v2`
- **初始化见 `init.sql`

### 前端 (Client)
- **框架：** Vue 3 (Composition API, `<script setup>`)
- **语言：** TypeScript
- **构建工具：** Vite
- **代码编辑器：** CodeMirror 6 (支持 SQL 语法高亮与格式化)
- **样式：** 原生 CSS 结合 Vue 组件作用域

## 目录结构

```text
sql_platform/
├── client/                 # Vue 3 前端工程
│   ├── src/                # 组件、样式及页面逻辑
│   ├── package.json        # 前端依赖配置
│   └── vite.config.ts      # Vite 构建配置
├── server/                 # Go 后端工程
│   ├── auth/               # 认证、鉴权、内置数据库及配置管理
│   ├── middleware/         # Gin 中间件 (如日志、权限拦截)
│   ├── routes/             # API 路由与前端静态资源托管
│   ├── sql/                # SQL 核心引擎 (执行、风险检测、执行计划等)
│   ├── web/dist/           # 编译后的前端静态文件目录 (使用 embed 嵌入)
│   ├── main.go             # 后端服务入口
│   └── go.mod              # Go 模块依赖
├── init.sql                # 数据库初始化脚本
└── README.md               # 项目说明文档
```

## 本地运行指南

### 环境依赖

- Go (1.20 或更高版本)
- Node.js (v20 或更高版本)及 npm 工具

### 1. 编译前端

进入 `client` 目录，安装依赖并进行打包。Vite 会将编译结果输出到 `server/web/dist` 目录下，以便 Go 后端在编译时将其直接嵌入到二进制文件中。

```bash
cd client
npm install
npm run build-only
```

### 2. 运行后端服务

进入 `server` 目录，启动 Gin 服务。默认将在 `2345` 端口运行。

```bash
cd ../server
go mod tidy
go run main.go
```

启动成功后，即可在浏览器中访问：`http://localhost:2345`

## 安全性说明

- **SQL 注入防范：** 平台通过正则表达式过滤 DML 语句，并在最外层使用安全的嵌套策略（`SELECT * FROM ( \n %s \n ) LIMIT ...`）彻底封堵了通过 `--` 或 `#` 注释绕过行数限制的漏洞。
- **敏感信息保护：** 数据库密码等敏感信息加密存储于平台内部数据库中，不会在前端明文暴露。
- **可追溯性：** `logs/access.log` 会记录所有 API 的 HTTP 请求日志，平台内的“审计历史”功能也会持久化保存每一条执行过的 SQL，确保一切操作有迹可循。
