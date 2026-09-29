import { blogPosts, blogPath } from "./blog-posts";

export type PublicPage =
  | "home"
  | "pricing"
  | "privacy"
  | "security"
  | "help"
  | "service"
  | "blog"
  | `blog/${string}`;
export const publicPages: Record<
  PublicPage,
  {
    path: string;
    title: string;
    description: string;
    articleSlug?: string;
    updated?: string;
  }
> = {
  blog: {
    path: "/blog",
    title: "服务器管理博客：VPS、Agent 与客户端订阅指南 · 星渡 Xingdu",
    description:
      "星渡博客分享个人 VPS 与多服务器管理、Agent 与 SSH 接入、客户端订阅及自托管选择的实用指南，也提供按需协作的权限管理建议。",
    updated: blogPosts
      .map((post) => post.updated)
      .sort()
      .at(-1),
  },
  ...Object.fromEntries(
    blogPosts.map((post) => [
      `blog/${post.slug}`,
      {
        path: blogPath(post),
        title: `${post.title} · 星渡博客`,
        description: post.description,
        articleSlug: post.slug,
        updated: post.updated,
      },
    ]),
  ),
  home: {
    path: "/",
    title: "星渡 Xingdu — 开源 VPS 服务器管理与协议部署面板",
    description:
      "星渡是适合个人与团队的开源 VPS 管理面板，集中管理服务器、部署协议节点并生成客户端配置。自托管免费，Xingdu Cloud 当前为公开测试版。",
  },
  help: {
    path: "/help",
    title: "帮助中心 · 星渡 Xingdu",
    description:
      "从登录和组织邀请，到服务器接入、协议部署与客户端订阅，了解星渡的使用步骤和常见问题。",
  },
  service: {
    path: "/service",
    title: "服务范围 · 星渡 Xingdu",
    description:
      "了解星渡公开测试版的功能范围、资源要求、费用说明与停止使用时的数据处理边界。",
  },
  pricing: {
    path: "/pricing",
    title: "价格与部署方式 · 星渡 Xingdu",
    description:
      "选择星渡 Starter、Premium 或 Enterprise：Starter 每月 $5 起，Premium 每月 $20 起、包含 50 台服务器，Enterprise 联系客服定制。",
  },
  privacy: {
    path: "/privacy",
    title: "隐私与数据处理说明 · 星渡 Xingdu",
    description:
      "了解星渡处理哪些数据、如何使用 SSH 凭据和协议密钥、必要 Cookie 的用途，以及自托管与未来托管服务的数据边界。",
  },
  security: {
    path: "/security",
    title: "安全设计 · 星渡 Xingdu",
    description:
      "了解星渡的组织权限、SSH 主机指纹校验、凭据加密、低权限探针与机器身份撤销机制。",
  },
};
