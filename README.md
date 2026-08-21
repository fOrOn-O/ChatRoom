# ChatRoom

一个前后端分离的即时通讯项目。后端使用 Go 提供 REST API 与 WebSocket 长连接，前端使用 Vue 3 构建实时聊天工作台。

面向小型团队和社区场景，提供从账号体系、联系人与群组管理到实时收发消息、文件传输的一体化体验。

## 核心能力

- 账号注册、登录与 JWT 鉴权
- 个人资料查询和更新，用户搜索
- 联系人添加、列表查询与删除
- 群组创建、成员查询、邀请、移除与退出
- 私聊、群聊与历史消息查询
- WebSocket 心跳、自动重连、增量消息补偿、在线状态和发送确认
- 图片与文件上传、受鉴权的文件下载
- 统一错误提示、路由鉴权、响应式聊天界面

## 项目亮点

- **实时消息链路**：通过 WebSocket 处理消息推送、心跳保活、发送确认、断线重连与遗漏消息补偿。
- **清晰的后端组织**：路由、中间件、Handler、数据模型和 WebSocket Hub 分层明确。
- **并发连接管理**：每个连接独立读写，Hub 使用 Channel 路由消息，并以 RWMutex 管理在线状态。
- **完整产品体验**：从认证、联系人、群组到文件与个人资料，覆盖高频即时通讯场景。
- **开箱即用开发环境**：前端已配置 API、WebSocket 和静态资源代理。

## 架构一览

~~~
Vue 3 + Vite
      │  HTTP / WebSocket
      ▼
Go + Gin API
      │
      ├── JWT 鉴权
      ├── WebSocket Hub
      └── GORM
             │
      ┌──────┴──────┐
      ▼             ▼
   MySQL          Redis
~~~

## 技术栈

| 层级 | 技术 |
| --- | --- |
| 后端 | Go、Gin、GORM、gorilla/websocket、Zap |
| 数据 | MySQL 8、Redis 7 |
| 认证 | JWT、bcrypt |
| 前端 | Vue 3、Vite、Pinia、Vue Router、Axios、Element Plus |

## 项目结构

~~~
ChatRoom/
├── backend/
│   ├── cmd/server/          # 服务启动入口
│   ├── configs/             # 示例配置与本地配置
│   ├── internal/            # API、模型与 WebSocket
│   ├── pkg/                 # 配置、数据库、日志、鉴权工具
│   └── scripts/init.sql     # 数据库结构
├── frontend/
│   ├── src/api/             # REST 请求封装
│   ├── src/stores/          # 用户与会话状态
│   ├── src/views/           # 登录、注册、聊天工作台
│   └── src/websocket/       # WebSocket 连接与重连
└── docs/                    # 项目设计资料
~~~

## 本地启动

### 1. 前置条件

- Go：版本以 backend/go.mod 为准
- Node.js 20+（含 npm）
- MySQL 8+
- Redis 7+

### 2. 配置并启动后端

在项目根目录执行：

~~~powershell
cd backend
Copy-Item configs/config.example.yaml configs/config.yaml
~~~

编辑新创建的 configs/config.yaml，填写本机 MySQL、Redis 连接信息，并生成一个独立 JWT 密钥：

~~~powershell
openssl rand -hex 32
~~~

将输出填入 jwt.secret。config.yaml 已被 .gitignore 忽略，不能提交。

创建数据库结构后启动服务：

~~~powershell
# 在 PowerShell 中
Get-Content scripts/init.sql | mysql -u root -p

go mod download
go run ./cmd/server
~~~

服务默认监听 http://localhost:8080，健康检查地址为 GET /health。

### 3. 启动前端

另开一个终端：

~~~powershell
cd frontend
npm install
npm run dev
~~~

访问 Vite 输出的本地地址（默认 http://localhost:5173）。开发服务器会把 /api、/ws 和 /static 代理到 http://localhost:8080。

## API 概览

所有业务接口的前缀为 /api/v1；除注册和登录外，均需 Authorization: Bearer token。

| 模块 | 接口 |
| --- | --- |
| 认证 | POST /auth/register、POST /auth/login |
| 用户 | GET/PUT /user/profile、GET /users/search |
| 联系人 | GET /friends、POST /friends/request、DELETE /friends/:friend_id |
| 群组 | GET/POST /groups、群详情、成员查询/邀请/移除、退出 |
| 消息 | GET /messages |
| 文件 | POST /files/upload、GET /files/:file_id/download |

WebSocket 地址：ws://localhost:8080/ws?token=token。客户端发送 chat 事件；服务端会推送 chat、chat_ack 和 online_status。

## 验证

~~~powershell
# 后端：编译并执行测试
cd backend
go test ./...

# 前端：生产构建
cd frontend
npm run build
~~~

## 配置说明

仓库提供 backend/configs/config.example.yaml 作为配置模板。复制为 config.yaml 后，填写本地数据库和 Redis 连接信息即可开始开发。Redis 是后端启动依赖，服务启动时会在连接超时窗口内执行连通性检查，连接失败则终止启动。

`redis.key_prefix` 用于隔离不同应用或环境的键空间；`dial_timeout`、`read_timeout` 和 `write_timeout` 控制 Redis 网络操作超时。聊天消息由 WebSocket 入口发布到 `redis.stream`，消费者完成 MySQL 持久化、实时投递和 ACK；处理失败的消息会进入 Pending 恢复流程，超过重试上限后转入死信 Stream。

## 文档

详细的接口设计、数据库模型与架构说明见 docs/开发文档.md。
