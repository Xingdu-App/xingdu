import { useSyncExternalStore } from "react";
import english from "./locales/en.json";

export type Locale = "zh-CN" | "en";
const key = "xingdu.locale";
const listeners = new Set<() => void>();
function applyDocumentLocale(next: Locale) {
  // Marketing pages are rendered separately and keep their own language.
  if (/^\/(app(?:\/|$)|login(?:\/|$))/.test(window.location.pathname))
    document.documentElement.lang = next;
}
function storedLocale(): Locale {
  try {
    return localStorage.getItem(key) === "en" ? "en" : "zh-CN";
  } catch {
    return "zh-CN";
  }
}
let locale: Locale = typeof window === "undefined" ? "zh-CN" : storedLocale();
function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
export function useLocale() {
  return useSyncExternalStore(
    subscribe,
    () => locale,
    () => "zh-CN" as Locale,
  );
}
export function setLocale(next: Locale) {
  locale = next;
  try {
    localStorage.setItem(key, next);
  } catch {
    /* Private browsing may block storage. */
  }
  applyDocumentLocale(next);
  listeners.forEach((listener) => listener());
}
export const localeTag = () => (locale === "en" ? "en-US" : "zh-CN");
export function t(text: string, values: Record<string, unknown> = {}): string {
  const translated =
    locale === "en"
      ? ((english as Record<string, string>)[text] ?? text)
      : text;
  return translated.replace(/\{(\d+)\}/g, (match, index: string) =>
    index in values ? String(values[index] ?? "") : match,
  );
}
if (typeof window !== "undefined") {
  applyDocumentLocale(locale);
  window.addEventListener("storage", (event) => {
    if (event.key !== key && event.key !== null) return;
    const next = storedLocale();
    if (next === locale) return;
    locale = next;
    applyDocumentLocale(next);
    listeners.forEach((listener) => listener());
  });
}
