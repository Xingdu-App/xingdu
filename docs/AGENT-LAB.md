# Docker Agent 接入验收

本地实验室使用 Ubuntu 24.04、Debian 13、Amazon Linux 2023 官方基础镜像，安装真实的 OpenSSH 和 systemd。它复用本地控制台及组织身份，经真实 HTTPS、SSH、数据库队列和 Worker 安装 Agent。不会连接 EC2 或真实 VPS。

## 启动

前提：本地 `make up` 已完成，`.local/initial-admin.json` 中有可用的管理员凭据，`.env` 中已配置 `XINGDU_CREDENTIAL_KEY` 和独立 Worker 数据库密码；主机有 Docker Compose、Python 3、OpenSSL（支持 `rsa -traditional`）、ssh-keygen。

```sh
make agent-lab-up
make agent-lab-test
```

首次构建需要下载系统包。日志、随机 SSH 密码、加密私钥、私钥口令、短期测试 CA 和验收报告均在 Git 忽略的 `.local/agent-lab/`，目录权限 0700，秘密文件 0600。不要把整个目录发给别人。测试脚本不输出密码、注册令牌或机器身份。

验收结束后，控制台会保留三台在线机器，标签为 `agent-lab`。Ubuntu/Debian 使用探针模式，Amazon Linux 使用托管模式，用于验证明确授权的 root 服务运行方式；托管模式还可执行明确授权的协议安装与卸载。后续运行协议验收会将三台实验机器均重新接入为托管模式。

重复运行 `agent-lab-test` 会撤销、卸载并重新安装**这三台实验机器**。不要把这些容器用于其他工作。脚本只删除固定测试容器内的 Agent 文件，不处理任意主机地址。

## 测试内容

1. 三个系统均通过控制端生成的 HTTPS 安装脚本注册，校验二进制，启用 systemd 探针服务。
2. 验证服务用户、0600 身份文件、NoNewPrivileges、空 capabilities 及连续心跳。
3. 重启容器，验证 systemd 自动恢复 Agent。
4. 暂停实验室 TLS 入口，验证超过 90 秒后控制台离线；恢复入口后，无须重新注册即可继续心跳。
5. 撤销身份，验证服务以 0 状态码退出且不进入重启循环。
6. 清理已撤销的安装，Ubuntu/Debian 经密码、Amazon Linux 经带口令的 PEM 私钥，通过 API → Worker → SSH 完成安装。指纹由容器内的主机公钥独立核验。Debian 额外验证可选的长期加密保存，以及使用已保存凭据重新安装。

结构化结果在 `.local/agent-lab/report.json`。失败不会被跳过或标记成功，保留现场供排查；不要将整个 `.local` 目录上传。

安装流程始终在 `/tmp noexec` 下运行，下载/SSH 包使用 `/usr/local/bin/` 下 0700 的随机临时目录，退出时清理。Agent 服务依赖 `network.target`，网络未就绪时由 Agent 重试，不因发行版的 wait-online 服务阻塞启动。

## 隔离与限制

- Compose overlay 只用于本地实验室，不并入生产默认配置。
- 测试网段为 `172.30.88.0/24`，仅 API/Worker 的实验室配置允许访问此网段；Docker 若检测到网段冲突会拒绝启动。
- 三个目标容器只加入 internal 网络，没有发布 SSH 端口，没有宿主目录、Docker socket、宿主 PID/network/cgroup namespace 挂载。
- systemd 测试需要 privileged 容器，使用**私有 cgroup namespace** 和容器自己的可写 cgroup 挂载。请只在专用开发 Docker 环境使用；容器特权不等于生产 Agent 的权限，探针服务本身仍使用受限用户及 systemd 沙箱。
- HTTPS 域名 `control.xingdu.test` 只在实验室网络内解析。CA 只安装到目标容器，不修改 macOS/浏览器信任，不使用 `curl -k` 或跳过证书验证。默认有效期 30 天，过期前清理实验室后删除本地证书并重新生成。
- overlay 会临时改变本地 API/Worker 的 Agent origin。实验室运行期间生成的安装命令只供这些容器使用。结束后用下面的清理命令恢复。
- 容器共享 Docker VM 的内核；CPU、内存、运行时间等 `/proc` 指标可能反映 VM 而非容器限额。不能把这些数值当成独立 VPS 的资源。
- arm64 Docker 上的通过结果不代表 amd64 的执行验收，也不覆盖 EC2 IMDS、AMI 初始化、云防火墙、独立内核启动和真实断网。真实 VPS 的验收仍需单独完成。

## 清理

```sh
make agent-lab-cleanup
```

清理只撤销并删除本次实验室登记的机器、删除四个实验容器，再从基础 Compose 配置恢复 API/Worker/Web。不会删除 PostgreSQL 卷或其他服务器。镜像、测试 CA 和本地凭据保留，便于再次运行；确认不需要后可自行删除 `.local/agent-lab/`。

不要使用 `docker compose down -v` 清理实验室，它会影响控制台数据库。

参考：[Amazon Linux 2023 官方容器说明](https://docs.aws.amazon.com/linux/al2023/ug/base-container.html)。

## 已完成的验收记录

2026-09-28，OrbStack / aarch64（arm64），三个镜像均运行真实 systemd 和 OpenSSH：

| 系统 | 主动安装 / monitor | SSH 安装 | 重启恢复 | 断联恢复 / 撤销 |
| --- | --- | --- | --- | --- |
| Ubuntu 24.04 | 通过 | 密码 / monitor 通过 | 通过 | 通过 |
| Debian 13 | 通过 | 密码 / monitor 通过；保存凭据复用通过 | 通过 | 通过 |
| Amazon Linux 2023 | 通过 | 加密 PEM / manage 通过 | 通过 | 通过 |

三个系统均保留 `/tmp noexec`；检查了 systemd 运行用户、实际进程 UID、探针进程的有效 capabilities 为零、身份文件权限及连续心跳。通过的是本地 arm64 容器验收，不包含真实 VPS/EC2 或 amd64 运行验收。

## 协议部署与转发验收

前提：先按上文启动本地控制端并准备 `.local/initial-admin.json`，运行 `make agent-lab-up`，再至少运行一次 `make agent-lab-test`，创建三台已知实验机器的 `fixtures.json`。随后执行：

```sh
make protocol-lab-test
```

此命令仅操作上述可丢弃的实验容器，会撤销旧身份、重新安装 **0.5.0-dev 托管 Agent** 并测试五种协议。重跑时会先卸载实验机器上的已有协议部署。不要在这些容器保存其他工作；该流程不适用于真实 VPS。全部测试成功后会卸载本轮协议服务，三台托管 Agent 保持在线。协议监听和测试目标均位于 internal 网络，没有向宿主发布协议端口，也不向公网目标发送测试流量。

2026-09-28，arm64 Docker / sing-box 1.14.2 实测：

| 系统 | Trojan TCP/TLS | VLESS TCP/TLS | VMess TCP/TLS | Hysteria 2 QUIC | TUIC v5 QUIC |
| --- | --- | --- | --- | --- | --- |
| Ubuntu 24.04 | 通过 | 通过 | 通过 | 通过 | 通过 |
| Debian 13 | 通过 | 通过 | 通过 | 通过 | 通过 |
| Amazon Linux 2023 | 通过 | 通过 | 通过 | 通过 | 通过 |

每个组合均完成真实 HTTP 转发，并验证错误认证被拒绝、私有 IP 与解析到私有地址的域名被阻止、运行时为非 root 用户。三个系统均验证容器重启后五种服务恢复转发、端口占用时拒绝安装且不替换原服务、卸载后服务停止、配置移除且 API 不再返回连接凭据。

结构化结果为 `.local/agent-lab/protocol-report.json`，不包含连接秘密；私钥和其他实验文件仍需保密。验证使用实验室 TLS 信任和 sing-box 客户端，不代表 Stash、Surge、Loon、Shadowrocket 的具体版本兼容性，也不代表真实 VPS、EC2 或 amd64 的运行验收。完整范围与操作边界见 [协议部署](PROTOCOL-DEPLOYMENT.md)。
