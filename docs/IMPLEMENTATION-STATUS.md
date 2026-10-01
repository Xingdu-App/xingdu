# 功能实现与验收状态

对照日期：2026-09-30。此页记录当前源码的能力与验收边界，作为 README、
开发计划和专题文档的状态索引。未合并的开发中功能不计入交付。

## 如何理解状态

- **已实现**：源码包含完整操作路径，不等于已在云端部署或所有环境可用。
- **已发布**：对应 tag 的构建产物已发布；不能由当前 VERSION 推断 tag 包含后续提交。
- **已验收**：仅适用于记录的系统、架构、协议、客户端版本和操作范围。

控制端版本、Agent 版本、节点运行时版本分别管理。当前 VERSION 为
`0.14.0-rc.2`；此前 `0.14.0-rc.1` tag 后的 SELinux 修复需要构建新版并升级，不能
认为已有同版本机器自动获得修复。安装或升级成功还须由新心跳确认。
发布流程与下载验证见 [Agent 发布](AGENT-RELEASE.md)。

## 当前能力

| 领域 | 已实现 | 剩余边界 |
| --- | --- | --- |
| 账户与组织 | 邮箱验证注册、密码找回、Google/GitHub 登录与绑定、会话撤销、邀请、成员权限、所有权转移、组织 RLS | Passkey、MFA、组织销毁及远端清理；实际邮件/OAuth 配置需部署验收 |
| 机器管理 | 服务器资料、主动 Agent 接入、默认托管安装、SSH 指纹固定、可选加密保存凭据、遥测、受控 Agent 升级及失败恢复 | 不执行任意命令；撤销身份不会停止已有服务；各发行版需单独验收 |
| 协议与节点 | 16 个协议选项（TrustTunnel 为独立 HTTP/2 runtime）、预检、安装/卸载、重启、本机状态、运行时版本、SELinux 检测和受控修复 | 不是全部客户端组合均已验收；防火墙/安全组不自动修改；Reality/WS/gRPC 未接入 |
| 配置生命周期 | 端口/证书编辑、凭据轮换、加密修订、启动失败恢复、历史版本恢复 | 结果不确定时需人工核实；不代表协议引擎升级或公网故障恢复已验收 |
| 线路与探测 | 直连、已有节点单层 TCP 中转、依赖保护、独立组织级协议探测进程 | 无多跳、UDP 中转、默认全球探测、告警通知投递和自动最优线路 |
| 证书 | ACME DNS-01 / Cloudflare 签发与续期程序、付费组织证书管理、CNAME 委托、随机域名、持久化续期及手动节点应用 | 已完成本地模拟与 RLS 验证；真实 DNS/公共 CA/节点应用待验收；无 ARI、CF OAuth 和自动节点应用，DNS 退役需人工维护 |
| 订阅与客户端 | Stash/Mihomo/Surge/Loon 和 HY2 URI 的受限导出、令牌管理、自定义规则、规则模板及多策略组路由方案 | 各格式拒绝不兼容组合；Shadowrocket 无专用输出；远程规则由客户端拉取 |
| SaaS 与运维 | Starter/Premium 组织级 Stripe 计费、配额、API Key、审计、加密备份/隔离恢复、CI、Agent 校验和及 OIDC 来源证明 | 企业定制需联系；真实支付/生产恢复须验收；更新器尚不自动验证来源证明；密钥轮换、分布式限流和备份调度待完善 |

## 已记录的验证证据

- PostgreSQL 非 owner 角色的跨租户访问、配额并发、密码/会话隔离及所有权转移一致性测试。
- Ubuntu 24.04、Debian 13、Amazon Linux 2023 的 arm64 容器：五种基础 TLS 协议、SS/SS2022、AnyTLS/HTTPS 的转发、错误认证拒绝、私网拦截、重启与卸载。
- Debian 13 / arm64 容器：新增 SOCKS5、Mixed、Hysteria 1、ShadowTLS、Snell 两种选项的单机测试；不推广到所有发行版或 App。
- 隔离容器及 Android 客户端的 Shadowsocks TCP 中转、配置更新、凭据轮换与历史恢复；Android 其他协议组合仍待验收。
- AlmaLinux 10.2 / amd64 / SELinux Enforcing 的 SS2022 实机故障复现与修复；同沙箱 HTTP/HTTPS 转发、Agent 策略安装/标签/进程域检查，以及 Stash macOS 4.3.0 节点测试通过。该记录不覆盖完整订阅导入、全部协议或其他 SELinux 策略。
- 导出字段与拒绝规则测试、部分官方 Mihomo 解析测试；不能作为 Surge/Loon/Stash 全量 App 验收证据。
- 加密备份篡改/截断拒绝、一次性测试库恢复；生产全库及灾难恢复仍需演练。

这些是已有验证记录，不表示本次文档更新重新执行了上述测试。

## 下一步验收

1. 发布包含 SELinux 修复的新 Agent，核对部署产物、下载端点和目标机器心跳。
2. 验收真实 DNS/公共 CA 签发续期、客户端订阅刷新和中转依赖下的证书维护。
3. 按客户端/OS/协议补齐实际导入、认证失败、证书拒绝、转发、更新与撤销测试。
4. 演练公网断网与升级恢复、生产备份恢复，核查真实邮件、OAuth 和支付流程。

操作说明见 [机器接入](MACHINE-ACCESS.md)、[协议部署](PROTOCOL-DEPLOYMENT.md)、
[节点生命周期](NODE-LIFECYCLE.md)、[可靠部署](RELIABLE-DEPLOYMENTS.md)、
[订阅](SUBSCRIPTIONS.md)、[自动证书](AUTOMATIC-CERTIFICATES.md)、
[账单](BILLING.md)及 [Zeabur 部署](ZEABUR.md)。

### TrustTunnel 开发状态（2026-10-01）

源码新增 TrustTunnel endpoint 1.1.0（HTTP/2）、独立运行时下载与摘要校验、
节点生命周期、证书管理和 Stash 订阅导出。最低 Agent 0.15.0-dev，尚未发布 Agent。
仅新增客户端协议适配，不依赖 Stash 服务端组件。HTTP/3、中转、自动探测和 SELinux
主机暂不支持；实际验收脚本和范围见 [协议部署](PROTOCOL-DEPLOYMENT.md#trusttunnel)。

本地验收已通过：`make check`、Docker Compose 构建与健康检查，以及 Ubuntu 24.04
arm64 官方 endpoint ↔ macOS Stash Core CLI 的真实协议测试。显式/默认 h2、TCP、
UDP、错误密码/指纹/ALPN 拒绝、私网 IP/域名/UDP 与平台地址阻断、重启、密码轮换、
历史修订恢复和删除均通过。Linux 容器另执行了真实 BPF 查询回归；临时节点、订阅、
平台地址别名和测试额度已清理。未测试手机 App、公网 VPS、HTTP/3 或 amd64 转发。
