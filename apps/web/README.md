# 星渡控制台

React + TypeScript + Vite。依赖安装与本地启动见 [开发指南](../../docs/DEVELOPMENT.md)。

开发服务器默认监听 `127.0.0.1:15173`，将 `/api` 和 `/health` 代理到 `127.0.0.1:18080`。页面使用用户会话访问真实 API，支持组织切换、成员权限、邀请链接和组织内服务器资料增删改查；Agent 接入和部署仍未实现，不模拟上线或部署成功。

```sh
npm ci
npm run dev
npm run lint
npm run build
```
