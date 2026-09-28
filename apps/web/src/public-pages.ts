export type PublicPage = "home" | "pricing" | "privacy" | "security";
export const publicPages: Record<
  PublicPage,
  { path: string; title: string; description: string }
> = {
  home: {
    path: "/",
    title: "星渡 Xingdu — 让服务器与团队，在一处相遇",
    description:
      "星渡是开源的服务器管理工作空间，支持 Agent 接入、SSH 安装、运行状态监测与组织协作。自托管免费，托管服务筹备中。",
  },
  pricing: {
    path: "/pricing",
    title: "价格与部署方式 · 星渡 Xingdu",
    description:
      "选择适合你的星渡：MIT 开源版本免费自托管，托管服务的价格与套餐将在开放前公布。",
  },
  privacy: {
    path: "/privacy",
    title: "隐私与数据处理说明 · 星渡 Xingdu",
    description:
      "了解星渡处理哪些数据、如何使用 SSH 凭据、必要 Cookie 的用途，以及自托管与未来托管服务的数据边界。",
  },
  security: {
    path: "/security",
    title: "安全设计 · 星渡 Xingdu",
    description:
      "了解星渡的组织权限、SSH 主机指纹校验、凭据加密、低权限探针与机器身份撤销机制。",
  },
};
