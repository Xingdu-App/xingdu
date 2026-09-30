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

## 组织级自助证书管理

控制台「证书管理」现提供自有域名 CNAME 委托、平台随机域名分配、签发任务、
自动续期、到期状态和受控节点应用。此为源码实现；目前完成本地模拟/数据库验证，
真实 Cloudflare DNS、公共 CA、线上 Agent 和客户端更新仍须部署验收。

### 付费与权限

创建、签发、续期和应用要求有效的付费订阅：Starter 最多 10 张，Premium 最多
50 张；trialing、过期及未付款状态不获得此权益。期末取消但仍在有效付费期内可继续
使用。免费用户仍可在节点配置中手动上传自己的证书。读者可查看状态；仅组织所有者
和管理员可管理证书。自部署运营者仍可使用本文前半部分的独立命令行工具。

配额计算包含待验证、失败和暂停资源，创建在组织事务锁内校验。降级后按创建时间保留
配额内较早的资源，超额资源暂停签发/续期/应用，移除多余资源或升级后可恢复。移除管理后停止续期、
删除托管密文，不撤销 CA 证书、不删除节点已安装证书，也不自动删除平台 A/AAAA。
域名声明保留为墓碑，不能跨组织自动重新分配；自有域名可由同组织重新添加恢复管理，
DNS 退役清理需运营者处理。

### 自有域名：一次性 CNAME 委托

填写完整节点域名后，平台生成唯一目标，用户添加：

```dns
node.user.example.                  A       <用户VPS公网IPv4>
_acme-challenge.node.user.example.  CNAME   _acme-challenge.<随机ID>.validation.platform.example.
```

业务解析指向 VPS。验证 CNAME 指向平台控制的验证区，关闭代理，不要启用隐藏
CNAME 的 flattening。平台在目标写临时 TXT，成功或失败都会尝试清理该临时记录。
每次签发与续期都校验准确目标，即使 CA 重用已验证的授权也不能跳过租户委托校验。
用户无需交出 DNS API Key；当前不支持 Cloudflare OAuth 一键授权或 HTTP-01。
不自动覆盖用户现有冲突记录，泛域名暂不接受。

### 平台随机域名

配置平台域名后，用户选择一台服务器，平台生成 128 位随机名称，申请独立证书及
私钥，在 Cloudflare 创建该机器专属 DNS-only A 或 AAAA。服务器地址必须为允许的
公网 IP，暂不解析机器资料中的主机名。平台域名只可应用到原绑定服务器。

现有 DNS 记录不匹配时停止签发，不覆盖或改指向。服务器地址修改、删除后的 DNS
退役需要人工维护；机器删除后无法继续完成平台签发。所有节点不共享一张泛域名证书。

### 部署配置

只给 API 服务添加以下配置，保留其他环境变量：

```ini
XINGDU_CERTIFICATE_VALIDATION_DOMAIN=validation.platform.example
XINGDU_CERTIFICATE_PLATFORM_DOMAIN=nodes.platform.example
XINGDU_CERTIFICATE_CLOUDFLARE_ZONE=<Zone ID>
XINGDU_CERTIFICATE_CLOUDFLARE_TOKEN=<仅目标区域 DNS 编辑权限的 Token>
XINGDU_CERTIFICATE_EMAIL=<ACME 联系邮箱>
XINGDU_CERTIFICATE_STATE_DIR=/var/lib/xingdu/certificate-state/acme
XINGDU_CERTIFICATE_ACCEPT_TOS=true
XINGDU_CERTIFICATE_PRODUCTION=false
```

以上域名为示例，必须换成实际控制的域名。两个子域名都须位于配置的同一 Cloudflare
区域；平台域名可留空，此时仅关闭随机域名分配。签发基本配置缺失时关闭创建和签发。
状态目录需持久保存 ACME 账户私钥，权限 0700；账户文件 0600。Compose 已为 API
挂载独立状态卷。托管证书私钥使用现有 `XINGDU_CREDENTIAL_KEY` 加密保存在数据库，
不返回浏览器/API 列表。不要更换加密密钥或验证域名而不做相应迁移。

默认测试 CA。确认真实 DNS 验证可用后设置 `XINGDU_CERTIFICATE_PRODUCTION=true`，
对测试证书重新申请正式签发；测试 CA 证书不会应用到节点。对同一证书，正常续期
只在距离到期不足 30 天时接受；切换 CA 可以重新签发。

### 持久化任务与应用边界

API 每 15 秒发现持久化任务；发现函数只提供任务/组织/当前所有者身份，业务操作
重新建立组织/用户事务范围，由非 owner 数据库角色执行并遵守 RLS。任务租约 12 分钟，
外部签发最多 10 分钟。崩溃后可恢复；临时失败按小时退避，最多 5 次；委托错误需用户
修复后重试。付费失效暂停，保留旧证书，恢复付费后到期证书可继续续期。

自动续期只更新托管证书。应用到节点需用户确认短暂中断，更新 TLS 域名和证书，
通过现有加密修订/Agent 更新任务下发，保留启动失败恢复及并发检查。处理中节点、
旧 Agent、中转入口或被其他入口引用的出口会被拒绝或需要协调维护。
签发成功、任务排队、Agent 应用成功和真实客户端连通性是不同状态。
客户端使用证书指纹时，应用续期证书后须刷新订阅，不允许用跳过 TLS 校验规避。

当前未实现 ARI、按 CA Registered Domain 的全局发证预算或 CA 失败分类/Retry-After。
平台随机域名上线前需结合公共 CA 限流规划容量，不将套餐配额视为 CA 可用容量。
公开证书会进入 Certificate Transparency；随机名称不会隐藏域名或机器 IP。

证书 API 与权限见 [API 文档](API.md)。
