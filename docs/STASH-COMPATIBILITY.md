# Stash 协议兼容范围

对照日期：2026-09-29。客户端能连接某种协议，不等于星渡已具备该协议的
服务端安装、凭据生成、重启、卸载和订阅导出能力。星渡保持独立运行时和
客户端适配器，不引入 Stash 的私有实现作为服务端依赖。

## 已实现

| 协议 | 星渡服务端 | Stash 导出 | 传输与前提 |
| --- | --- | --- | --- |
| Shadowsocks / SS2022 | sing-box | 独立 YAML 字段 | TCP；无须域名证书；UDP 暂未开放 |
| Trojan / VLESS / VMess | sing-box | 独立 YAML 字段 | TCP + TLS；提供匹配证书与私钥 |
| Hysteria 2 / TUIC v5 | sing-box | 独立 YAML 字段 | QUIC + TLS；需要 UDP 端口可达 |
| AnyTLS | sing-box | `type: anytls`、密码、SNI、证书指纹 | Agent 0.10.0-dev 起；本轮仅 TCP |
| HTTPS 代理 | sing-box HTTP inbound + TLS | `type: http`、`tls: true`、用户名/密码 | Agent 0.10.0-dev 起；仅 TCP；用户名为 `xingdu` |

新增协议也可导出为 Mihomo。Surge、Loon 与 HY2 URI 对新增协议明确拒绝，
不套用其他客户端语法，不静默剔除节点。TLS 私钥不进入订阅；客户端收到
叶证书 SHA-256 指纹，不关闭服务端身份验证。AnyTLS 的 UDP 在服务端拒绝，
不会仅靠客户端的 `udp: false` 作为网络隔离措施。

新协议不会发给不支持的旧 Agent。旧协议保留各自的最低版本要求；控制台
与后端按协议检查版本，升级后仍以真实心跳确认，而非仅凭下载完成。

## 后续兼容项

| Stash 能力 | 星渡尚缺的部分 |
| --- | --- |
| VLESS Vision / Reality | 独立密钥、short ID、握手目标限制、TLS 模式与导出字段；不能把普通 TLS 节点标成 Reality |
| WebSocket / gRPC / HTTP2 / XHTTP | 传输参数、路径校验和运行时支持矩阵；协议类型与承载网络分开建模 |
| Snell | 固定运行时的版本/授权核查，以及独立服务端与客户端握手验收 |
| Mieru / TrustTunnel / Juicity | 固定运行时或独立运行时适配、校验和、安装/卸载及安全出口验证 |
| WireGuard / Tailscale / MASQUE | 密钥、路由、权限和生命周期不同，需要专用接入模型 |
| SOCKS5 / 明文 HTTP | 不直接开放公网明文凭据；本轮优先提供 HTTPS |
| Hysteria 1 / ShadowsocksR | 旧协议维护与运行时适配成本较高，现阶段优先 HY2 与 AEAD SS |
| SS 插件 / UDP over TCP / QUIC 混淆 | 需要客户端、服务端一致的参数与逐包目标隔离测试 |

## 验证边界

配置测试检查认证字段、证书固定、无 TLS 私钥泄漏、UDP 限制及不兼容格式
拒绝。`scripts/protocol-lab.py --protocol anytls --protocol http` 用隔离的
Linux 容器验证实际转发、错误密码拒绝、私网地址与私网域名拦截、服务重启
和卸载。容器内 sing-box 客户端验证不等于发布版 Stash App 导入/联网验证，
也不等于公网 VPS、安全组或所有架构已验收。

依据：[Stash 官方协议格式](https://stash.wiki/proxy-protocols/proxy-types)、
[sing-box AnyTLS inbound](https://sing-box.sagernet.org/configuration/inbound/anytls/)、
[sing-box HTTP inbound](https://sing-box.sagernet.org/configuration/inbound/http/)、
[Mihomo AnyTLS](https://wiki.metacubex.one/en/config/proxies/anytls/)。

2026-09-29：AnyTLS / HTTPS 在 Ubuntu 24.04、Debian 13、Amazon Linux 2023
的 arm64 容器上共六组验收通过：真实 TCP 转发、错误密码拒绝、私网 IP/
域名拦截、非 root 运行、容器重启恢复、端口冲突保护与卸载。测试后恢复
本地实验组织原有计费状态。该结果不涵盖发布版 Stash App 或真实 VPS。
