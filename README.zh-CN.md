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

> **早期开发预览。** 当前可以在本地运行控制台；现已支持多用户登录、组织切换、成员权限、邀请链接与隔离的服务器资料管理；已实现 Agent 注册、探针和 SSH 安装流程；托管 Agent 的协议部署已实现，已实现受协议与证书限制的客户端导出、节点重启与状态、账户安全及配额/审计/备份工具，线路编排尚未开放。请仅在本地体验，当前版本不适合公网或生产部署。

## 现在可以体验什么

- **服务器资料：** 新增、编辑和删除服务器记录，管理地址、SSH 连接信息、标签和备注。
- **组织协作：** 创建与切换组织，邀请成员，分配所有者、管理员、成员和只读权限。
- **租户隔离：** PostgreSQL 行级安全策略与独立低权限运行账号。
- **用户登录：** 可选公开注册、本地创建账号、可撤销会话，以及受保护的管理接口。
- **机器接入：** 主动安装 Agent，或通过 SSH 密码 / 私钥安装；校验主机指纹，可选长期加密保存凭据。配置与验收边界见 [机器接入](docs/MACHINE-ACCESS.md)。
- **节点列表：** 按组织汇总已成功部署的节点，支持搜索、协议筛选与节点详情；卸载成功后移出列表，操作结果保留在部署记录中。
- **协议部署：** 托管 Agent 可安装和卸载 Trojan、VLESS、VMess、Hysteria 2、TUIC v5；使用用户提供的 TLS 证书，查看执行进度，由管理员主动获取连接凭据。前提与验收边界见 [协议部署](docs/PROTOCOL-DEPLOYMENT.md)。
- **真实服务状态：** 查看 API 和数据库是否可用，连接失败时显示错误并支持重试。
- **基础订阅：** 选择已部署节点、添加有序的域名/CIDR 规则，生成可重置或停用的 Stash（默认）、Mihomo、Surge 或 Loon 配置链接，或 Hysteria 2 URI；不同格式有明确的协议与证书限制。权限和客户端兼容边界见[订阅说明](docs/SUBSCRIPTIONS.md)。
- **本地 Docker 环境：** 一次启动控制台、API、PostgreSQL、数据库迁移和 Worker 进程。
- **开发基础：** Go 与 TypeScript 代码、自动化检查，以及独立的 Worker 和 Agent 程序入口。

Worker 已执行 SSH Agent 安装任务，Agent 可以注册、上报状态并执行固定的协议安装与卸载任务；不开放任意远程命令，已在 [Docker 实验室](docs/AGENT-LAB.md) 验证 Ubuntu 24.04、Debian 13、Amazon Linux 2023（arm64）的 systemd 安装；真实 VPS/EC2 验收仍待完成。五种协议也已在上述三个 arm64 Docker 系统中使用 sing-box 客户端验证实际转发、错误认证拒绝、私有目标阻止、重启恢复与卸载；不代表具体客户端应用或真实 VPS/EC2 的兼容性。控制台目前提供简体中文界面。

## 我们希望实现的体验

```text
接入 VPS → 选择协议与线路 → 部署并验证 → 导入客户端
```

| 方向 | 规划能力 |
| --- | --- |
| 服务器管理 | 接入已有 Linux VPS，查看服务器健康状态 |
| 协议部署 | 扩展传输方式与证书生命周期管理 |
| 线路管理 | 直连与单层中转线路 |
| 可靠变更 | 版本化部署、进度追踪、失败重试与配置回滚 |
| 客户端订阅 | 为 Stash、Surge、Loon、Shadowrocket 分别提供配置输出 |

以上为开发路线，尚不是当前预览版的可用功能。协议和客户端版本的兼容范围将随实际验证逐步公布。阶段安排见 [开发计划](docs/PLAN.md)。

## 本地体验

需要安装 **Git、Docker 和 Docker Compose v2**。使用容器运行预览版，无须在本机安装 Go 或 Node.js。

```sh
git clone https://github.com/Xingdu-App/xingdu.git
cd xingdu
cp .env.example .env
docker compose up --build -d --wait
```

启动后打开官网 **[http://127.0.0.1:15173](http://127.0.0.1:15173)**，控制台入口为 **[/app](http://127.0.0.1:15173/app)**。官网提供价格、隐私和安全说明；托管版价格尚未公布。首次构建需要下载依赖和容器镜像，可能耗时数分钟。

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

第三方依赖与协议引擎遵循各自的许可证。当前运行时为独立进程运行的 sing-box 1.14.2，遵循其上游 GPL-3.0-or-later 许可证；星渡的 MIT 许可证不改变该运行时的许可。见 [第三方声明](THIRD_PARTY_NOTICES.md)。

当前六项开发的实现、验证与剩余工作见[交付状态](docs/IMPLEMENTATION-STATUS.md)。
