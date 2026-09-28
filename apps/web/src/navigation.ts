// Resource IDs are navigation hints; the API still enforces organization access.
export function clearDetail(url: URL): URL {
  for (const key of ["host", "node", "view"]) url.searchParams.delete(key);
  return url;
}
export function organizationURL(current: URL, id: string, clear = true): URL {
  const url = new URL(current);
  if (clear) clearDetail(url);
  if (id) url.searchParams.set("organization", id);
  else url.searchParams.delete("organization");
  return url;
}
export function detailURL(
  current: URL,
  detail?: { host?: string; node?: string; protocols?: boolean },
): URL {
  const url = clearDetail(new URL(current));
  if (detail?.node) url.searchParams.set("node", detail.node);
  else if (detail?.host) {
    url.searchParams.set("host", detail.host);
    if (detail.protocols) url.searchParams.set("view", "protocols");
  }
  return url;
}
export function detailFromURL(url: URL) {
  const node = url.searchParams.get("node") ?? "";
  const host = node ? "" : (url.searchParams.get("host") ?? "");
  return {
    node,
    host,
    protocols:
      !!node || (!!host && url.searchParams.get("view") === "protocols"),
  };
}
export function writeURL(url: URL, replace = false) {
  if (url.href !== window.location.href) {
    window.history[replace ? "replaceState" : "pushState"](null, "", url);
  }
}
