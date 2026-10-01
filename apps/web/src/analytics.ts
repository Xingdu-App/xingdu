// Only explicit public routes and fixed event labels may enter analytics.
const aiSources = new Set([
  "chatgpt.com",
  "claude.ai",
  "perplexity.ai",
  "gemini.google.com",
  "copilot.microsoft.com",
]);
function safeSearch(url: URL) {
  if (!url.search) return true;
  const entries = [...url.searchParams.entries()];
  return (
    entries.length === 1 &&
    entries[0][0] === "utm_source" &&
    aiSources.has(entries[0][1])
  );
}
export function analyticsAISource(href: string, referrer: string) {
  const url = new URL(href);
  if (url.hash || !safeSearch(url)) return undefined;
  const source = url.searchParams.get("utm_source");
  if (source && aiSources.has(source)) return source;
  try {
    const host = new URL(referrer).hostname.replace(/^www\./, "");
    if (aiSources.has(host)) return host;
  } catch {
    /* Direct visit. */
  }
  return undefined;
}
export function analyticsPage(href: string, paths: readonly string[]) {
  const url = new URL(href);
  if (!paths.includes(url.pathname) || !safeSearch(url) || url.hash)
    return null;
  return url.origin + url.pathname;
}
export function analyticsID(value: string | undefined) {
  return /^G-[A-Z0-9]+$/.test(value || "") ? value! : null;
}
export function analyticsReferrer(value: string) {
  try {
    return new URL(value).origin;
  } catch {
    return "";
  }
}
export function analyticsTarget(href: string, origin: string) {
  const url = new URL(href, origin);
  if (url.search || url.hash) return null;
  if (url.origin === origin) {
    if (url.pathname === "/app") return "console";
    if (url.pathname === "/pricing") return "pricing";
  }
  if (
    url.origin === "https://github.com" &&
    url.pathname === "/Xingdu-App/xingdu"
  )
    return "github";
  return null;
}

export function analyticsContent(path: string, paths: readonly string[]) {
  if (!paths.includes(path)) return null;
  const content_type = path.startsWith("/blog/")
    ? "article"
    : path === "/blog"
      ? "blog"
      : path === "/pricing"
        ? "pricing"
        : path === "/help" || path.startsWith("/docs/")
          ? "documentation"
          : "page";
  return { content_type, content_id: path };
}

export function analyticsClick(
  href: string,
  origin: string,
  paths: readonly string[],
) {
  let url: URL;
  try {
    url = new URL(href, origin);
  } catch {
    return null;
  }
  if (url.origin === origin && url.pathname === "/app/billing" && !url.hash) {
    const entries = [...url.searchParams.entries()];
    if (
      entries.length === 1 &&
      entries[0][0] === "plan" &&
      ["start", "premium"].includes(entries[0][1])
    )
      return { name: "marketing_plan_select", params: { plan: entries[0][1] } };
    return null;
  }
  const destination = analyticsTarget(href, origin);
  if (destination)
    return { name: "marketing_cta_click", params: { destination } };
  if (url.origin === origin && !url.search && !url.hash) {
    const content = analyticsContent(url.pathname, paths);
    if (content) return { name: "marketing_content_click", params: content };
  }
  if (href === "mailto:info@xingdu.app")
    return { name: "marketing_cta_click", params: { destination: "contact" } };
  return null;
}

export function analyticsScrollDepth(
  scrollY: number,
  height: number,
  viewport: number,
) {
  if (
    ![scrollY, height, viewport].every(Number.isFinite) ||
    viewport <= 0 ||
    height <= viewport
  )
    return 0;
  return Math.min(100, Math.max(0, ((scrollY + viewport) / height) * 100));
}
