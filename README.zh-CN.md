# 星渡 · Xingdu

**连点成网，一键抵达。**

[English](README.md) · [简体中文](README.zh-CN.md)

[![CI](https://github.com/Xingdu-App/xingdu/actions/workflows/ci.yml/badge.svg)](https://github.com/Xingdu-App/xingdu/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

星渡是一个开源的 VPS 与代理线路管理项目，致力于将服务器接入、协议部署、直连与中转线路、客户端订阅整合到一个可自托管的控制台中。

面向个人和小团队，星渡采用独立的协议引擎与客户端适配设计，让你自由选择客户端，统一管理自己的服务器与线路。

> **早期开发预览。** 当前可以在本地运行控制台；身份认证、VPS 接入、协议部署和订阅导出尚未开放。请仅在本地体验，当前版本不适合公网或生产部署。

## 现在可以体验什么

- **Web 控制台：** 响应式导航、空服务器列表，以及规划中功能的说明页面。
- **真实服务状态：** 查看 API 和数据库是否可用，连接失败时显示错误并支持重试。
- **本地 Docker 环境：** 一次启动控制台、API、PostgreSQL、数据库迁移和 Worker 进程。
- **开发基础：** Go 与 TypeScript 代码、自动化检查，以及独立的 Worker 和 Agent 程序入口。

Worker 目前尚不执行部署任务，Agent 尚不能接入或配置服务器。控制台目前提供简体中文界面。

## 我们希望实现的体验

```text
接入 VPS → 选择协议与线路 → 部署并验证 → 导入客户端
```

| 方向 | 规划能力 |
| --- | --- |
| 服务器管理 | 接入已有 Linux VPS，查看服务器健康状态 |
| 协议部署 | 通过运行时适配器生成并校验配置 |
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

启动后打开 **[http://127.0.0.1:15173](http://127.0.0.1:15173)**。首次构建需要下载依赖和容器镜像，可能耗时数分钟。

新安装的控制台会显示空服务器列表。服务器接入完成前，“添加服务器”按钮暂不可用。

| 服务 | 本地地址 |
| --- | --- |
| Web 控制台 | `http://127.0.0.1:15173` |
| API | `http://127.0.0.1:18080` |
| PostgreSQL | `127.0.0.1:54329` |

所有对外映射的端口均绑定回环地址。示例凭据仅供本地开发使用；如果已有 `.env` 文件，请保留现有配置，不要覆盖。

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
| [开发指南](docs/DEVELOPMENT.md) | 本地环境、开发命令和 API 入口 |
| [开发计划](docs/PLAN.md) | 功能范围与阶段安排 |
| [适配器设计](docs/ADAPTERS.md) | 协议引擎与客户端适配边界 |
| [贡献说明](CONTRIBUTING.md) | 参与项目的约定 |
| [安全说明](SECURITY.md) | 漏洞报告指引 |

配套文档目前为简体中文。请勿在公开 Issue 中附上凭据、私有服务器信息或订阅令牌。

## 许可证

星渡采用 [MIT 许可证](LICENSE)，允许个人和商业用途的使用、修改与再分发。分发软件副本或重要部分时，请保留版权及许可声明。

第三方依赖与协议引擎遵循各自的许可证。
