import { useEffect, useRef } from "react";
import { setLocale, useLocale } from "./i18n";

export default function MarketingLanguage() {
  const locale = useLocale();
  const details = useRef<HTMLDetailsElement>(null);
  useEffect(() => {
    const closeOutside = (event: PointerEvent) => {
      if (
        event.target instanceof Node &&
        !details.current?.contains(event.target)
      ) {
        details.current?.removeAttribute("open");
      }
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && details.current?.open) {
        details.current.open = false;
        details.current.querySelector("summary")?.focus();
      }
    };
    document.addEventListener("pointerdown", closeOutside);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("pointerdown", closeOutside);
      document.removeEventListener("keydown", escape);
    };
  }, []);
  return (
    <details className="site-language" ref={details}>
      <summary aria-label={locale === "en" ? "Change language" : "切换语言"}>
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.6"
          aria-hidden="true"
        >
          <circle cx="12" cy="12" r="9" />
          <ellipse cx="12" cy="12" rx="4" ry="9" />
          <path d="M3 12h18" />
        </svg>
        <span>{locale === "en" ? "EN" : "中文"}</span>
        <span className="site-language-chevron" aria-hidden="true">
          ⌄
        </span>
      </summary>
      <div
        className="site-language-options"
        role="group"
        aria-label="Language / 语言"
      >
        {(
          [
            ["zh-CN", "简体中文"],
            ["en", "English"],
          ] as const
        ).map(([value, label]) => (
          <button
            key={value}
            type="button"
            lang={value}
            aria-pressed={locale === value}
            onClick={() => {
              setLocale(value);
              if (details.current) {
                details.current.open = false;
                details.current.querySelector("summary")?.focus();
              }
            }}
          >
            {label}
            <span aria-hidden="true">{locale === value ? "✓" : ""}</span>
          </button>
        ))}
      </div>
    </details>
  );
}
