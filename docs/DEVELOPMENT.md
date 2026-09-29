# 开发指南

## 环境

- Go 1.26.1 或更新的兼容版本（以 go.mod 为准）。
- Node.js 24 LTS；本地也可使用 package.json engines 范围内的版本。
- Docker 与 Compose v2。

## 完整容器环境

在仓库根目录：

```sh
cp .env.example .env
make up
```

如果已有 `.env`，不要覆盖。示例密码仅供本地开发；迁移使用 `POSTGRES_PASSWORD`；API/Worker 使用独立 `XINGDU_APP_DATABASE_PASSWORD`（至少 16 字节），Worker 使用 `XINGDU_WORKER_DATABASE_PASSWORD`。Compose 中建议使用 URL 安全的随机十六进制密码；手动配置连接串时对特殊字符进行 URL 编码。当前 Compose 用于本地开发；公网自托管需要另行配置域名、HTTPS、邮件和运维措施。已有 `.env` 需新增运行账号密码，不能继续让 API 使用迁移账号。

| 服务 | 地址 / 行为 |
| --- | --- |
| Web | http://127.0.0.1:15173 |
| API | http://127.0.0.1:18080 |
| PostgreSQL | 127.0.0.1:54329 |
| Migrate | 一次性应用嵌入式迁移，成功退出 |
| Worker | 执行 SSH 安装队列；不执行协议部署任务 |

Agent 提供注册、心跳和 Linux systemd 安装，不在 Compose 中自动连接真实 VPS。使用方式见 [机器接入](MACHINE-ACCESS.md)。跨发行版的真实 systemd/SSH 接入验收可运行 `make agent-lab-up`、`make agent-lab-test`，环境边界与清理见 [Docker 实验室](AGENT-LAB.md)。

停止服务：`make down`。数据库命名卷保留；不要随意使用 `docker compose down -v`，该命令会删除数据库数据。

## 热更新开发

停止完整环境后，仅启动数据库，并在根目录加载本地开发变量：

```sh
make down
docker compose up -d --wait db
set -a
. ./.env
set +a
make setup
docker compose run --rm migrate
# DATABASE_URL 必须指向 xingdu_app，且使用对应密码
make dev-api
```

另开一个终端，在仓库根目录执行 `make dev-web`。控制台使用同源 API 代理，不需要开放 CORS。

Worker 可使用 `make worker`，Agent 可使用 `make agent`；二者支持 SIGINT / SIGTERM 退出，Agent 未指定安装参数时只发送心跳，`--install` 明确执行 Linux systemd 安装。

## 检查与构建

```sh
make check
make build
```

`make check` 包括 Go vet、race 测试、前端 lint 与生产构建。`make build` 生成 `bin/` 下五个 Go 程序及 `apps/web/dist/`。

数据库集成测试需使用专用测试数据库：

```sh
docker compose exec -T db createdb -U xingdu xingdu_test
XINGDU_TEST_DATABASE_URL='postgres://xingdu:xingdu-local-dev@127.0.0.1:54329/xingdu_test?sslmode=disable' go test -race ./internal/storage ./internal/httpapi -count=1
```

已有测试数据库时不重复执行 createdb；如修改了密码，同步修改测试连接。未设置 `XINGDU_TEST_DATABASE_URL` 时，数据库集成测试会明确跳过。CI 提供独立测试数据库。

## 用户初始化与登录

完整容器环境启动后执行：

```sh
docker compose exec api admin --username admin
```

密码在终端隐藏输入，长度为 12–72 字节，不放入命令参数或日志。该命令创建用户及其初始组织，可多次创建不同用户名，不覆盖已有账号。自动化可通过标准输入传入密码，勿将真实密码写入脚本或提交历史。

本机 Go 开发环境加载 `.env` 并应用迁移后可用 `go run ./cmd/admin --username admin`。

会话有效期为 24 小时，数据库只保存令牌的 SHA-256 摘要。Cookie 设置 HttpOnly 和 SameSite=Strict；HTTPS 来源启用 Secure。退出后数据库会话立即撤销。页面刷新会恢复有效会话，过期会回到登录页。

`XINGDU_PUBLIC_ORIGIN` 默认 `http://127.0.0.1:15173`，必须与浏览器地址的协议、主机和端口一致，不带末尾斜杠。非回环来源必须使用 HTTPS；不得通过伪造代理头关闭保护。当前 Compose 仍仅面向本地开发。

写请求同时校验 Origin、Sec-Fetch-Site（若存在）、JSON Content-Type 和 `X-Xingdu-Request: 1`；登录后的写操作还必须提供 `X-CSRF-Token`。前端自动处理这些请求头。登录有每来源 IP 每分钟 10 次限流以及密码计算并发限制；当前反向代理下来源地址可能聚合，公网部署前需设计可信代理与分布式限流。

## 组织与成员

架构、角色、RLS 和邀请语义见 [SaaS 设计](SAAS.md)。

- `GET/POST /api/v1/organizations`：列出自己的组织 / 创建组织。
- `GET /api/v1/members`、`PUT/DELETE /api/v1/members/{id}`：当前组织成员及权限管理。
- `GET/POST /api/v1/invitations`、`DELETE /api/v1/invitations/{id}`：邀请列表、生成和撤销。
- `POST /api/v1/invitations/accept`：使用请求体中的 `token` 接受邀请，不自动提升现有成员权限。
- `GET /api/v1/auth/config`：查询公开注册是否开放。
- `POST /api/v1/auth/register`：显式开启注册后使用用户名、密码和 `organization` 名称创建账号。

服务器、成员和邀请管理请求必须携带 `X-Xingdu-Organization` 组织 ID（`org_` 加 32 位小写十六进制），服务端在每次操作时验证成员关系；无成员资格返回 403。接受邀请和组织列表不依赖当前选择。公开注册默认关闭，设置 `XINGDU_REGISTRATION_ENABLED=true` 可启用。

## 当前 API

公开端点：

- `GET /health/live`：进程存活，不依赖数据库。
- `GET /health/ready`：数据库与所需 schema 可用，否则 503。
- `POST /api/v1/auth/login`：用户名与密码登录，设置新会话 Cookie。

需要登录：

- `GET /api/v1/auth/session`：当前用户 ID、用户名与 CSRF 令牌。
- `POST /api/v1/auth/logout`：撤销当前会话并清除 Cookie。
- `GET /api/v1/system`：版本、阶段、数据库状态及功能能力。
- `GET /api/v1/hosts`：真实服务器资料列表，空库返回 `data: []`。
- `POST /api/v1/hosts`：新增服务器资料。
- `PUT /api/v1/hosts/{id}`：完整更新服务器资料。
- `DELETE /api/v1/hosts/{id}`：删除资料，不操作实际 VPS。

服务器字段为 `name`、`address`、`ssh_port`、`ssh_user`、`tags`、`notes`。同一组织内地址与端口组合不能重复。状态由服务端管理，新增固定为 pending；不能通过更新资料伪造在线状态。SSH 密码和私钥使用独立安装接口提交，不属于服务器资料字段。

未登录返回 401，跨站或 CSRF 验证失败返回 403，重复记录返回 409，字段校验失败返回 422，数据库不可用返回 503。底层数据库错误不会直接返回给客户端。

## Git 身份与隐私

本项目维护者的本地提交身份使用 `Xingdu <noreply@xingdu.app>`。新建开发目录后仅设置仓库级配置：

```sh
git config --local user.name Xingdu
git config --local user.email noreply@xingdu.app
```

提交前核对 author 和 committer；CI 发布流程尚未建立，不会自动发布包。真实 `.env`、运行数据和本机路径不应提交。外部贡献者可使用自己的公开身份；贡献继续适用 MIT。

## 机器接入配置

新增 `XINGDU_WORKER_DATABASE_PASSWORD`（独立低权限 Worker 账号），可选 `XINGDU_CREDENTIAL_KEY`（64 位随机 hex，用于 SSH 凭据加密）和 `XINGDU_AGENT_ORIGIN`（VPS 可达的 HTTPS 来源）。私有网络 SSH 需部署者显式配置 `XINGDU_SSH_ALLOWED_CIDRS`。没有加密密钥时，SSH 安装关闭，主动 Agent 注册仍可用。详细端点与安全语义见 [机器接入](MACHINE-ACCESS.md)。

资源 ID 的类型前缀、128 位随机部分与迁移边界见 [ID 设计](ID-DESIGN.md)。浏览器 URL 和 JSON 中应完整保留 ID，不移除前缀，不按 UUID 解析。
