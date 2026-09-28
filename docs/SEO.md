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
