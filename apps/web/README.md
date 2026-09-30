# 星渡官网与控制台

React + TypeScript + Vite。依赖安装与本地启动见 [开发指南](../../docs/DEVELOPMENT.md)。

开发服务器默认监听 `127.0.0.1:15173`，将 `/api` 和 `/health` 代理到
`127.0.0.1:18080`。控制台支持中英文，使用真实 API 与用户会话；已提供组织、
成员邀请、服务器、Agent 接入/升级、协议节点、配置修订、中转、订阅路由、
API Key 和账单管理。实现与实际联网验收的区别见
[功能状态](../../docs/IMPLEMENTATION-STATUS.md)。

```sh
npm ci
npm run dev
npm run lint
npm run build
```

官网路由为 `/`、`/pricing`、`/privacy`、`/security`；控制台位于 `/app`，`/login` 也可进入登录流程。旧版首页邀请链接会保留 fragment 并跳转至控制台。

生产构建预渲染官网、博客目录及全部文章，生成独立标题、描述、canonical、Open Graph、Twitter Card、JSON-LD、RSS、sitemap 和 robots.txt；控制台带有 noindex 标记且不进入站点地图。构建后自动校验生成页面的元数据、内部链接及结构化数据。构建时可通过 `XINGDU_SITE_URL` 指定公开站点域名（默认 `https://xingdu.app`，仅影响 SEO 元数据，不更改 API 会话来源）。开发服务器使用客户端渲染。

价格页展示 Starter（$5/月、$40/年、10 台）、Premium（$20/月、$200/年、50 台）和联系定制的 Enterprise；自托管软件采用 MIT。购买入口受 Cloud 模式与 Stripe 配置影响，见 [账单](../../docs/BILLING.md)。隐私页应随数据处理、分析工具和部署配置同步维护。

博客编辑与上线收录检查见 [SEO 与博客维护](../../docs/SEO.md)。
