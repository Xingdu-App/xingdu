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
