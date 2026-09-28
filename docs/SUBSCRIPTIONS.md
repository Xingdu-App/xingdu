# 基础订阅

订阅把当前组织的已部署节点与一组有序规则组合成客户端配置。每次客户端拉取链接都会重新生成配置，因此编辑节点选择和规则不需要重新分发链接。

## 使用

1. 在“节点”确认至少有一个部署成功的节点。
2. 打开“订阅”，创建订阅，选择客户端格式（默认 Stash）并选择节点。
3. 按需要添加精确域名、域名后缀或 IPv4/IPv6 CIDR 规则。规则按列表顺序匹配，可选走代理、直连或拒绝；最后选择未匹配流量走代理或直连。
4. 保存并复制生成的链接，在对应的 Stash 或 Mihomo 客户端中添加远程配置。链接只在创建或重置时显示，请妥善保存。

导出的 `Xingdu` 选择组包含选中的节点，由客户端选择实际使用的节点。节点名称附带其 ID，避免重名节点或内置策略名称冲突。节点必须处于部署成功状态才会导出；已卸载或正在变更的节点不会出现在新配置中。所有节点均不可用时，接口明确返回错误，不会生成悄悄改为全直连的配置。

服务器地址必须能从客户端访问。本地 Docker 实验中的容器地址无法直接从外部设备访问；开发环境的回环订阅地址也仅能在当前机器使用。

## 支持范围

当前提供 **Mihomo / Clash.Meta 完整 YAML 配置**，包含 Trojan、VLESS、VMess 的 TCP + TLS，以及 Hysteria 2、TUIC v5 的 QUIC + TLS 节点。配置显式包含协议凭据、TLS 名称与服务端叶证书 SHA-256 指纹，不关闭证书校验，不导出服务端 TLS 私钥。证书更换后客户端需要更新订阅。

这里的指纹是证书固定（pinning），客户端只接受订阅指定的服务端叶证书。导出前会检查证书有效期、域名和部署配置。实现依据 [Mihomo TLS 文档](https://wiki.metacubex.one/en/config/proxies/tls/)、[协议字段](https://wiki.metacubex.one/en/config/proxies/)和[规则文档](https://wiki.metacubex.one/en/config/rules/)。旧版 Clash 和不同 App 的字段支持不同；Surge、Loon 和分享链接的明确范围见下方适配矩阵，Shadowrocket 尚待验收。

当前规则保存在单个订阅内，不包含共享规则库、在线规则集拉取、套餐、流量配额或单独的订阅用户凭据。规则最多 100 条，节点最多 100 个。域名规则使用普通 ASCII 域名（国际化域名请使用 Punycode），不接受通配符；“域名后缀”本身会匹配子域。

## 权限与链接

组织 owner/admin 可以创建、编辑、删除和重置订阅链接。成员可以查看订阅元数据，但不能从列表获取链接或节点凭据。订阅链接采用随机令牌，数据库只保存其哈希；链接持有者无需登录即可获取配置。请把链接当作密码，不要放进公开 Issue、截图或聊天记录。

重置链接使旧链接立即失效；停用或删除订阅会阻止后续拉取。**这不会撤销客户端已下载的节点凭据，也不会断开已有连接。** 若要撤销已泄露的节点访问，仍需移除相应节点并重新部署以更换凭据。基础订阅使用所选节点已有的共同凭据，尚无每位订阅用户独立凭据。

部署在反向代理后时，必须使用 HTTPS，并保证代理/CDN/访问日志不记录订阅查询参数。应用的订阅响应禁止缓存；应用日志不会记录链接令牌。当前项目整体仍是本地开发预览。

## 验证

导出器测试覆盖五种协议字段、证书指纹、服务端私钥排除、IPv6、重复节点名、规则顺序与注入拒绝。生成的五协议配置已通过官方 Mihomo v1.19.31（macOS arm64）的 `-t` 配置校验；这仅证明该内核版本接受配置，并不等同于所有客户端 App 的导入及联网验收。

可选的真实内核校验：自行从 Mihomo 官方发布页下载并核验其 SHA-256 后，将 `XINGDU_TEST_MIHOMO_BINARY` 指向本地二进制，运行 `go test -v ./internal/subscription`。该测试只校验临时生成的配置，不启动监听或连接代理服务器。

## Local acceptance

`make check` covers organization/RLS boundaries, token lifecycle and configuration rendering. To additionally validate generated fixtures with a locally installed official Mihomo binary, set `XINGDU_TEST_MIHOMO_BINARY` to its absolute path when running the checks.

With the disposable Agent Lab running and at least one successfully deployed lab node, run `python3 scripts/subscription-lab.py`. It creates and cleans up a temporary subscription, exercises export/edit/rotation/disable/delete through the running HTTP service, and checks proxy logs for token leakage. Set the same optional binary variable to parse the live exported configuration. Reports contain no credentials and are saved under ignored `.local/subscription-lab/`. This does not prove client-app interoperability or public VPS reachability.


## Stash 格式

新建订阅默认选择 Stash，可在创建或编辑时切换为 Mihomo。现有订阅迁移时保留 Mihomo；旧版链接显式 `format=mihomo` 或 `format=clash` 的输出不变。新生成的链接使用订阅保存的格式；旧链接需切换格式时可重置链接（旧令牌失效），或由链接持有者显式改为 `format=stash`。

Stash 按[官方协议文档](https://stash.wiki/proxy-protocols/proxy-types)独立适配：Hysteria 2 使用 `auth`、TUIC 明确 `version: 5`、TLS 使用 `server-cert-fingerprint` 和 `sni`。支持当前五种 TLS 节点，保留有序规则与策略组，不关闭证书验证。未支持的客户端格式会返回 422，不会静默当作其他客户端输出。

Stash 字段映射、格式持久化和真实 HTTP 订阅已验证；尚未完成发布版 Stash App 的导入和实际转发验收。Mihomo 的解析测试不作为 Stash 验收证据。

## 客户端适配矩阵（2026-09-28）

| 格式 | Trojan | VLESS | VMess | HY2 | TUIC v5 | TLS 信任 | 验证层级 |
|---|---|---|---|---|---|---|---|
| Stash | 输出 | 输出 | 输出 | 输出 | 输出 | 固定叶证书 SHA-256 | 字段单测；App 联网待验收 |
| Mihomo | 输出 | 输出 | 输出 | 输出 | 输出 | 固定叶证书 SHA-256 | v1.19.31 解析；App 联网待验收 |
| Surge | 输出 | 拒绝 | 输出 | 输出 | 输出 | 固定叶证书 SHA-256 | 生成器单测；真实解析/联网待验收 |
| Loon | 输出 | 输出 | 输出 | 输出 | 拒绝 | 明确选择系统 CA；完整受信任链 | 生成器/隔离 CA 测试；App 待验收 |
| HY2 分享链接 | 拒绝 | 拒绝 | 拒绝 | 输出 | 拒绝 | `pinSHA256` + `insecure=0` | URI 解析单测；App 待验收 |
| Shadowrocket | 待验收 | 待验收 | 待验收 | 待验收 | 待验收 | 尚未确认导入后证书固定语义 | 暂不提供专用输出 |

本表的“输出”只表示适配器已实现，不表示 App 端已经通过真实连接测试。Surge 的 HY2 要求至少 iOS 5.8.0 / Mac 5.4.0；其他协议应使用支持对应协议及证书固定字段的当前版本，最低版本尚未完成矩阵验收。Loon 不猜测未在官方文档中定义的 pinning 字段；选择该格式即明确采用系统 CA 验证，导出时校验证书域名、有效期、链和系统根。私有 CA、自签名、缺少中间证书的节点会明确报错。客户端与控制平面的根证书集可能不同，仍需 App 验收。

Surge 采用独立 INI 适配：VMess AEAD + TLS、TUIC v5 的 UUID/密码分别输出。不支持的节点不会被静默剔除，整个输出返回 422。INI 名称会替换语法分隔字符，并附带节点 UUID，避免配置注入和名称冲突。Loon 格式不能通过旧链接的 `format` 参数自动切换信任模式，必须由管理员编辑订阅明确选择。

分享链接只支持有官方 URI pinning 规范的 Hysteria 2。必须没有分流规则且默认走节点，否则拒绝导出，不丢弃既有路由语义。链接包含节点凭据，应视同密码。

依据官方来源：

- [Surge 协议总览](https://manual.nssurge.com/policies/overview.html)、[TLS 固定证书](https://manual.nssurge.com/policies/tls.html)、[VMess](https://manual.nssurge.com/policies/vmess.html)、[TUIC](https://manual.nssurge.com/policies/tuic.html)。
- [Loon 节点语法](https://github.com/Loon0x00/LoonManual/blob/master/docs/cn/node.md)、[策略组](https://github.com/Loon0x00/LoonManual/blob/master/docs/cn/policygroup.md)。
- [Hysteria 2 URI 标准](https://hysteria.network/docs/developers/URI-Scheme/)。

待验收操作：使用一次性测试订阅和隔离网络，逐个记录客户端/OS 版本、导入结果、TLS 错误证书拒绝、认证失败、TCP/UDP 转发、订阅更新和撤销行为；不要覆盖用户日常代理配置。未完成这些步骤前，不对外宣称对应客户端全量兼容。
