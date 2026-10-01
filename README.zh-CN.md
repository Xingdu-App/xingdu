<p align="center">
  <img src="apps/web/public/xingdu-logo.png" width="128" alt="Xingdu logo" />
</p>

# 星渡 · Xingdu

**连点成网，一键抵达。**

[English](README.md) · [简体中文](README.zh-CN.md)

[![CI](https://github.com/Xingdu-App/xingdu/actions/workflows/ci.yml/badge.svg)](https://github.com/Xingdu-App/xingdu/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

星渡是一个开源的 VPS 与代理线路管理项目，致力于将服务器接入、协议部署、直连与中转线路、客户端订阅整合到一个支持自托管的多租户 SaaS 控制台中。

面向个人和小团队，星渡采用独立的协议引擎与客户端适配设计，让你自由选择客户端，统一管理自己的服务器与线路。

> **Xingdu Cloud 已在 [xingdu.app](https://xingdu.app) 正式上线。** 可以使用云端托管服务，也可以自行部署 MIT 开源版本。协议、客户端导出范围与运维要求见下方文档；使用前请验证目标服务器与客户端组合，本地构建成功不代表兼容性或恢复能力已经验证。

## 现在可以体验什么

- **服务器资料：** 新增、编辑和删除服务器记录，管理地址、SSH 连接信息、标签和备注。
- **组织协作：** 创建与切换组织，邀请成员，分配所有者、管理员、成员和只读权限。
- **租户隔离：** PostgreSQL 行级安全策略与独立低权限运行账号。
- **用户登录：** 邮箱验证注册、邮箱/密码登录，以及可选 Google / GitHub 登录与账号绑定；支持本地创建账号和可撤销会话。第三方应用配置与验收边界见 [登录说明](docs/SOCIAL-LOGIN.md)。
- **机器接入：** 主动安装 Agent，或通过 SSH 密码 / 私钥安装；校验主机指纹，可选长期加密保存凭据。配置与验收边界见 [机器接入](docs/MACHINE-ACCESS.md)。
- **节点列表：** 按组织汇总已成功部署的节点，支持搜索、协议筛选与节点详情；卸载成功后移出列表，操作结果保留在部署记录中。
- **协议部署与维护：** 托管 Agent 提供 SS/SS2022、Trojan、VLESS、VMess、HY1/HY2、TUIC、AnyTLS、HTTPS、SOCKS5/Mixed、ShadowTLS 和 Snell 选项，支持预检、重启、配置编辑、凭据轮换及版本恢复。传输、证书、最低 Agent 版本与验收范围见 [协议部署](docs/PROTOCOL-DEPLOYMENT.md) 和 [可靠部署](docs/RELIABLE-DEPLOYMENTS.md)。
- **协议参数矩阵（开发中）：** 独立 Xray/sing-box 适配器、XHTTP 的 HTTP/1.1/2/3 分离上下行、VLESS Encryption/Vision、WireGuard 与 QUIC 参数。要求 Agent 0.16.0-dev；本地参考客户端 72 项已通过，公网部署与客户端 App 验收在[矩阵](docs/PROTOCOL-MATRIX-72.md)中分别记录。
- **真实服务状态：** 查看 API 和数据库是否可用，连接失败时显示错误并支持重试。
- **客户端订阅：** 生成可撤销的 Stash（默认）、Mihomo、Surge、Loon 配置链接或 HY2 URI。支持自定义规则、可复用模板与多策略组路由方案；各格式明确拒绝不兼容组合。详见 [订阅](docs/SUBSCRIPTIONS.md) 与 [路由模板](docs/SUBSCRIPTION-TEMPLATES.md)。
- **本地 Docker 环境：** 一次启动控制台、API、PostgreSQL、数据库迁移和 Worker 进程。
- **开发基础：** Go 与 TypeScript 代码、自动化检查，以及独立的 Worker 和 Agent 程序入口。

Worker 执行 SSH 引导安装，Agent 主动领取授权的固定任务，不开放任意远程命令。控制台支持简体中文与英文。已有容器和部分实机验收，包括 AlmaLinux 10.2 / amd64 / SELinux Enforcing 的 SS2022 修复及 Stash macOS 4.3.0 节点测试；这些结果不代表所有协议、发行版或客户端版本均兼容。完整范围与源码／发布包区别见 [功能状态](docs/IMPLEMENTATION-STATUS.md)。

## 使用流程与后续方向

```text
接入 VPS → 选择协议与线路 → 部署并验证 → 导入客户端
```

已实现节点配置版本、失败恢复、单层 TCP 中转和运营者独立探测程序。
自动证书使用运营者部署的 ACME DNS-01 / Cloudflare 程序，尚无租户自助授权。
后续重点是实际客户端组合、公网故障恢复与证书续期验收，以及 Passkey、
通知告警和更多传输方式。见 [功能状态](docs/IMPLEMENTATION-STATUS.md)
与 [开发计划](docs/PLAN.md)。

## 本地体验

需要安装 **Git、Docker 和 Docker Compose v2**。使用容器运行项目，无须在本机安装 Go 或 Node.js。

```sh
git clone https://github.com/Xingdu-App/xingdu.git
cd xingdu
cp .env.example .env
docker compose up --build -d --wait
```

启动后打开官网 **[http://127.0.0.1:15173](http://127.0.0.1:15173)**，控制台入口为 **[/app](http://127.0.0.1:15173/app)**。官网提供价格、隐私和安全说明；托管版提供 Starter、Premium 与 Enterprise 套餐，见 [账单说明](docs/BILLING.md)。首次构建需要下载依赖和容器镜像，可能耗时数分钟。

先在本地终端创建用户及初始组织，按提示输入 12–72 字节的密码。密码输入不会回显，没有默认密码；该命令不会覆盖已有用户名。

```sh
docker compose exec api admin --username admin
```

登录后选择组织，即可添加第一台服务器的资料。在“组织与成员”中可生成 7 天有效的单次邀请链接，私下分享给目标成员。设置 `XINGDU_REGISTRATION_ENABLED=true` 可启用注册；未启用时，可用同一 CLI 创建其他用户。保存连接信息不会访问 VPS，新记录会保持**待接入**状态，直至注册后的 Agent 发回实际心跳。请勿在备注中填写密码或私钥。

| 服务 | 本地地址 |
| --- | --- |
| 官网 | `http://127.0.0.1:15173` |
| Web 控制台 | `http://127.0.0.1:15173/app` |
| API | `http://127.0.0.1:18080` |
| PostgreSQL | `127.0.0.1:54329` |

所有对外映射的端口均绑定回环地址。请使用上述控制台地址访问：写请求会校验 `XINGDU_PUBLIC_ORIGIN`，默认值为 `http://127.0.0.1:15173`。示例凭据仅供本地开发使用；如果已有 `.env` 文件，请保留现有配置，不要覆盖，并补充 `XINGDU_APP_DATABASE_PASSWORD`（至少 16 位 URL 安全字符），用于独立运行账号，并新增安装 Worker 的 `XINGDU_WORKER_DATABASE_PASSWORD`。

停止预览环境并保留数据库数据：

```sh
docker compose down
```

## 开发与贡献

欢迎提交问题、使用场景、文档改进和代码贡献。较大的改动建议先通过 [Issue](https://github.com/Xingdu-App/xingdu/issues) 讨论范围。

如需在容器外开发，请安装 `go.mod` 指定的 Go 版本、Node.js 24 LTS 和 Make，然后执行：

```sh
make setup
make check
```

热更新、数据库准备和集成测试方法见 [开发指南](docs/DEVELOPMENT.md)。数据库集成测试需要专用测试数据库；未设置 `XINGDU_TEST_DATABASE_URL` 时会跳过。

| 文档 | 内容 |
| --- | --- |
| [功能状态](docs/IMPLEMENTATION-STATUS.md) | 当前能力、发布范围与实际验收证据 |
| [管理 API](docs/API.md) | 组织 API 密钥、授权范围及服务器／节点自动化 |
| [协议部署](docs/PROTOCOL-DEPLOYMENT.md) | 协议支持、TLS、Agent 要求与服务生命周期 |
| [SaaS 架构](docs/SAAS.md) | 租户边界、成员角色、邀请与 RLS |
| [开发指南](docs/DEVELOPMENT.md) | 本地环境、开发命令和 API 入口 |
| [开发计划](docs/PLAN.md) | 功能范围与阶段安排 |
| [适配器设计](docs/ADAPTERS.md) | 协议引擎与客户端适配边界 |
| [贡献说明](CONTRIBUTING.md) | 参与项目的约定 |
| [安全说明](SECURITY.md) | 漏洞报告指引 |

配套文档目前为简体中文。请勿在公开 Issue 中附上凭据、私有服务器信息或订阅令牌。

## 许可证

星渡采用 [MIT 许可证](LICENSE)，允许个人和商业用途的使用、修改与再分发。分发软件副本或重要部分时，请保留版权及许可声明。

第三方依赖与协议引擎遵循各自的许可证。托管引擎包括独立进程运行的 sing-box、Xray（含星渡的限定修复）及 TrustTunnel endpoint。上游 GPL-3.0-or-later、MPL-2.0 等许可证保持有效；星渡的 MIT 许可证不改变这些运行时的许可。见 [第三方声明](THIRD_PARTY_NOTICES.md)。
