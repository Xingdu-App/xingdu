import { createContext, useContext } from "react";
import type { Locale } from "./i18n";
import english from "./locales/marketing-en.json";

export const MarketingLocale = createContext<Locale>("zh-CN");
export function translateMarketing(text: string, locale: Locale) {
  return locale === "en"
    ? ((english as Record<string, string>)[text] ?? text)
    : text;
}
export function useMarketingText() {
  const locale = useContext(MarketingLocale);
  return (text: string) => translateMarketing(text, locale);
}
