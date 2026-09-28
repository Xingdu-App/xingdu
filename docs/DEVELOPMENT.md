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

如果已有 `.env`，不要覆盖。示例密码仅供本地开发；`DATABASE_URL` 中的密码必须与 `POSTGRES_PASSWORD` 一致，特殊字符需要 URL 编码。当前 Compose 用于开发；完整的生产身份认证、TLS 与密钥管理尚未实现。

| 服务 | 地址 / 行为 |
| --- | --- |
| Web | http://127.0.0.1:15173 |
| API | http://127.0.0.1:18080 |
| PostgreSQL | 127.0.0.1:54329 |
| Migrate | 一次性应用嵌入式迁移，成功退出 |
| Worker | 骨架进程，检查数据库；不执行部署任务 |

Agent 当前仅有独立进程入口，不在 Compose 中连接真实主机。

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
make migrate
make dev-api
```

另开一个终端，在仓库根目录执行 `make dev-web`。控制台使用同源 API 代理，不需要开放 CORS。

Worker 可使用 `make worker`，Agent 可使用 `make agent`；二者支持 SIGINT / SIGTERM 退出，Agent 尚不注册设备或修改系统。

## 检查与构建

```sh
make check
make build
```

`make check` 包括 Go vet、race 测试、前端 lint 与生产构建。`make build` 生成 `bin/` 下四个 Go 程序及 `apps/web/dist/`。

数据库集成测试需使用专用测试数据库：

```sh
docker compose exec -T db createdb -U xingdu xingdu_test
XINGDU_TEST_DATABASE_URL='postgres://xingdu:xingdu-local-dev@127.0.0.1:54329/xingdu_test?sslmode=disable' go test -race ./internal/storage -count=1
```

已有测试数据库时不重复执行 createdb；如修改了密码，同步修改测试连接。未设置 `XINGDU_TEST_DATABASE_URL` 时，数据库集成测试会明确跳过。CI 提供独立测试数据库。

## 当前 API

- `GET /health/live`：进程存活，不依赖数据库。
- `GET /health/ready`：数据库可用且所需 schema 已应用，否则 503。
- `GET /api/v1/system`：版本、阶段、数据库状态及未开放能力。
- `GET /api/v1/hosts`：数据库中的真实服务器列表；空库返回 `data: []`，数据库失败返回 503。

没有写接口、登录或公开 Agent 接入端点。不要将当前开发 API 暴露到公网。

## Git 身份与隐私

本项目维护者的本地提交身份使用 `Xingdu <noreply@xingdu.app>`。新建开发目录后仅设置仓库级配置：

```sh
git config --local user.name Xingdu
git config --local user.email noreply@xingdu.app
```

提交前核对 author 和 committer；CI 发布流程尚未建立，不会自动发布包。真实 `.env`、运行数据和本机路径不应提交。外部贡献者可使用自己的公开身份；贡献继续适用 MIT。
