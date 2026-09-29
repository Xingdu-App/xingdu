// Only explicit public routes and fixed event labels may enter analytics.
export function analyticsPage(href: string, paths: readonly string[]) {
  const url = new URL(href);
  if (!paths.includes(url.pathname) || url.search || url.hash) return null;
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
