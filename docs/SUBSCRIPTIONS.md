# 节点订阅与路由配置

订阅把当前组织的已部署节点与一组有序规则组合成客户端配置。每次客户端拉取链接都会重新生成配置，因此编辑节点选择和规则不需要重新分发链接。

## 使用

1. 在“节点”确认至少有一个部署成功的节点。
2. 打开“订阅”，创建订阅，选择客户端格式（默认 Stash）并选择节点。
3. 可选择内置路由方案、配置策略组与节点分配，再按需添加精确域名、域名后缀或 IPv4/IPv6 CIDR 规则。自定义规则先按顺序匹配，随后匹配模板规则集，最后执行默认策略；HY2 URI 不支持分流与策略组。
4. 保存后，组织 owner/admin 可以随时在订阅卡片查看、复制链接或展开二维码，刷新页面后仍然可用。按订阅保存的客户端格式添加远程配置；Stash 和 Surge 另提供打开客户端的导入按钮。二维码在浏览器本地生成，不发送给第三方服务。

未使用 `routing` 模板的订阅导出 `Xingdu` 选择组；使用路由模板时按保存的名称与模式输出默认组和分类组，支持手动选择、延迟选择或故障切换。默认组包含全部选中的可用节点，详见 [路由模板](SUBSCRIPTION-TEMPLATES.md)。节点输出优先使用原名称，不附带内部 ID。重名、与内置策略同名或 INI 字符清理后重名时，仅添加短后缀（如 `Tokyo - 2`），策略组引用使用相同名称；调整节点顺序可能改变重名后缀。节点必须处于部署成功状态才会导出；已卸载或正在变更的节点不会出现在新配置中。所有节点均不可用时，接口明确返回错误，不会生成悄悄改为全直连的配置。

服务器地址必须能从客户端访问。本地 Docker 实验中的容器地址无法直接从外部设备访问；开发环境的回环订阅地址也仅能在当前机器使用。

## 支持范围

当前提供 Stash / Mihomo 完整 YAML、Surge / Loon 配置及 HY2 URI，各自的协议支持与拒绝范围见下方矩阵及 [Stash 扩展兼容](STASH-COMPATIBILITY.md)。配置显式包含协议凭据；Stash / Mihomo 的普通 TLS 节点另外包含 TLS 名称与服务端叶证书 SHA-256 指纹，不关闭证书校验，不导出服务端 TLS 私钥。证书更换后客户端需要更新订阅。

这里的指纹是证书固定（pinning），客户端只接受订阅指定的服务端叶证书。导出前会检查证书有效期、域名和部署配置。实现依据 [Mihomo TLS 文档](https://wiki.metacubex.one/en/config/proxies/tls/)、[协议字段](https://wiki.metacubex.one/en/config/proxies/)和[规则文档](https://wiki.metacubex.one/en/config/rules/)。旧版 Clash 和不同 App 的字段支持不同；Surge、Loon 和分享链接的明确范围见下方适配矩阵，Shadowrocket 尚待验收。

组织规则模板按复制语义复用，完整路由方案使用保存的版本化 preset 和策略组。远程规则由客户端直接拉取允许的社区规则源，API 不提供规则镜像或上游内容快照。不提供订阅用户独立凭据或流量配额；SaaS 组织计费另见 [账单](BILLING.md)。规则最多 100 条，节点最多 100 个。域名规则使用普通 ASCII 域名（国际化域名请使用 Punycode），不接受通配符；“域名后缀”本身会匹配子域。

## 权限与链接

组织 owner/admin 可以创建、编辑、删除和重置订阅链接，并通过登录后的网页随时查看完整地址。普通成员和 API Key 列表请求只返回元数据，不返回链接或节点凭据。订阅采用随机令牌：数据库保存用于认证的哈希，以及使用控制端加密密钥加密的令牌副本；创建和重置时两者在同一事务中更新。链接持有者无需登录即可获取配置。请把链接当作密码，不要放进公开 Issue、截图或聊天记录。

旧订阅若只保存过哈希，无法恢复原链接。页面会提示管理员手动重置一次；升级不会自动重置或使旧链接失效。加密密钥不可用时，页面明确提示链接无法读取。

导入按钮遵循 [Stash URL Scheme](https://stash.wiki/en/faq/url-schema) 和 [Surge URL Scheme](https://manual.nssurge.com/tools/url-scheme.html)，将订阅 URL 编码后交给客户端。其他格式提供复制和二维码入口，不猜测导入协议。点击按钮只表示尝试打开客户端，尚未完成真实 App 导入验收。

重置链接使旧链接立即失效；停用或删除订阅会阻止后续拉取。**这不会撤销客户端已下载的节点凭据，也不会断开已有连接。** 若要撤销已泄露的节点访问，可在节点配置编辑中轮换凭据，等待更新成功并让客户端刷新订阅；也可卸载相应节点。新凭据生效不代表已有连接立即中断。恢复含旧凭据的历史修订会重新启用旧凭据，见 [配置恢复](RELIABLE-DEPLOYMENTS.md#编辑与恢复)。基础订阅使用所选节点已有的共同凭据，尚无每位订阅用户独立凭据。

部署在反向代理后时，必须使用 HTTPS，并保证代理/CDN/访问日志不记录订阅查询参数。应用的订阅响应禁止缓存；应用日志不会记录链接令牌。Xingdu Cloud 已正式上线；客户端兼容范围仍需按下文的验证矩阵区分。

## 验证

导出器测试覆盖七种协议字段、证书指纹、服务端私钥排除、IPv6、重复节点名、规则顺序与注入拒绝。生成的七协议配置已通过官方 Mihomo v1.19.31（macOS arm64）的 `-t` 配置校验；这仅证明该内核版本接受配置，并不等同于所有客户端 App 的导入及联网验收。

可选的真实内核校验：自行从 Mihomo 官方发布页下载并核验其 SHA-256 后，将 `XINGDU_TEST_MIHOMO_BINARY` 指向本地二进制，运行 `go test -v ./internal/subscription`。该测试只校验临时生成的配置，不启动监听或连接代理服务器。

## Local acceptance

`make check` covers organization/RLS boundaries, token lifecycle and configuration rendering. To additionally validate generated fixtures with a locally installed official Mihomo binary, set `XINGDU_TEST_MIHOMO_BINARY` to its absolute path when running the checks.

With the disposable Agent Lab running and at least one successfully deployed lab node, run `python3 scripts/subscription-lab.py`. It creates and cleans up a temporary subscription, exercises export/edit/rotation/disable/delete through the running HTTP service, and checks proxy logs for token leakage. Set the same optional binary variable to parse the live exported configuration. Reports contain no credentials and are saved under ignored `.local/subscription-lab/`. This does not prove client-app interoperability or public VPS reachability.


## Stash 格式

新建订阅默认选择 Stash，可在创建或编辑时切换为 Mihomo。现有订阅迁移时保留 Mihomo；旧版链接显式 `format=mihomo` 或 `format=clash` 的输出不变。新生成的链接使用订阅保存的格式；旧链接需切换格式时可重置链接（旧令牌失效），或由链接持有者显式改为 `format=stash`。

Stash 按[官方协议文档](https://stash.wiki/proxy-protocols/proxy-types)独立适配：Hysteria 2 使用 `auth`、TUIC 明确 `version: 5`、TLS 使用 `server-cert-fingerprint` 和 `sni`。支持 Shadowsocks / Shadowsocks 2022 和五种 TLS 节点，保留有序规则与策略组，不关闭证书验证。未支持的客户端格式会返回 422，不会静默当作其他客户端输出。

Stash 字段映射、格式持久化和真实 HTTP 订阅已验证；另记录了 AlmaLinux 10.2 / amd64 / SELinux Enforcing 的 SS2022 修复后 Stash macOS 4.3.0 节点测试通过。此项不等于全部订阅导入、转发行为与协议组合完成 App 验收。Mihomo 解析不能作为 Stash App 验收证据。

## 基础协议客户端矩阵（2026-09-30）

| 格式 | SS / SS2022 | Trojan | VLESS | VMess | HY2 | TUIC v5 | TLS 信任 | 验证层级 |
|---|---|---|---|---|---|---|---|---|
| Stash | 输出 | 输出 | 输出 | 输出 | 输出 | 输出 | 固定叶证书 SHA-256 | 字段单测；SS2022 实机节点测试；其他组合待验收 |
| Mihomo | 输出 | 输出 | 输出 | 输出 | 输出 | 输出 | 固定叶证书 SHA-256 | v1.19.31 解析；App 联网待验收 |
| Surge | 输出 | 输出 | 拒绝 | 输出 | 输出 | 输出 | 固定叶证书 SHA-256 | 生成器单测；真实解析/联网待验收 |
| Loon | 拒绝 | 输出 | 输出 | 输出 | 输出 | 拒绝 | 明确选择系统 CA；完整受信任链 | 生成器/隔离 CA 测试；App 待验收 |
| HY2 分享链接 | 拒绝 | 拒绝 | 拒绝 | 拒绝 | 输出 | 拒绝 | `pinSHA256` + `insecure=0` | URI 解析单测；App 待验收 |
| Shadowrocket | 待验收 | 待验收 | 待验收 | 待验收 | 待验收 | 待验收 | 尚未确认导入后证书固定语义 | 暂不提供专用输出 |

Shadowsocks 系列当前仅提供 TCP，服务端与导出配置均关闭 UDP（运行时的 UDP 私有目标隔离尚待补齐）。不使用 TLS，因此不会生成 SNI、证书指纹或跳过证书验证字段。Stash / Mihomo 使用 `type: ss`、`cipher`、`password`、`udp: false`；Surge 使用 `ss`、`encrypt-method`、`password`、`udp-relay=false`，依据 [Surge 官方 Shadowsocks 文档](https://manual.nssurge.com/policies/shadowsocks.html)。2022 采用标准 Base64 编码的 32 字节 AES-256 密钥。当前 Loon 的 Shadowsocks 适配尚未实现，不代表 Loon 客户端不支持该协议。

本表的“输出”只表示适配器已实现，不表示 App 端已经通过真实连接测试。Surge 的 HY2 要求至少 iOS 5.8.0 / Mac 5.4.0；其他协议应使用支持对应协议及证书固定字段的当前版本，最低版本尚未完成矩阵验收。Loon 不猜测未在官方文档中定义的 pinning 字段；选择该格式即明确采用系统 CA 验证，导出时校验证书域名、有效期、链和系统根。私有 CA、自签名、缺少中间证书的节点会明确报错。客户端与控制平面的根证书集可能不同，仍需 App 验收。

Surge 采用独立 INI 适配：VMess AEAD + TLS、TUIC v5 的 UUID/密码分别输出。不支持的节点不会被静默剔除，整个输出返回 422。INI 名称会替换语法分隔字符，仅在名称冲突时添加短后缀，避免配置注入和名称冲突。Loon 格式不能通过旧链接的 `format` 参数自动切换信任模式，必须由管理员编辑订阅明确选择。

分享链接只支持有官方 URI pinning 规范的 Hysteria 2。必须没有分流规则且默认走节点，否则拒绝导出，不丢弃既有路由语义。链接包含节点凭据，应视同密码。

依据官方来源：

- [Surge 协议总览](https://manual.nssurge.com/policies/overview.html)、[TLS 固定证书](https://manual.nssurge.com/policies/tls.html)、[VMess](https://manual.nssurge.com/policies/vmess.html)、[TUIC](https://manual.nssurge.com/policies/tuic.html)。
- [Loon 节点语法](https://github.com/Loon0x00/LoonManual/blob/master/docs/cn/node.md)、[策略组](https://github.com/Loon0x00/LoonManual/blob/master/docs/cn/policygroup.md)。
- [Hysteria 2 URI 标准](https://hysteria.network/docs/developers/URI-Scheme/)。

待验收操作：使用一次性测试订阅和隔离网络，逐个记录客户端/OS 版本、导入结果、TLS 错误证书拒绝、认证失败、TCP/UDP 转发、订阅更新和撤销行为；不要覆盖用户日常代理配置。未完成这些步骤前，不对外宣称对应客户端全量兼容。

## Stash 扩展兼容

新增 AnyTLS 与 HTTPS 代理的托管部署及 Stash / Mihomo 导出，要求 Agent
0.10.0-dev 或更新版本；TLS 证书必填，当前仅 TCP。完整协议矩阵、后续差距
及验证边界见 [Stash 兼容说明](STASH-COMPATIBILITY.md)。

## 规则模板

在订阅页面的「规则模板」创建、编辑或删除模板。模板包含名称、有序分流规则及
未匹配时的默认连接方式。组织所有者和管理员可管理，其他成员只读；不同组织
通过 PostgreSQL FORCE RLS 隔离。每个组织最多 100 个模板，每个模板最多 100 条规则。

新建或编辑订阅时选择模板，确认替换草稿规则与默认方式，再保存订阅后生效。
节点选择、客户端格式和订阅链接保持不变。应用采用复制语义：之后编辑或删除
模板不会自动改变已有订阅，需要重新选择模板并保存。Hysteria 2 分享链接不支持
分流规则，因此只能应用空规则且默认为使用节点的模板。

组织规则模板不包含节点、令牌或客户端凭据；编辑模板不会自动同步到已经应用的订阅。远程规则与多策略组能力由独立的路由方案提供，见下节。

## 基础规则模板与完整路由方案

基础规则模板包含静态 DOMAIN、DOMAIN-SUFFIX、IP-CIDR/IPv6 和默认动作，
最多 100 条，可复制到组织内编辑。另提供九个版本化完整路由方案，覆盖综合、
细分、娱乐、游戏、办公云服务及直连优先等场景；支持多策略组、节点分配、
规则源目标调整和默认策略。具体方案与稳定 ID 见 [路由模板](SUBSCRIPTION-TEMPLATES.md)。

Stash/Mihomo 输出 rule-providers，Surge 输出 URL RULE-SET，Loon 输出
Remote Rule。客户端直接下载并缓存允许的 ACL4SSR 规则文件，API 不下载、
镜像或固定其内容。上游变化、不可达或客户端缓存可能影响路由；初次导入需
能访问 GitHub raw 域名。不会向上游发送订阅令牌，不接受任意规则 URL，
也不执行外部脚本、MITM 或公共订阅转换服务。

HY2 URI 不能承载规则与策略组，带路由方案的订阅不能导出为该格式。
路由分类与延迟测试不保证流媒体解锁、特定地区出口或应用访问成功。

## 新增代理协议导出

| 协议 | Stash | Mihomo | Surge | Loon / HY2 URI |
| --- | --- | --- | --- | --- |
| SOCKS5 / Mixed | `socks5` | `socks5` | `socks5` | 拒绝 |
| Hysteria 1 | `hysteria` / `up-speed` / `down-speed` | `hysteria` / `up` / `down` | 拒绝 | 拒绝 |
| ShadowTLS v3 + SS2022 | SS + `shadow-tls` 插件 | SS + `shadow-tls` 插件 | 拒绝 | 拒绝 |
| Snell v4 兼容 | `snell`, `version: 4` | 拒绝 | `snell`, `version=4` | 拒绝 |
| Snell v6 测试版 | `snell`, `version: 6` | 拒绝 | `snell`, `version=6` | 拒绝 |

Mixed 订阅选择同端口的 SOCKS5 入口；手动配置也可使用 HTTP CONNECT。
ShadowTLS 订阅分别输出内层 SS2022 密钥和外层密码，使用系统 CA 验证公共
握手域名，不输出错误的自签名证书固定字段。Snell v4 兼容模式对应固定
运行时的 v5 服务端，不支持 v5 QUIC Proxy Mode，因此客户端明确使用 v4。
Snell v6 需要支持 v6 的 Stash 或 Surge 版本，不能当作旧版客户端兼容选项。

此表表示导出器实现和字段测试，不代表这些新增协议已完成各 App 的真实
联网验收。不支持的组合明确返回兼容性错误，包含多个节点时也不会静默
丢弃不支持的节点。

字段依据：[Stash 协议文档](https://stash.wiki/en/proxy-protocols/proxy-types)、
[Mihomo SS 插件](https://wiki.metacubex.one/en/config/proxies/ss/)、
[Surge Snell](https://manual.nssurge.com/policies/snell.html)。

## 配置 API 与策略组图标

单条设置读取、条件 PATCH、配置预览和下载接口见 [API 文档](API.md#subscription-configuration-api)。
新权限为 subscriptions:read/write/export，旧 API Key 不自动获得新权限。
读取设置不返回节点密码或 bearer 链接；预览与下载包含连接凭据并需单独导出权限。
网页编辑携带读取时的 revision，冲突时需刷新列表重新编辑，不自动覆盖他人的修改。

节点选择使用后端 subscription_selectable：更新失败但已恢复有效配置的节点仍可选择；
结果不确定、有待确认候选配置的节点不能加入新选择。客户端格式兼容性单独判断。

策略组可配置 HTTPS 图标 URL。Stash/Mihomo 输出 icon；Mihomo 显示取决于面板，
其他格式在预览或下载响应中说明图标未导出。网页编辑、节点选择和模板切换
保留同 ID 策略组的图标。此项尚未完成真实客户端图标显示验收。

## TrustTunnel 导出

`trusttunnel` 当前仅支持 `format=stash`：包含 `username: xingdu`、随机 `password`、
`alpn: [h2]`、`quic: false`、`udp: true`、`sni` 和 `server-cert-fingerprint`，
保持证书校验开启。其他客户端格式返回不兼容错误，不会忽略该节点。
HTTP/3 和中转未开放。真实验证范围见 [协议部署](PROTOCOL-DEPLOYMENT.md#trusttunnel)。

### 自定义传输（0.16.0-dev）

VLESS/VMess 的 Stash、Mihomo 导出保留 TLS、WS 路径/Host、gRPC service、
HTTP/h2、ALPN、VLESS Encryption 与 Vision；WireGuard 导出客户端密钥与参数。
Stash XHTTP 的 HTTP/1.1/3、无 TLS gRPC、gRPC/HTTP Vision、TUIC cubic
以及 Surge/Loon 自定义传输会明确拒绝，不会静默降级。
详见[传输兼容性及验收范围](TRANSPORT-COMPATIBILITY.md)。

## 原生 sing-box 与通用分享订阅（0.17.0-rc.1）

新增 `singbox`（JSON）、`uri`（每行一个分享链接）和 `base64`（整个 URI
列表的标准 Base64）。创建、编辑、预览及下载使用同一格式值，链接保持原有
访问控制与令牌语义。原生 JSON 可供支持对应配置格式的客户端使用；尚未宣称
Hiddify App 的所有版本已完成导入验收。

`singbox` 输出 SS、SS2022、VLESS、VMess、Trojan、HY2、HY1、TUIC、AnyTLS、
HTTPS、SOCKS5、Mixed。保留有序域名/IP 规则及默认策略，不导出 Clash 远程
规则预设。暂不输出 WireGuard、ShadowTLS、Snell、TrustTunnel、XHTTP、VLESS
Encryption 或 Xray 原始 HTTP 传输。不兼容时整个导出失败，不删除选中节点。

普通 TLS JSON 仅使用所选叶证书作为信任锚，保持域名与有效期检查，不关闭
证书验证；不把完整链中的根或中间证书扩大为信任锚。REALITY 导出公钥、
short ID、握手域名与 Chrome 指纹；可输出为 Mihomo、sing-box、URI、Base64，
Stash 格式明确拒绝未验收的 REALITY。Xray 26.9.9 REALITY 的握手要求与
当前 sing-box 客户端不兼容，sing-box / Mihomo 完整配置明确拒绝该组合；
Xray 服务端请使用支持新版 REALITY 的 Xray 客户端，不能沿用旧客户端验收。

`uri` / `base64` 输出 SS、SS2022、VLESS、VMess、Trojan、HY2、TUIC。
分享列表无法携带分流规则、策略组或图标，因此带这些路由配置的订阅不允许
选择链接格式。复杂请求头、拆分传输、原始 HTTP、XHTTP 和中转配置明确拒绝。
普通 TLS 链接要求系统信任的完整证书链；私有 CA 应使用完整配置。HY2 链接
保留叶证书 SHA-256 校验，REALITY 使用公钥认证。仍需客户端逐项验收。

## 保存前的配置兼容检查

订阅编辑器根据实际节点配置检查所选格式，展示可导出节点数量与每个不兼容
节点的原因，包含协议、传输方式、REALITY、证书信任要求和分流设置。
“全选”仅选择通过检查的节点，切换格式保留已有选择，不自动删除节点。
检查失败或正在检查时不能保存启用的订阅；已选不兼容节点仍可取消选择。

API 在保存时重复检查，下载和预览继续使用同一导出规则。检查报告不返回
密码、密钥、证书或配置内容。可导出只代表配置可表达，尚未建立按客户端
版本的验收目录，不能据此宣称某个 App 版本已能联网。
