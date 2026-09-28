# Zeabur 部署

使用同一代码仓库的三个常驻服务：API、Worker 和 Web，以及已有 PostgreSQL。迁移是一次性任务，不作为常驻服务保存数据库管理密码。本文描述部署包与检查方法；构建成功不等于云端部署、域名或 TLS 验收完成。

## 服务映射

每个代码服务的构建上下文均为仓库根目录，通过服务自己的 `ZBPACK_DOCKERFILE_PATH` 选择 Dockerfile；不要给所有服务共享这一变量。此变量采用相对构建根目录的完整路径，参见 [Zeabur Dockerfile 部署文档](https://zeabur.com/docs/zh-CN/deploy/methods/dockerfile)。

| 服务 | Dockerfile | 默认进程 | 对外端口 |
| --- | --- | --- | --- |
| API | `deploy/api.zeabur.Dockerfile` | `server` | 私网 8080，无须公共域名 |
| Worker | `deploy/worker.zeabur.Dockerfile` | `worker` | 无 HTTP 端口/公共域名 |
| Web | `deploy/web.zeabur.Dockerfile` | 非 root nginx | 8080，绑定公共 HTTPS 域名 |
| 一次性迁移 | `deploy/migrate.zeabur.Dockerfile` | `migrate`，完成即退出 | 无 |

平台若存在旧的启动命令覆盖，需要清除或使其与上述进程一致。Worker 镜像只包含 Worker 与 SSH 安装需要的 Agent 产物；API 镜像包含 server、Agent 和固定摘要校验的协议运行时；迁移镜像只包含 migrate。API/Worker 不携带迁移程序。

## 数据库与机密分离

先对已有 PostgreSQL 备份并核实恢复能力，再通过一次性迁移创建数据库角色并应用迁移。

迁移任务临时接收：

- `DATABASE_URL`：具有迁移与角色配置权限的专用管理连接；以 PostgreSQL 服务实际地址、数据库和 TLS 要求为准。
- `XINGDU_APP_DATABASE_PASSWORD`：为 `xingdu_app` 生成的独立随机密码。
- `XINGDU_WORKER_DATABASE_PASSWORD`：为 `xingdu_worker` 生成的另一独立随机密码。
- 已有数据库执行前缀 ID 迁移时，临时提供现有的 `XINGDU_CREDENTIAL_KEY`，用于在同一事务内重新绑定加密凭据；不能生成新密钥替代。迁移完成后删除临时服务变量。详见 [资源 ID 迁移](ID-DESIGN.md)。

迁移镜像从仓库根目录构建：

```sh
docker build -f deploy/migrate.zeabur.Dockerfile -t xingdu-migrate:release .
```

在能够访问目标数据库的受控执行环境，通过 secret manager 注入上述变量后运行。不要将机密写入命令行参数、Git 文件或日志：

```sh
docker run --rm \
  --env DATABASE_URL \
  --env XINGDU_APP_DATABASE_PASSWORD \
  --env XINGDU_WORKER_DATABASE_PASSWORD \
  xingdu-migrate:release
```

公网运行环境无法解析平台私网数据库地址时，需在同一私网的一次性任务中运行，或使用受保护的数据库隧道；不要为了迁移直接开放未保护的 PostgreSQL 公网端口。可在受控环境直接运行同版本的 `migrate` 可执行程序。也可在同一 Zeabur 项目创建专用的临时迁移服务，选择迁移 Dockerfile，仅向该临时服务注入管理连接与角色密码。确认日志显示迁移完成且进程成功退出后，立即停止/删除临时服务并移除其管理变量；若平台自动重启退出的进程，应及时暂停服务，避免反复执行。不要通过无限 sleep 保持管理员服务存活。

API 只接收 `xingdu_app` 的 `DATABASE_URL`，Worker 只接收 `xingdu_worker` 的 `DATABASE_URL`。不得把平台默认的 PostgreSQL 管理连接直接引用到 API/Worker，也不要把 `XINGDU_APP_DATABASE_PASSWORD`、`XINGDU_WORKER_DATABASE_PASSWORD` 或管理连接作为项目级共享变量。数据库密码含特殊字符时，连接 URL 必须按 URL 用户信息规则编码。

运行时仍由代码拒绝数据库所有者/超管连接，并使用事务内租户上下文和 PostgreSQL RLS；不要通过提高 API/Worker 角色权限修复配置错误。

## 常驻服务变量

API：

- `DATABASE_URL`：`xingdu_app` 专用连接。
- `XINGDU_HTTP_ADDR=0.0.0.0:8080`：镜像默认值。
- `XINGDU_PUBLIC_ORIGIN=https://<公共域名>`：浏览器访问的唯一正式 origin，无路径或尾斜线。
- `XINGDU_AGENT_ORIGIN=https://<公共域名>`：机器能够访问的受信任 HTTPS origin，Web 会转发 Agent API。
- `XINGDU_CREDENTIAL_KEY`：64 位十六进制随机密钥，与 Worker 保持一致，另行安全备份。
- `XINGDU_REGISTRATION_ENABLED=false`：默认关闭公开注册；完成账户与邮件流程验证后按运营要求开放。
- Google / GitHub 可选登录：仅 API 配置各自的 `XINGDU_GOOGLE_CLIENT_ID`、`XINGDU_GOOGLE_CLIENT_SECRET`、`XINGDU_GITHUB_CLIENT_ID`、`XINGDU_GITHUB_CLIENT_SECRET`，回调配置见 [第三方登录](SOCIAL-LOGIN.md)。
- `XINGDU_SSH_ALLOWED_CIDRS`：通常留空，保持私网/元数据地址拒绝策略；仅在明确需要的管理网段设置。

Worker：

- `DATABASE_URL`：`xingdu_worker` 专用连接。
- 与 API 相同的 `XINGDU_PUBLIC_ORIGIN`、`XINGDU_AGENT_ORIGIN`、`XINGDU_CREDENTIAL_KEY` 和按需设置的 `XINGDU_SSH_ALLOWED_CIDRS`。
- 不配置 `PORT`、公共 HTTP 域名或 API 的启动命令。

Web：

- `XINGDU_API_UPSTREAM=<API 私网 DNS 主机名>:8080`：必填，例如 `xingdu-api.zeabur.internal:8080`；实际名称以该服务的私网连接信息为准，重命名服务不会自动修改该主机名，参见 [Zeabur 私网文档](https://zeabur.com/docs/zh-CN/deploy/networking/private-networking)。只接受 DNS 主机名与端口，不接受 URL、凭据或路径。
- 不接收数据库密码、凭据加密密钥或邮件凭据。

Web 启动脚本从容器 `/etc/resolv.conf` 读取实际 DNS resolver，用 nginx 动态解析 API 私网主机名。没有写死 Docker 的 `127.0.0.11`，API 重建改变私网 IP 后无需重新构建 Web。nginx 模板只替换指定的两个环境变量，保留 `$uri` 等 nginx 变量；订阅 token 查询参数的代理访问日志和错误日志均关闭。

## 健康与上线检查

- API 私网 `GET /health/ready`：检查运行时数据库连接及所需迁移；API 镜像内健康检查使用该路径。
- Web `GET /health/web`：只检查 nginx；不能当作 API 健康证明。
- 公共域名 `GET /health/ready`：通过 Web 反代核实 API 可达。
- 公共首页、`/app/nodes` 等直接 URL、登录与刷新、会话 Cookie Secure 属性、组织隔离、API/Worker 数据库身份，需要分别验证。
- Agent 安装地址必须返回当前构建产物，并从机器侧实际验证 HTTPS 信任与心跳；Web/API HTTP 200 不能替代此项。
- Worker 无 HTTP 服务，需通过平台进程状态、无敏感信息的日志及受控任务执行验证。不要给 Worker 配置 HTTP 探活后把正常后台进程误判故障。

先完成迁移，再滚动部署 API 与 Worker，确认 API readiness 后部署 Web。涉及不兼容数据库修改时单独安排维护窗口；不要假设回滚镜像会自动回滚 schema。加密密钥丢失无法解密历史机器凭据，数据库备份必须与密钥备份、恢复演练配套。

## 可选：Stripe 云端计费

默认 `MODE=self_hosted`，不收取订阅费。托管服务准备接入支付时，
仅在 API 服务配置 `MODE=cloud`、`STRIPE_SECRET_KEY`、
`STRIPE_WEBHOOK_SECRET`、`STRIPE_PRICE_MONTHLY`、`STRIPE_PRICE_YEARLY` 和
`STRIPE_PORTAL_CONFIGURATION`。不要把 Stripe 密钥配置到 Web 构建环境。
价格固定为 USD 5/月、USD 40/年，每组织 5 台服务器；配置和测试步骤见
[BILLING.md](BILLING.md)。Web 模板已转发 `/api/v1/billing/stripe/webhook`，
公开回调地址应使用该部署实际的 HTTPS 域名。真实支付和公网回调需单独验收。
