# 星渡官网与控制台

React + TypeScript + Vite。依赖安装与本地启动见 [开发指南](../../docs/DEVELOPMENT.md)。

开发服务器默认监听 `127.0.0.1:15173`，将 `/api` 和 `/health` 代理到 `127.0.0.1:18080`。页面使用用户会话访问真实 API，支持组织切换、成员权限、邀请链接和组织内服务器资料增删改查；Agent 接入与 SSH 安装已实现，协议部署仍未实现，不模拟上线或部署成功。

```sh
npm ci
npm run dev
npm run lint
npm run build
```

官网路由为 `/`、`/pricing`、`/privacy`、`/security`；控制台位于 `/app`，`/login` 也可进入登录流程。旧版首页邀请链接会保留 fragment 并跳转至控制台。

生产构建预渲染四个公共页面，生成独立标题、描述、canonical、Open Graph、sitemap 和 robots.txt；控制台不被索引。构建时可通过 `XINGDU_SITE_URL` 指定公开站点域名（默认 `https://xingdu.app`，仅影响 SEO 元数据，不更改 API 会话来源）。开发服务器使用客户端渲染。

价格暂按 MIT 免费自托管、托管版价格待公布展示。隐私页描述当前数据处理实现；托管服务正式开放前仍需补充运营主体、地区、保留期限和隐私联络方式。
