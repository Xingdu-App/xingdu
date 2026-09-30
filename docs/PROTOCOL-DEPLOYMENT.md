# 协议部署

星渡通过托管 Agent 执行固定的安装、启动与卸载任务。当前使用独立进程运行的 **sing-box 1.14.2**，不绑定 Stash 或其他客户端；客户端需自行支持所选协议。已提供节点列表与 Stash / Mihomo / Surge / Loon 等受限订阅格式，具体范围见 [订阅文档](SUBSCRIPTIONS.md)；新增配置编辑、版本恢复和单层 TCP 中转；自动证书为独立运营者进程，详见 [可靠部署](RELIABLE-DEPLOYMENTS.md) 与 [自动证书](AUTOMATIC-CERTIFICATES.md)。

## 协议与最低 Agent 版本

以下为 15 个 API 协议选项的实现矩阵，不代表全部系统或客户端均已验收。
最低版本来自 `internal/protocol.MinimumAgentVersion`；dev 标签表示能力门槛，
不是建议使用未发布构建。源码与已发布产物的区别见 [功能状态](IMPLEMENTATION-STATUS.md)。

| 协议（API 值） | 传输 / 认证 | 域名与证书 | 最低 Agent |
| --- | --- | --- | --- |
| Shadowsocks (`shadowsocks`) | TCP；chacha20-ietf-poly1305 密码 | 无需 | 0.8.0-dev |
| SS2022 (`shadowsocks2022`) | TCP；2022-blake3-aes-256-gcm 密钥 | 无需 | 0.8.0-dev |
| Trojan (`trojan`) | TCP + TLS；密码 | 自备匹配 PEM | 0.7.0-dev |
| VLESS (`vless`) | TCP + TLS；UUID | 自备匹配 PEM | 0.7.0-dev |
| VMess (`vmess`) | TCP + TLS；UUID | 自备匹配 PEM | 0.7.0-dev |
| Hysteria 2 (`hysteria2`) | QUIC / UDP + TLS；密码 | 自备匹配 PEM | 0.7.0-dev |
| TUIC v5 (`tuic`) | QUIC / UDP + TLS；UUID 与密码 | 自备匹配 PEM | 0.7.0-dev |
| AnyTLS (`anytls`) | TCP + TLS；密码 | 自备匹配 PEM | 0.10.0-dev |
| HTTPS (`http`) | TCP + TLS；用户名/密码 | 自备匹配 PEM | 0.10.0-dev |
| SOCKS5 (`socks`) | TCP；明文用户名/密码 | 无需；仅用于可信网络/加密隧道 | 0.14.0-dev |
| Mixed (`mixed`) | 同端口 HTTP CONNECT / SOCKS5；明文用户名/密码；TCP | 无需；仅用于可信网络/加密隧道 | 0.14.0-dev |
| Hysteria 1 (`hysteria`) | QUIC / UDP + TLS；auth_str | 自备匹配 PEM | 0.14.0-dev |
| ShadowTLS v3 (`shadowtls`) | TCP；外层密码 + 内层 SS2022 密钥 | 固定公共握手域名；不上传证书 | 0.14.0-dev |
| Snell v4 兼容 (`snell`) | 加密 TCP；PSK；v5 服务端 / v4 客户端 | 无需 | 0.14.0-dev |
| Snell v6 测试版 (`snell6`) | 加密 TCP；PSK；需 v6 客户端 | 无需 | 0.14.0-dev |

TCP 协议需放行所选 TCP 端口；QUIC 协议需放行所选 UDP 端口。
QUIC 是传输方式，没有独立的通用 QUIC 代理选项。Reality、WebSocket、gRPC
尚未接入。SS/SS2022 当前仅开放 TCP：运行时 UDP 会话复用无法保证逐报文
私网目标隔离，因此服务端和订阅均关闭 UDP。其他选项的限制见下文。

## 安装前提

- Linux amd64 / arm64 与 systemd；托管 Agent 已接入并在最近 90 秒内上报心跳。各协议最低版本见上表；配置更新与恢复还要求 **0.12.0-dev**。控制端按语义版本检查最低版本，较新版本可用；预检、任务创建和领取都会检查。
- 操作者为该组织的所有者或管理员。成员与只读成员可查看部署记录，不能安装、卸载或读取连接凭据。
- 控制端配置 `XINGDU_CREDENTIAL_KEY`，与现有凭据加密配置一致，密钥不得进入数据库或 Git。
- Shadowsocks 系列直接使用服务器 IP，不需要域名或证书；固定使用上表加密方式，不支持自定义插件或密码。
- TLS 协议需要已有与 TLS 域名匹配、在有效期内、适合服务器用途的 PEM 证书及匹配私钥。客户端必须正常验证证书；私有 CA 的信任需由用户明确配置，不提供跳过验证默认值。
- 选择未占用的监听端口，自行配置云安全组和机器防火墙。星渡不自动修改防火墙。

已有只读探针不会自动升级为 root 托管模式。先核实并停止旧 Agent，按 [机器接入](MACHINE-ACCESS.md) 的卸载/重新安装流程处理已有服务与状态目录，再明确选择托管模式、授权 root 权限并重新注册。安装脚本不会覆盖未知的已有 Agent。已有 Agent 可在「接入 / 状态」点击升级；支持自更新后无需 SSH 凭据，旧版保留 SSH 升级入口，详见 [网页升级](MACHINE-ACCESS.md#网页升级-agent)。

## 运行时产物

容器构建会准备 Agent 和固定版本运行时。容器外开发可执行：

```sh
make runtimes
make agents
export XINGDU_AGENT_ARTIFACT_DIR="$PWD/bin/agents"
make dev-api
```

`make runtimes` 获取固定的上游发行包，分别验证压缩包和解包后二进制的 SHA-256，准备 amd64、arm64 产物。Agent 从控制端下载匹配架构的固定运行时并验证内置摘要，不接受用户传入下载地址或任意命令。不要通过改成未知镜像、跳过摘要校验来解决下载失败。

星渡代码使用 MIT；独立运行的 sing-box 遵循其上游 GPL-3.0-or-later 许可，MIT 不覆盖上游运行时。再分发运行时时须保留并遵守上游许可，参见 [第三方声明](../THIRD_PARTY_NOTICES.md)。

## 控制台流程

1. 打开服务器的「协议部署」，确认托管 Agent 就绪。
2. 新建节点，填写名称、协议和端口；TLS 协议填写匹配的域名、证书与私钥；ShadowTLS 选择允许的公共握手域名但不上传证书，其余无 TLS 选项不填写证书。
3. 明确确认安装运行时、创建系统服务并启动端口监听。
4. 等待 Agent 领取任务，观察排队、执行、完成或失败状态。
5. 管理员主动点击「连接信息」获取服务器、端口、TLS 域名、客户端凭据和公开证书。TLS 私钥不会回显。
6. 在支持该协议的客户端配置并验证实际流量。完成状态仅证明当次任务启动检查成功，不表示持续协议健康，也不证明公网路由、DNS、安全组或客户端兼容性。

同一机器一次只执行一个部署操作。安装中断或租约过期不会自动重试；先核实机器状态再决定卸载或重新创建，避免重复执行不确定的操作。卸载失败可在状态恢复后再次请求卸载；已发出的操作无法通过关闭对话框撤回。

## 数据与撤销边界

- TLS 私钥和生成的客户端凭据在控制端以 AES-256-GCM 加密存储；AAD 绑定组织、机器和部署。服务端需解密后交付所属 Agent，这不是对控制端不可见的端到端加密。
- 部署列表仅返回元数据与执行结果，不返回凭据。连接信息接口限组织所有者/管理员，并记录审计；返回客户端凭据和公开证书，不返回 TLS 私钥。
- 机器上的运行配置必须包含服务所需密钥与凭据，以受限权限保存。当前生成的运行时配置禁用流量日志；控制端仍保留任务结果、机器心跳和审计事件，操作系统及部署平台日志取决于实际配置。
- 卸载用于停止并移除该部署的服务与配置，不移除其他部署、共享运行时或自动更改防火墙。只有完成回报后，才视为卸载完成。
- **撤销 Agent 或删除服务器资料不会停止已经部署的协议服务。** 需要终止访问时，先卸载服务并确认完成，再撤销接入；若机器已失联，应由管理员在机器上处理。
- 在线删除不能保证备份中的历史凭据立即消失。自托管部署者负责备份加密、访问控制和保留期限。

## 验证边界

`make check` 覆盖协议配置、输入验证、执行边界及数据库/API 权限测试；数据库用例需设置专用 `XINGDU_TEST_DATABASE_URL`。容器协议验收入口为 `make protocol-lab-test`；需先准备本地管理员、启动 Agent 实验室并至少运行一次 `make agent-lab-test` 创建实验机器。Cloud 模式下，实验组织需要能容纳三台机器的有效测试套餐。该命令会将三台可丢弃实验机器重新接入为当前版本的托管 Agent，测试后卸载协议服务并保留 Agent 在线；internal 网络不向宿主发布协议端口。操作方法见 [Agent 实验室](AGENT-LAB.md)。

2026-09-28 的 arm64 Docker 验收已通过：

| 系统 | Trojan / VLESS / VMess（TCP + TLS） | Hysteria 2 / TUIC v5（QUIC） | 重启恢复 / 卸载 |
| --- | --- | --- | --- |
| Ubuntu 24.04 | 三种均通过 | 两种均通过 | 通过 |
| Debian 13 | 三种均通过 | 两种均通过 | 通过 |
| Amazon Linux 2023 | 三种均通过 | 两种均通过 | 通过 |

十五个组合均验证真实 HTTP 转发、错误认证拒绝、私有 IP 及私有域名目标阻止和非 root 运行；三个系统均验证重启后再次转发、端口占用保护、卸载停止服务并删除配置。证据为 `.local/agent-lab/protocol-report.json`。测试客户端为 sing-box，不代表其他客户端应用的兼容性。容器测试不等于真实 VPS、EC2 网络、公网安全组或 amd64 的运行验收。生产发布前仍需验证证书生命周期、重启恢复、断网恢复、失联机器清理和实际公网客户端流量。

2026-09-29 补充验证：Shadowsocks 与 Shadowsocks 2022 在以上三个 arm64 系统上均通过 TCP 转发、错误密钥拒绝、私有 IP/域名拦截、非 root 运行、API 重启、容器重启恢复和卸载。已确认 UDP 不可转发；固定运行时在复用 UDP 会话时存在后续报文绕过私有目标规则的问题，因此本轮仅开放 TCP。六个组合的记录位于 `.local/agent-lab/protocol-report.json`，仍不代表 Stash / Surge App 或公网 VPS 验收。

可仅复验新协议：

```sh
python3 scripts/protocol-lab.py --protocol shadowsocks --protocol shadowsocks2022
```

## 节点与部署记录

成功安装的协议服务称为节点。控制台「节点」按当前组织汇总所有服务器上的节点，支持按名称、服务器、地址或 TLS 域名搜索，以及协议筛选。管理员可从节点详情主动查看连接信息或卸载；成员和只读成员仅能查看元数据。

`GET /api/v1/nodes` 在 PostgreSQL RLS 范围内读取，不返回凭据或 TLS 私钥。安装成功时保存 `installed_at`，等待安装和安装失败不会创建节点；卸载排队、执行中、失败或中断时保留节点，只有收到卸载成功回报才从列表移除。撤销 Agent 不会停止服务，节点仍保留。删除服务器资料会级联删除相关元数据，但不会停止机器上已有服务。

节点的「已部署」是任务结果，Agent 在线是机器心跳，均不代表协议服务当前健康或公网可达。已实现独立运营者协议探测程序，但不是默认全球探测服务；未运行探测进程时没有实连结果，详见 [外部探测](RELIABLE-DEPLOYMENTS.md#独立外部探测)。

升级迁移会将现有 `succeeded / deploy` 记录回填为节点。升级前已进入卸载流程的旧记录无法可靠证明先前安装成功，因此不会猜测回填；仍可在部署记录中查看和处理。

## Stash 扩展兼容

新增 AnyTLS 与 HTTPS 代理的托管部署及 Stash / Mihomo 导出，要求 Agent
0.10.0-dev 或更新版本；TLS 证书必填，当前仅 TCP。完整协议矩阵、后续差距
及验证边界见 [Stash 兼容说明](STASH-COMPATIBILITY.md)。

首次安装需下载并校验运行时。Agent 0.11.1-dev 将下载上限设为 8 分钟，
任务执行上限为 9 分钟，控制端租约为 10 分钟；API 下载响应上限为 9 分钟。
请先部署匹配的控制端再升级 Agent。心跳独立上报；下载仍禁止重定向，
保留大小上限和 SHA-256 校验，失败时不启用未完整验证的文件。

## 服务端版本

Agent 0.13.0-dev 起，在服务状态上报中附带 `runtime_version`，取自节点自身托管 systemd 单元指定的 sing-box 安装文件，而非当前 Agent 内置的下载版本。只读取受管路径，不执行额外命令或上传配置内容。界面显示“服务端版本”，与 Agent 版本分开；它表示节点配置的安装版本，不证明进程健康或公网连通性。

发布时先升级 API 并应用迁移 038，再升级 Agent；旧 API 不接受新增状态字段。

旧 Agent、缺失的二进制或无法确认的受管路径上报空值，界面显示“未上报”。升级 Agent 后，已有节点会在下一轮状态上报时补齐，无需重新部署。版本跟随 `service_checked_at` 的最后检查时间；离线时不代表当前运行状态。

## 扩展代理协议（Agent 0.14.0-dev）

| API 协议值 | 服务端与客户端配置 | 证书 / 传输 |
| --- | --- | --- |
| `socks` | SOCKS5，固定用户名 `xingdu` 与随机密码 | 无 TLS，明文认证，仅 TCP 转发 |
| `mixed` | 同端口 HTTP CONNECT 与 SOCKS5，固定用户名 `xingdu` | 无 TLS，明文认证，仅 TCP 转发 |
| `hysteria` | Hysteria 1，`auth_str`，上下行带宽提示均为 100 Mbps | 自备 TLS 证书，QUIC / UDP，ALPN `hysteria` |
| `shadowtls` | ShadowTLS v3 + SS2022 AES-256-GCM；独立内外层凭据 | 不上传证书；公共握手域名；仅 TCP 转发 |
| `snell` | sing-box Snell v5 服务端，客户端指定 v4 | 无需证书；加密 TCP；不启用 v5 QUIC Proxy Mode |
| `snell6` | Snell v6 测试版，默认加密流量整形 | 无需证书；仅 TCP 转发；需要兼容 v6 的客户端 |

这些协议支持托管部署、连接信息、更新/轮换、重启、卸载与单层 TCP 中转的
配置生成。旧 Agent 不能领取新增协议任务；配置更新也同时检查入口、出口
协议所需版本。SOCKS5 和 Mixed 不提供链路加密，应通过可信网络或加密隧道
使用；不要把它们当作 HTTPS 代理。客户端导出能力见订阅文档。

ShadowTLS 的 `server_name` 只能选择 `www.microsoft.com`、`www.apple.com`
或 `cloud.tencent.com`，服务器必须能访问该域名的 TCP 443。固定目标和
关闭通配 SNI 避免用户指定任意内部握手目标；不接受自定义端口、IP、证书
或运行时 JSON。启用 v3 strict mode，解密后的流量进入无独立监听端口的
SS2022 inbound，继续经过私网目标阻止规则。`credential` 是 SS2022 密钥，
`password` 是独立 ShadowTLS 密码，客户端仍正常校验握手域名的公共证书。

SOCKS5、Mixed、ShadowTLS、Snell 的 UDP 转发在服务端显式拒绝，订阅也
标记 TCP-only。Hysteria 1 的 100 Mbps 是拥塞控制参数，不是套餐限速。
Snell v6 尚处于上游测试阶段，可能有协议变动。Naive 按当前范围暂不接入；
TUN、透明代理、Cloudflare Tunnel 及 VPN endpoint 需要独立的接入模型。

实现依据：固定 [sing-box 1.14.2 入站文档](https://github.com/SagerNet/sing-box/tree/v1.14.2/docs/configuration/inbound)、
[Snell 版本说明](https://sing-box.sagernet.org/configuration/inbound/snell/)。

2026-09-29 新增协议验收：Debian 13 / arm64 的六个选项均通过托管安装、
真实 TCP 流量转发、错误凭据拒绝、私有 IP/域名拦截、非 root 运行、容器
重启恢复、端口冲突保护和卸载。Mixed 同时测试 HTTP 和 SOCKS5 入口；
ShadowTLS 分别测试错误的内层与外层凭据，握手使用隔离实验室 CA 签发的
TLS 1.3 目标，不是公共握手域名的网络可达性测试。除 Hysteria 1 外的新增
选项均实测拒绝 UDP 转发。Hysteria 1 本轮证明 QUIC 传输承载 TCP 流量，
不据此宣称完成 UDP 应用或各客户端 App 验收。

新增 Mihomo 配置通过本地官方 Mihomo 解析测试；Snell 的 Stash / Surge
导出仍是字段测试，未完成对应 App 联网验收。其他 Linux 发行版、amd64、
公网 VPS 和新增协议中转仍需分别验收，不能由此处的单机容器结果替代。

```sh
python3 scripts/protocol-lab.py --system debian --protocol socks --protocol mixed \
  --protocol hysteria --protocol shadowtls --protocol snell --protocol snell6
```

## SELinux 检测与修复

Agent 检查 `/sys/fs/selinux/enforce`，区分未启用、Permissive 与 Enforcing
主机；启用 SELinux 时，在部署、配置更新和用户主动重启之前准备专用策略。
现有节点可使用「重启服务」触发检查与修复。后台服务状态检查只读，不自动
安装软件、修改策略或重启节点。服务仍运行但没有进入专用域时，上报
`policy_required`，页面显示「安全策略待修复」并提供「修复安全策略并重启」。

策略源码随 Agent 构建嵌入，使用发行版工具编译并安装共享的
`xingdu_runtime` 模块。程序进入 `xingdu_runtime_t`，二进制与配置分别标记为
`xingdu_runtime_exec_t` 和 `xingdu_runtime_conf_t`。保留 DynamicUser、
NoNewPrivileges、只读系统目录及 capability 限制；不关闭 SELinux，不将域
设为 Permissive，不使用 `bin_t` / `unconfined_service_t`，不为 `init_t` 全局
开放网络权限。策略允许代理所需的 TCP/UDP 监听和出站、DNS、证书读取及
systemd 传入的配置描述符；不授予普通文件写入或执行 Shell 的权限。
私网和云元数据目的地仍由运行时路由规则拒绝。

工具缺失时，仅支持通过现有 dnf/yum 软件源安装 `selinux-policy-devel` 和
`policycoreutils`；不添加软件源、执行系统升级或关闭包签名检查。不支持的
工具环境明确失败。修复只重标记已确认归属的程序与当前节点配置，不递归
重标记整台机器；管理员的本地 fcontext 覆盖导致标签不一致时，返回
`selinux_label_conflict`，不删除或覆盖管理员策略。共享模块在卸载单个节点
时保留，避免影响其他节点。

失败保存固定的 `selinux_*` 结果码，包括检测、工具安装、策略编译/加载、
文件标签和服务域验证。不上报原始流量日志、审计日志、机器地址或凭据。
服务启动成功后还检查实际进程域，但这仍不等于公网客户端验收。
旧 Agent 不具备这些检查；本次代码尚须发布并升级后才能使用自动修复。

实机验收：AlmaLinux 10.2 Enforcing 上的 Shadowsocks 2022 节点，在原有
systemd 沙箱下复现出站 `name_connect` 拒绝；专用策略加载后，同沙箱下
HTTP/HTTPS 转发通过，故障节点重启后 Stash 节点测试通过。另通过编译后的
Agent 测试程序验证自动策略安装、重复修复、文件标签与进程域检查。
此结果不代表其他发行版、SELinux MLS 策略或全部协议均已实机验收。
`TestSELinuxRepairOnOwnedNode` 为显式启用的 root/SELinux 实机测试，需设置
`XINGDU_SELINUX_TEST_NODE_ID` 为管理员已授权的现有节点；测试会安装/修复
共享策略和标签，不重启节点，也不修改节点配置或凭据。普通 `make check`
默认跳过该实机用例。

参考：[Red Hat 自定义 SELinux 策略](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/10/html/using_selinux/writing-a-custom-selinux-policy)、
[SELinux NoNewPrivileges 域转换](https://github.com/SELinuxProject/selinux-notebook/blob/main/src/object_classes_permissions.md)。
