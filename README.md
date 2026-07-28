# 数据库查询平台

基于 Go、Gin 和 Vue 3 构建的受控只读数据库查询平台。系统只保留用户认证、连接授权、数据库连接管理、查询工作台、元数据浏览、查询历史、Excel 导出和 SQL 收藏夹。

![登录页面](./server/images/login.jpg)

![查询工作台](./server/images/query.jpg)

![数据库连接管理](./server/images/connect.jpg)

![用户权限管理](./server/images/user.jpg)

## 功能

### 用户与权限

- 账号密码登录、退出、获取当前用户和修改密码
- 会话失效或未登录时统一返回登录页，业务 API 返回 `401`
- 支持 `admin` 和 `user` 角色
- 管理员维护用户、数据库连接和用户连接权限
- 普通用户只能看到和查询已授权连接
- 新建用户首次登录时强制修改密码

### 数据库连接

| 数据库     | 默认端口 | 连接范围              | 元数据范围                                                    |
| ---------- | -------: | --------------------- | ------------------------------------------------------------- |
| MySQL      |     3306 | 数据库名              | 配置数据库中的表和字段                                        |
| Oracle     |     1521 | 服务名，可选 schema   | 配置 schema；未配置时使用连接用户                             |
| PostgreSQL |     5432 | 数据库名，可选 schema | 配置 schema；未配置时使用 `current_schema()`，通常为 `public` |
| MSSQL      |     1433 | 数据库名              | 当前数据库内可见 schema 的表、视图和字段                      |

连接管理支持新增、编辑、删除、测试连接、启用/禁用以及 `can_connect` 查询开关。测试失败时在表单内显示错误信息，不弹出额外错误窗口。

Oracle 只读账号查询其他 schema 时，除了目标对象的 `SELECT` 权限，还要确保账号能读取相应数据字典视图。PostgreSQL 指定 schema 时，需要授予 schema 的 `USAGE` 权限和目标表的 `SELECT` 权限。

### 查询工作台

- 只允许单条 `SELECT` 或 `WITH` 查询
- 服务端最多返回 500 行
- 支持 `Ctrl + Enter` 执行和一键清空编辑器
- 支持 SQL 关键字、表、字段及表别名字段补全
- 元数据支持按表名、字段名和注释搜索
- 选中表后展示字段类型和字段注释
- 左侧导航栏和右侧元数据栏均可折叠
- 查询结果位于编辑器和元数据区域下方，可拖动调整高度、隐藏或最大化
- 查询结果支持每页 20、50 或 100 行
- 查询结果可导出为 Excel `.xlsx`

### 历史与收藏

- 查询历史按登录用户保存在浏览器 `localStorage`
- 仅保留最近 20 条成功且有返回数据的查询记录
- 历史和收藏位于同一个右侧抽屉，通过 Tab 切换
- 查询历史可一键加入 SQL 收藏夹
- SQL 收藏由后端持久化，支持保存、编辑、删除和复用

## 安全边界

- 查询、元数据、Excel 导出和 SQL 收藏接口均要求登录
- 服务端会再次校验用户与数据库连接的授权关系
- 应用层拒绝显式跨范围引用：MySQL 跨数据库、PostgreSQL/Oracle 跨 schema、MSSQL 跨数据库和 linked server
- schema、`search_path` 和默认数据库不能替代数据库自身权限控制
- 每个连接应使用专用只读账号，仅授予目标库或 schema 的必要查询权限
- 不要配置 MySQL `root`、Oracle `SYS`、MSSQL `sa` 或其他高权限账号
- API 响应使用 `Cache-Control: no-store`，避免退出后复用旧的认证或业务数据
- 管控库配置文件包含密码，禁止提交实际使用的 `db.yml`

## 技术栈

### 后端

- Go 1.25.11
- Gin 1.12
- MySQL 5.7 管控库
- MySQL：`go-sql-driver/mysql`
- Oracle：`sijms/go-ora/v2`
- PostgreSQL：`lib/pq`
- MSSQL：`go-mssqldb`
- YAML 配置：`goccy/go-yaml`
- Excel 导出：`xuri/excelize`
- 前端资源通过 Go `embed` 嵌入可执行文件

### 前端

- Vue 3.5
- TypeScript 5.9
- Vite 7
- CodeMirror 6
- Lucide 图标
- 原生 CSS

Node.js 要求为 `20.19.0` 及以上的 Node 20，或 `22.12.0` 及以上版本。

## 数据表

| 表名                 | 说明               |
| -------------------- | ------------------ |
| `user`               | 平台用户           |
| `session`            | 登录会话           |
| `db_connection`      | 数据库连接配置     |
| `user_db_connection` | 用户与连接授权关系 |
| `sql_favorite`       | SQL 收藏           |

## 目录结构

```text
db_platform/
├── client/                     # Vue 前端
│   ├── src/App.vue             # 单页应用
│   ├── package.json
│   └── vite.config.ts
├── server/                     # Go 后端
│   ├── auth/                   # 会话与鉴权中间件
│   ├── config/                 # YAML 配置加载与管控库连接
│   ├── routes/                 # API 与静态资源路由
│   ├── sql/                    # 查询、元数据、连接、用户和收藏逻辑
│   ├── web/dist/               # 前端构建产物
│   ├── logs/                   # 默认访问日志目录
│   ├── main.go
│   └── go.mod
├── db.example.yml              # 后端配置示例
├── init.sql                    # 管控库初始化脚本
└── README.md
```

## 初始化

### 1. 初始化管控库

项目当前使用 MySQL 5.7 作为管控库。执行前必须修改 `init.sql` 中 `fixed_aes_encrypt`、`fixed_aes_decrypt` 使用的固定密钥，以及初始化管理员密码，然后执行：

```bash
mysql -u root -p < init.sql
```

初始化脚本会创建 `db_platform` 数据库、业务表、加解密函数和默认管理员。

### 2. 创建配置文件

在项目根目录复制配置示例：

Linux / macOS：

```bash
cp db.example.yml db.yml
```

Windows PowerShell：

```powershell
Copy-Item db.example.yml db.yml
```

编辑 `db.yml`，填写管控库的实际连接信息：

```yaml
platform_db:
  host: 127.0.0.1
  port: 3306
  user: db_platform
  password: 替换为实际密码
  name: db_platform

session:
  cookie_name: db_platform_session_token
  expire_hours: 8
```

配置说明：

| 配置项                 | 是否必填 | 默认值                      | 说明                   |
| ---------------------- | -------- | --------------------------- | ---------------------- |
| `platform_db.host`     | 是       | 无                          | 管控库地址             |
| `platform_db.port`     | 否       | `3306`                      | 管控库端口             |
| `platform_db.user`     | 是       | 无                          | 管控库账号             |
| `platform_db.password` | 是       | 无                          | 管控库密码             |
| `platform_db.name`     | 是       | 无                          | 管控库名称             |
| `session.cookie_name`  | 否       | `db_platform_session_token` | 登录 Cookie 名称       |
| `session.expire_hours` | 否       | `8`                         | 会话有效期，单位为小时 |

`db.yml` 已加入 `.gitignore`。配置文件不存在、YAML 格式错误或必填项缺失时，服务会在启动阶段直接终止。

### 3. 构建前端

```bash
cd client
npm install
npm run build
```

构建结果输出到 `server/web/dist`。修改前端后必须重新构建前端，再编译后端。

## 运行

生产或日常使用时，推荐直接运行编译好的二进制文件。二进制文件已经嵌入 `server/web/dist` 中的前端资源，不需要单独启动前端服务。

Linux / macOS：

```bash
./db_platform --config_file db.yml
```

Windows PowerShell：

```powershell
.\db_platform.exe --config_file db.yml
```

`--config_file` 是必填参数，支持相对路径和绝对路径。参数名是 `--config_file`

默认监听 `1520` 端口，访问 `http://localhost:1520`。通过 `PORT` 修改端口：

Linux / macOS：

```bash
PORT=8080 ./db_platform --config_file db.yml
```

Windows PowerShell：

```powershell
$env:PORT = "8080"
.\db_platform.exe --config_file db.yml
```

查看帮助：

Linux / macOS：

```bash
./db_platform --help
```

Windows PowerShell：

```powershell
.\db_platform.exe --help
```

开发调试时也可以不提前编译，直接用 `go run`：

```bash
cd server
go run . --config_file ../db.yml
```

`go run` 使用同样的配置文件参数、端口环境变量和帮助参数：

```bash
go run . --help
```

## 编译

Linux / macOS：

```bash
cd server
go build -o db_platform .
```

Windows PowerShell：

```powershell
cd server
go build -o db_platform.exe .
```

编译后按上一节的二进制运行方式启动服务。

## 访问日志(调整目录)

- 默认日志目录为程序工作目录下的 `logs`
- 文件名为 `access-YYYY-MM-DD.log`，按本地日期每天生成一个文件
- 服务跨过午夜后自动切换文件，无需重启
- 每次启动和日期切换时清理超过 14 天的日志
- 目录不存在时自动创建；目录不可写时启动失败
- 仅记录 `/api/` 请求的时间、客户端 IP、方法、路径、状态码、耗时和 User-Agent
- 不记录请求体、登录密码或数据库密码

通过 `LOG_DIR` 指定日志目录。

Linux / macOS：

```bash
LOG_DIR=/var/log/db-platform ./db_platform --config_file db.yml
```

Windows PowerShell：

```powershell
$env:LOG_DIR = "D:\db-platform-logs"
.\db_platform.exe --config_file ..\db.yml
```

## 默认管理员

当前初始化脚本创建以下账号：

- 用户名：`admin`
- 密码：`Admin`

默认管理员不会被强制要求首次改密，初始化或首次部署后必须立即修改密码。

## 验证

前端：

```bash
cd client
npm run build
```

后端：

```bash
cd server
go test ./...
```
