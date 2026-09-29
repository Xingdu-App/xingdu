# SEO 与博客维护

## 已实现的页面输出

公开页面与博客文章在构建时预渲染，不依赖搜索引擎执行 JavaScript 获取正文。
公开页面的尾斜线与 index.html 入口通过 308 统一到 canonical 路径。
页面定义位于 `apps/web/src/public-pages.ts`，文章源位于
`apps/web/src/blog-posts.ts`。新增文章会自动进入路由、目录、站点地图和 RSS。

每页独立输出标题、描述、canonical、Open Graph 和 Twitter Card。
JSON-LD 包含品牌 Organization、WebSite 与 WebPage；博客目录为 CollectionPage，
文章另有 BlogPosting 和与可见导航一致的 BreadcrumbList。
不生成虚构评分、用户数量、作者履历或未经验证的产品承诺。

站点地图为 `/sitemap.xml`，文章更新订阅为 `/blog/feed.xml`。
文章更新日期与 sitemap lastmod、JSON-LD dateModified 共用真实内容日期，
不在每次构建时刷新。当前没有公开发布日记录，因此不虚构 datePublished。
控制台、登录和 404 页面带有 noindex 标记，站点地图仅包含公开页面。
robots.txt 是抓取指引，不是访问控制或确保已收录 URL 移除的机制。

## 首批文章与阅读路径

| 主题 | 目标问题 | 产品入口 |
| --- | --- | --- |
| 多台 VPS 管理 | 如何统一机器台账、接入与部署 | 入门帮助、控制台 |
| 自托管与云端 | 成本、维护责任和数据位置如何选择 | 价格页、隐私、服务范围 |
| Agent 与 SSH | 安装方式、权限和凭据如何处理 | 安全设计、机器接入说明 |
| 客户端订阅 | 格式、规则、证书和链接更新有何区别 | 客户端矩阵、控制台 |
| 团队权限 | 组织、邀请与人员交接如何安排 | 账户安全、组织说明 |

这些选题来自产品实际使用场景，没有声称经过搜索量或竞争度统计。
内容以解决问题为主，文章末尾再引导读者了解产品。避免批量替换关键词生成近似页面。

## 新增或更新文章

1. 添加稳定的英文 slug、具体标题、独立摘要、分类及真实内容更新日期。
2. 给每节设置唯一 id，正文提供可执行步骤、选择依据与当前能力边界。
3. related 仅引用已有文章，references 指向相关产品页或确实支持结论的来源。
4. 更新现有能力时，同时核对博客、帮助和价格页，避免把字段导出说成 App 实测。
5. 运行 `make check`；构建会校验标题/摘要唯一性、预渲染正文、canonical、
   JSON-LD、日期、内部链接、RSS 及私有页面 noindex。
6. 用容器验证文章直达、刷新、未知文章 404，并检查手机排版。

## 域名与发布

`XINGDU_SITE_URL` 是**构建时**的公开 origin，默认 `https://xingdu.app`。
支持 HTTP(S) origin，不允许用户名、密码、子路径、查询参数或 fragment。
修改域名需要重新构建，运行时改变 API origin 不会更新已生成的元数据。
两个 Web Dockerfile 都提供同名 build argument；自定义部署域名时传入该参数。
本地测试可使用保留域名 `https://example.com` 验证生成结果。

上线后由站点所有者完成以下检查；本地构建不代表已被搜索引擎抓取或收录：

- 核对公网域名、HTTPS、canonical 和站点地图一致，部署平台未增加 noindex。
- 在 Google Search Console 验证域名并提交 `/sitemap.xml`，按需要配置其他搜索平台。
- 对首页和代表性文章执行 URL Inspection 与 Rich Results Test。
- 检查未知文章返回真实 HTTP 404，不是状态为 200 的应用错误页。
- 观察真实搜索查询、展示、点击和收录情况，再决定后续选题与旧文更新。
- 正式文章上线后，如需展示发布日期，再填写真实日期并同步可见正文与结构化数据。

结构化数据帮助搜索引擎理解页面，不保证富媒体展示或排名。

## 官方参考

- [Google Article 结构化数据](https://developers.google.com/search/docs/appearance/structured-data/article)
- [站点地图构建与提交](https://developers.google.com/search/docs/crawling-indexing/sitemaps/build-sitemap)
- [标题链接](https://developers.google.com/search/docs/appearance/title-link)
- [面包屑](https://developers.google.com/search/docs/appearance/structured-data/breadcrumb)

## Google Analytics 4（可选）

Web 构建时设置 `VITE_GA_MEASUREMENT_ID=G-XXXXXXXXXX`，再重新构建发布。
两个 Web Dockerfile 都接受同名 build argument；Vite 本地开发可从环境变量读取。
该 ID 是公开标识，不是 API 密钥。未设置或格式不合法时，不显示统计偏好、
不加载 Google 脚本，自部署默认关闭。仅增加这一项，不覆盖现有部署变量。
在托管平台确认变量已传入 Docker 构建参数，仅设置容器运行时变量不会生效。

在星渡专用 GA4 Web 数据流中关闭 Enhanced measurement（增强型衡量），
以免自动采集出站链接、表单或历史路由；关闭 Google signals、广告个性化，
不要关联广告账户。建议事件数据保留设置为两个月。接入前先核对数据流域名。

用户允许统计后才加载 gtag；拒绝时不向 Google 发送统计请求。页脚可重新选择，
撤回会停止后续事件、删除当前主机的 GA Cookie 并刷新页面卸载标签；不会删除
已经发送到 Google 的历史数据。统计仍受浏览器拦截、拒绝同意等影响，并非完整访问人数。

仅精确匹配公开页面白名单。控制台、登录、API、未知路由，以及带未知查询参数或
fragment 的页面不会加载统计。仅允许下文列出的单个 AI 来源 UTM 标记，
不支持任意 UTM 活动参数。
`page_location` 使用公开 origin + 路径，`page_title` 使用固定公开标题，
referrer 仅保留 origin。不要添加邮箱、用户 ID、组织 ID、机器地址、订阅 URL
等参数。Google 仍会接收请求所需的网络信息，不能声称此统计完全匿名。

| 事件 | 含义 |
| --- | --- |
| `page_view` | 同意统计后的公开页面访问，关闭默认自动 page view 避免重复 |
| `marketing_cta_click` | 固定 `destination`：`console`、`pricing`、`github` |

在 GA4 为 `destination` 建立事件范围自定义维度。入口点击不是注册成功、
首次接入成功或支付成功；本次未实现这些后台业务转化事件。

发布验收：无选择/拒绝时无 Google 请求；同意后每页只有一次 page_view；
点击入口可见固定事件标签；切入控制台不加载 gtag；撤回后刷新不再加载；
带未知查询参数和邀请片段的页面不加载；已知 AI 来源仅上报固定标签；
同时用 GA4 Realtime 核对真实数据接收。
构建或本地测试通过不能代替线上 GA4 接收验证。

## 上线后四周的自然获客

1. 第一周：在 Search Console 验证域名，提交 sitemap，检查首页和五篇现有
   指南的索引状态；完善 GitHub README 演示图、安装入口与托管版入口。
2. 第二周：发布两篇可复现的实操文章，例如 Ubuntu/Debian Agent 安装排障、
   自有节点导入 Stash/Mihomo 的分步流程。写明版本、错误示例与验证范围。
3. 第三周：补充规则模板组合教程（开发与影音可同时使用）和服务器迁移指南。
   在允许项目分享的技术社区发布实际解决问题的案例，避免重复灌水和虚构对比。
4. 第四周：根据 Search Console 的真实查询/展示/点击改标题与内容，更新有
   展示但点击低的文章，完善读者卡住的步骤；不要为了数量生成同质页面。

每周分别看搜索展示与点击（Search Console）、自然搜索访问与入口点击（GA4），
以及真实注册、首台 Agent 接入、首个订阅创建、付费（业务系统）。避免将入口
点击当注册率，或把 GA4 中未采集的业务指标视为零。当前选题是产品场景假设，
不代表已验证搜索量；四周后用实际查询数据调整。

## GEO：公开事实页与 AI 来源

新增 `/personal-vps`、`/protocols`、`/docs/api` 与三篇操作清单，均预渲染为
无需登录的 HTML，并自动加入 sitemap。公开 API 页复用控制台文档，但不
请求用户、组织、密钥或机器数据。协议表明确区分基础版本与尚未确认云端
发布的扩展；每次发布协议或调整客户端导出时同步核对该表。

robots.txt 对 `*` 和 `OAI-SearchBot` 使用相同的公开访问和私有路径排除，
不改变 GPTBot 的独立训练许可策略。robots 不是访问控制；私有接口仍要求
鉴权。CDN、防火墙与托管平台也必须允许真实抓取请求，不能用本地构建成功
代替线上抓取验证。依据：
[OpenAI crawlers](https://developers.openai.com/api/docs/bots)。

原先所有带参数的 URL 都不加载分析。现在只允许一个 `utm_source`，值为
`chatgpt.com`、`claude.ai`、`perplexity.ai`、`gemini.google.com` 或
`copilot.microsoft.com`；未知值、额外参数、重复参数及片段仍拒绝。
上报的 `page_location` 始终移除查询参数。`page_view` 和
`marketing_cta_click` 可携带固定枚举 `ai_source`，也可从匹配的来源域名
推导；来源路径、查询和用户提示词均不采集。来源标记可以被伪造，只用于
访问分析，不能证明 AI 自然推荐或引用。既有统计配置及隐私设置仍适用。

上线后在 GA4 注册事件范围自定义维度 `ai_source`，检查实际接收后再报告
流量。未配置 Measurement ID 时不会发送分析；不把没有数据解释成零流量。
本次没有新增第三方追踪服务，也没有采集注册成功或付费转化。

### 上线后的人工核验

1. 在 Google Search Console 和 Bing Webmaster Tools 验证域名所有权，提交
   `https://xingdu.app/sitemap.xml`。保存提交与抓取结果，不把提交成功当收录。
2. 对三个新页面执行 URL 检查，确认 200、正文、canonical、非 noindex。
   记录搜索引擎最后抓取时间。若被拦截，用官方公布的抓取身份规则排查。
3. 用固定问题、新对话和联网搜索测试：个人多台 VPS 怎么管理；多协议节点
   管理面板怎么选；跨客户端订阅怎么维护；如何用 API 管理 VPS。
4. 记录日期、平台/模式、完整问题、是否自然提及星渡、引用 URL、事实是否
   正确。品牌词查询与不带品牌的需求查询分开；重复采样，不依据一次回答
   判断排名。Bing 的 AI 报告仅代表其覆盖的产品，不能外推为所有 AI 流量。
5. 每周人工比较引用、AI 来源访问和控制台入口点击。外部介绍应披露作者
   关系，不购买假评测、不冒充用户，也不发布未验证的性能或兼容性结论。

本站新页面并不保证 AI 推荐。Search Console、Bing、GA4 的账号配置与线上
收录需在部署之后单独确认；本地校验不代表这些外部操作已经完成。
