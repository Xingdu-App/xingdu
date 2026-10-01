// Only fixed route names and fixed operation labels are returned to GA.
const consolePages = new Set([
  "overview",
  "hosts",
  "nodes",
  "certificates",
  "routes",
  "subscriptions",
  "deployments",
  "api-keys",
  "api-docs",
  "members",
  "profile",
  "security",
  "settings",
  "billing",
]);
const consoleQuery = new Set([
  "organization",
  "host",
  "node",
  "protocols",
  "page",
  "plan",
  "checkout",
]);
export function consoleAnalyticsLocation(href: string) {
  try {
    const url = new URL(href);
    const pathname = url.pathname.replace(/\/+$/, "");
    const legacyPage = url.searchParams.get("page");
    const page =
      pathname === "/app"
        ? legacyPage && consolePages.has(legacyPage)
          ? legacyPage
          : "overview"
        : pathname.replace(/^\/app\//, "");
    if (
      !consolePages.has(page) ||
      url.hash ||
      [...url.searchParams.keys()].some((key) => !consoleQuery.has(key))
    )
      return null;
    return { page, location: url.origin + "/app/" + page };
  } catch {
    return null;
  }
}
const operationLabels: Record<string, string[]> = {
  checkout: ["选择套餐", "Choose plan"],
  manage_billing: ["管理账单", "Manage billing"],
  billing_monthly: ["月付", "Monthly"],
  billing_yearly: ["年付", "Yearly"],
  apply_route: ["应用线路", "Apply route"],
  create: ["创建", "添加", "Create", "Add"],
  create_key: ["创建密钥", "Create key"],
  add_domain: ["添加域名", "Add domain"],
  add_host: ["添加服务器", "Add server"],
  create_subscription: [
    "创建订阅",
    "创建并生成链接",
    "Create subscription",
    "Create and generate link",
  ],
  edit: ["编辑", "编辑配置", "Edit", "Edit configuration"],
  edit_permissions: ["修改权限", "Edit permissions"],
  save_permissions: ["保存权限", "Save permissions"],
  save: [
    "保存",
    "保存修改",
    "保存配置",
    "保存服务器",
    "Save",
    "Save changes",
    "Save configuration",
    "Save server",
  ],
  apply: ["应用配置", "应用证书", "Apply configuration", "Apply certificate"],
  cancel: ["取消", "Cancel"],
  close: ["关闭", "关闭弹窗", "Close", "×"],
  copy: [
    "复制",
    "复制密钥",
    "复制链接",
    "复制订阅链接",
    "复制配置",
    "Copy",
    "Copy key",
    "Copy link",
    "Copy subscription link",
    "Copy configuration",
  ],
  hide_key: ["已保存，隐藏密钥", "Saved, hide key"],
  download: ["下载", "下载配置", "Download", "Download configuration"],
  delete: [
    "删除",
    "删除资料",
    "删除服务器资料",
    "确认删除",
    "确认删除资料",
    "Delete",
    "Delete record",
    "Confirm delete",
  ],
  revoke: ["撤销", "确认撤销", "Revoke", "Confirm revocation"],
  select_all: ["全选", "Select all"],
  deselect_all: ["取消全选", "Deselect all"],
  refresh: ["刷新", "Refresh"],
  retry: ["重试", "Retry"],
  deploy: ["部署", "安装", "Deploy", "Install"],
  restart: ["重启", "Restart"],
  uninstall: ["卸载", "Uninstall"],
  issue_certificate: [
    "签发证书",
    "申请签发",
    "重新签发",
    "Issue certificate",
    "Request issuance",
    "Reissue",
  ],
  view: ["查看", "查看节点 →", "View", "View node →"],
  docs: [
    "API 文档",
    "API documentation",
    "查看 API 文档 →",
    "View API documentation →",
  ],
  clear_filter: ["清除筛选", "Clear filters"],
  logout: ["退出登录", "Sign out"],
};
export function interactionAction(
  label: string,
  kind: string,
  translations: Record<string, string> = {},
) {
  if (
    ![
      "button",
      "link",
      "checkbox",
      "radio",
      "combobox",
      "option",
      "tab",
      "summary",
      "menuitem",
    ].includes(kind)
  )
    return null;
  const normalized = label.trim().replace(/\s+/g, " ");
  for (const [action, labels] of Object.entries(operationLabels)) {
    if (
      labels.some(
        (label) => label === normalized || translations[label] === normalized,
      )
    )
      return action;
  }
  // Unknown labels can contain user names, addresses or credentials. Never send them.
  return [
    "button",
    "link",
    "checkbox",
    "radio",
    "combobox",
    "option",
    "tab",
    "summary",
    "menuitem",
  ].includes(kind)
    ? kind
    : null;
}
