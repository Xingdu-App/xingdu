# 星渡控制台

React + TypeScript + Vite。依赖安装与本地启动见 [开发指南](../../docs/DEVELOPMENT.md)。

开发服务器默认监听 `127.0.0.1:15173`，将 `/api` 和 `/health` 代理到 `127.0.0.1:18080`。页面从真实 API 读取服务器列表；规划中的功能只显示说明，不模拟部署成功。

```sh
npm ci
npm run dev
npm run lint
npm run build
```
