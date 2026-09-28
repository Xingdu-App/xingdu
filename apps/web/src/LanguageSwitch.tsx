import { setLocale, useLocale } from "./i18n";

export default function LanguageSwitch() {
  const locale = useLocale();
  return (
    <div
      className="language-switch"
      role="group"
      aria-label={locale === "en" ? "Language" : "语言"}
    >
      <span aria-hidden="true">◎</span>
      <button
        type="button"
        lang="zh-CN"
        aria-pressed={locale === "zh-CN"}
        onClick={() => setLocale("zh-CN")}
      >
        中文
      </button>
      <span aria-hidden="true" className="language-divider">
        /
      </span>
      <button
        type="button"
        lang="en"
        aria-pressed={locale === "en"}
        onClick={() => setLocale("en")}
      >
        English
      </button>
    </div>
  );
}
