# Google、GitHub 与邮箱登录

星渡保留邮箱验证注册和邮箱/密码登录，Google 与 GitHub 作为可选身份提供方。每个部署独立创建 OAuth 应用；不要复用其他产品的密钥。只有 API 接收 OAuth client secret，Web 与 Worker 不需要这些机密。API 还需要持久保存的 `XINGDU_CREDENTIAL_KEY`（64 位十六进制），用于加密短期 OAuth 状态；未配置时第三方登录同样禁用。未配置的提供方保持禁用，配置存在也不等于真实授权流程已经验收。

## 提供方配置

| 提供方 | API 环境变量 | 回调路径 |
| --- | --- | --- |
| Google | `XINGDU_GOOGLE_CLIENT_ID`、`XINGDU_GOOGLE_CLIENT_SECRET` | `/api/v1/auth/oauth/google/callback` |
| GitHub | `XINGDU_GITHUB_CLIENT_ID`、`XINGDU_GITHUB_CLIENT_SECRET` | `/api/v1/auth/oauth/github/callback` |

回调地址是 `XINGDU_PUBLIC_ORIGIN` 加上表中路径，必须与提供方后台完全一致。例如星渡托管服务的 Google 使用 `https://xingdu.app/api/v1/auth/oauth/google/callback`，GitHub 使用 `https://xingdu.app/api/v1/auth/oauth/github/callback`。更换域名时，需要同步更新提供方回调地址、`XINGDU_PUBLIC_ORIGIN` 和 API/Worker 的 `XINGDU_AGENT_ORIGIN`。正式环境使用 HTTPS。

Google 创建 Web application 类型客户端，并配置同意屏幕、受众以及测试用户。只申请登录所需的 OpenID 与邮箱信息。GitHub 创建独立 OAuth App，使用身份和邮箱范围，不申请仓库或组织管理权限。应用名称、首页及隐私政策应指向实际部署的星渡服务。参考 [Google OpenID Connect](https://developers.google.com/identity/openid-connect/openid-connect) 与 [GitHub OAuth Web flow](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps)。

在 Zeabur API 服务添加上述变量后重启 API；不要把密钥添加为项目共享变量，也不要写入前端构建变量、Git、命令行参数或截图。邮箱验证继续通过 `RESEND_API_KEY` 与 `XINGDU_EMAIL_FROM` 独立配置，参考 [邮箱注册](EMAIL-REGISTRATION.md)。

## API 与迁移

迁移 `019_social_auth.sql` 新增身份绑定与短期授权状态，`020_social_password_guard.sql` 限制未启用密码方式的账号。先迁移，再发布 API 与 Web。身份绑定表使用账户级 RLS；登录前的身份解析通过只读、无登录权限角色所拥有的窄范围数据库函数完成，不授予 API 绕过 RLS 的权限。

- `GET /api/v1/auth/config` 返回 `oauth_providers.google` 与 `oauth_providers.github` 配置状态。
- `POST /api/v1/auth/oauth/{provider}/start` 接收 `mode: login` 或 `mode: link`，返回 `authorization_url`。绑定需要当前会话与 CSRF 校验；两种模式都要求同源请求。
- `GET /api/v1/auth/oauth/{provider}/callback` 单次处理回调，跳转到固定站内路径，只携带白名单结果码。
- `GET /api/v1/account/identities` 返回 `has_password` 与已绑定身份，不返回提供方 token 或 subject。
- `DELETE /api/v1/account/identities/{provider}` 在当前会话下解绑，并原子检查剩余可用登录方式。

## 账号边界

第三方身份由提供方与稳定的 subject / 用户 ID 标识，不能用可变用户名或邮箱替代。提供方返回已验证邮箱后，首次登录会按规范化邮箱匹配已有的已验证邮箱账号，并自动关联，无需先手动绑定。只忽略大小写和首尾空白，不折叠加号别名或 Gmail 点号。仅用户名相同的旧账号不自动关联；已关联身份始终优先按稳定 subject 识别，不因提供方邮箱变化转移账号。公开注册关闭时，已有账号仍可自动关联登录，只有创建新账号被禁止。同一邮箱的并发首次登录会串行处理，避免重复创建账号和组织。不同邮箱仍可在登录后通过安全中心主动关联。

解绑只移除当前关联，再次使用同一已验证邮箱登录会重新关联。解绑必须保留至少一种可用登录方式，已经停用的提供方不计入可用方式。第三方新账号没有用户设置的本地密码；邮箱密码登录与密码修改界面不能将其误认为已有密码账号。后续设置密码或找回密码需要独立的身份验证流程。

OAuth 状态十分钟过期、单次消费，并通过独立的 HttpOnly / SameSite=Lax Cookie 绑定发起浏览器。PKCE verifier 与 Google nonce 加密暂存；Google 同时验证令牌签名、签发方、受众和 nonce。绑定回调重新检查原登录会话仍有效。第三方访问令牌不持久保存。

应用内 OAuth 回调路径禁用 Nginx 访问日志与错误日志，响应禁止缓存并设置 `Referrer-Policy: no-referrer`。外部负载均衡、平台日志和监控也应避免采集该路径的查询参数，因为授权 code 与 state 会经过回调 URL。

## Passkey 后续接入

这里的 Apple “keyless” 指 Passkey，通过 Face ID / Touch ID 等完成设备端验证，不是“通过 Apple 登录”。Passkey 基于 WebAuthn，也应支持兼容的 Android、Windows 和硬件安全密钥。服务端保存公钥和凭据标识，不接收指纹、面容或私钥。参考 [Apple Passkeys](https://developer.apple.com/passkeys/)。

后续实现将包括：登录后注册与管理多个凭据、一次性挑战与 origin/RP ID 校验、免密码登录、撤销及最后一种登录方式保护。正式域名与 RP ID 需要提前确定；在预览域名注册的凭据不能假定自动适用于另一个注册域名。本轮不提供可点击的 Passkey 登录入口，也不宣称已经支持设备端验证。

## 验收边界

自动化测试可以覆盖状态绑定、重放、身份冲突和接口错误；真正上线仍需分别完成 Google 与 GitHub 的真实授权、拒绝授权、登录、绑定、解绑和原邮箱账号回归。没有真实 OAuth 应用凭据时，不能用配置检查或模拟提供方测试代替这些验收。
