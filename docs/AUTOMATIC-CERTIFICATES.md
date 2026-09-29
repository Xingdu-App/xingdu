# 自动签发与续期

提供独立 `cmd/certificates` 程序，使用 ACME DNS-01 和 Cloudflare DNS API。
默认 Let's Encrypt 测试 CA；正式 CA 需显式启用。当前配置由运营者管理，
网页支持替换证书与查看有效期，尚无租户自助 DNS 授权界面。

构建 `go build -o bin/xingdu-certificates ./cmd/certificates`。
将以下环境变量放在权限 0600 的环境文件或 secret manager 中：

```ini
XINGDU_ACME_STATE=/var/lib/xingdu-certificates/node-a
XINGDU_ACME_DOMAIN=node.example.com
XINGDU_ACME_EMAIL=ops@example.com
XINGDU_ACME_ACCEPT_TOS=true
XINGDU_ACME_PRODUCTION=false
XINGDU_ACME_ZONE=Cloudflare区域ID
XINGDU_ACME_DNS_TOKEN=仅授权目标区域DNS编辑的令牌
```

`ACCEPT_TOS` 表示运营者已确认所选 CA 的服务条款。每个状态目录仅对应一个
域名和 CA 环境；测试切正式环境时使用新的目录，避免复用测试证书。程序只
创建自己的 `_acme-challenge` TXT 记录，等待 DNS 可见后完成挑战，并删除
此次创建的记录；不会替换用户已有 TXT 记录。DNS 传播有超时，失败不签发。
进程被强制终止时可能留下挑战记录，需核对后清理。

首次运行在私密目录写入 `account.pem` 和 `certificate.json`，均为 0600。
JSON 包含证书及私钥，不要粘贴到日志或提交 Git。可用于首次创建 TLS 节点。
不重复签发剩余有效期超过 30 天的缓存证书。

节点已创建后补充：

```ini
XINGDU_ACME_API=https://control.example.com
XINGDU_ACME_ORG=组织ID
XINGDU_ACME_API_KEY=待替换
XINGDU_ACME_HOST=服务器ID
XINGDU_ACME_NODE=节点ID
```

API Key 需要 `nodes:credentials` 和 `nodes:write`。目标节点必须与指定域名
一致。程序签发后提交版本化更新，由 Agent 校验、应用及失败恢复；不会直接
SSH 修改文件。每天执行一次即可续期，示例 systemd 单元见 `deploy/`。
提交成功仅表示排队，仍需检查节点更新结果。更新失败可重跑，优先重用已签发
证书，不重复申请。被中转引用的出口需要先解除依赖再续期，不能静默切换。

当前客户端导出固定叶证书指纹，证书更换后客户端需刷新订阅。不能承诺已有
缓存配置无感续期。DNS 提供商只实现 Cloudflare，其他提供商尚未支持。

本地测试覆盖完整模拟 ACME 订单、DNS 挑战清理、证书签发与文件权限；没有
使用真实 DNS Token 或公共 CA 完成签发，公共签发和续期仍需部署验收。

参考：[Go ACME](https://pkg.go.dev/golang.org/x/crypto/acme)、
[Cloudflare DNS API](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/create/)。

证书缓存绑定域名与 CA 目录。更换域名或测试／正式 CA 必须使用新的私有状态目录，
已有不匹配或未绑定的缓存会被拒绝，避免把测试证书用于正式环境。
