export function subscriptionImportURL(
  format: string,
  url: string,
): string | null {
  const parsed = new URL(url);
  if (!["https:", "http:"].includes(parsed.protocol)) return null;
  if (format === "stash")
    return `stash://install-config?url=${encodeURIComponent(url)}`;
  if (format === "surge")
    return `surge:///install-config?url=${encodeURIComponent(url)}`;
  return null;
}

export async function copySubscriptionURL(value: string): Promise<boolean> {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(value);
      return true;
    }
  } catch {
    /* Fall back when browser permission denies the modern API. */
  }
  const field = document.createElement("textarea");
  field.value = value;
  field.style.cssText = "position:fixed;left:-9999px;top:0";
  const previous = document.activeElement as HTMLElement | null;
  document.body.appendChild(field);
  field.select();
  try {
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    field.remove();
    previous?.focus();
  }
}
