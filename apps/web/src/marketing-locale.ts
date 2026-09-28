import { createContext, useContext } from "react";
import type { Locale } from "./i18n";
import english from "./locales/marketing-en.json";

export const MarketingLocale = createContext<Locale>("zh-CN");
export function useMarketingText() {
  const locale = useContext(MarketingLocale);
  return (text: string) =>
    locale === "en"
      ? ((english as Record<string, string>)[text] ?? text)
      : text;
}
