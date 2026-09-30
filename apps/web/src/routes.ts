import { t } from "./i18n";
export const pages = [
  {
    id: "billing",
    get label() {
      return t("套餐与账单");
    },
    icon: "◇",
  },
  {
    id: "overview",
    get label() {
      return t("概览");
    },
    icon: "◈",
  },
  {
    id: "hosts",
    get label() {
      return t("服务器");
    },
    icon: "▤",
  },
  {
    id: "nodes",
    get label() {
      return t("节点");
    },
    icon: "✧",
  },
  {
    id: "certificates",
    get label() {
      return t("证书管理");
    },
    icon: "♢",
  },
  {
    id: "routes",
    get label() {
      return t("线路");
    },
    icon: "⌁",
  },
  {
    id: "subscriptions",
    get label() {
      return t("客户端订阅");
    },
    icon: "▧",
  },
  {
    id: "deployments",
    get label() {
      return t("部署记录");
    },
    icon: "◷",
  },
  {
    id: "api-keys",
    get label() {
      return t("API 密钥");
    },
    icon: "⚿",
  },
  {
    id: "api-docs",
    get label() {
      return t("API 文档");
    },
    icon: "≡",
  },
  {
    id: "members",
    get label() {
      return t("人员管理");
    },
    icon: "♧",
  },
  {
    id: "profile",
    get label() {
      return t("个人中心");
    },
    icon: "○",
  },
  {
    id: "security",
    get label() {
      return t("安全中心");
    },
    icon: "◇",
  },
  {
    id: "settings",
    get label() {
      return t("设置");
    },
    icon: "⚙",
  },
] as const;
export type Page = (typeof pages)[number]["id"];
export const pagePath = (page: Page) => `/app/${page}`;
export function isConsolePath(path: string) {
  const normalized = path.replace(/\/+$/, "");
  return (
    normalized === "/app" ||
    normalized === "/login" ||
    pages.some((page) => pagePath(page.id) === normalized)
  );
}
export function pageFromURL(url: URL): Page {
  const path = url.pathname.replace(/\/+$/, "");
  const match = pages.find((page) => pagePath(page.id) === path);
  if (match) return match.id;
  // Preserve older bookmarked query-string URLs.
  return (
    pages.find((page) => page.id === url.searchParams.get("page"))?.id ??
    "overview"
  );
}
