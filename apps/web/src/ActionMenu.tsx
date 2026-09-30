import { useEffect, useRef } from "react";
import type { ReactNode } from "react";
import { t } from "./i18n";

export default function ActionMenu({
  children,
  disabled = false,
}: {
  children: ReactNode;
  disabled?: boolean;
}) {
  const ref = useRef<HTMLDetailsElement>(null);
  useEffect(() => {
    function outside(event: PointerEvent) {
      if (
        event.target instanceof Node &&
        !ref.current?.contains(event.target)
      ) {
        if (ref.current) ref.current.open = false;
      }
    }
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, []);
  return (
    <details
      className="action-menu"
      ref={ref}
      onKeyDown={(event) => {
        if (event.key === "Escape" && ref.current?.open) {
          event.preventDefault();
          event.stopPropagation();
          ref.current.open = false;
          ref.current.querySelector("summary")?.focus();
        }
      }}
    >
      <summary
        className="button secondary compact"
        aria-disabled={disabled}
        onClick={(event) => {
          if (disabled) event.preventDefault();
        }}
      >
        {t("更多")} <span aria-hidden="true">⌄</span>
      </summary>
      <div
        className="action-menu-items"
        onClick={(event) => {
          if (
            event.target instanceof Element &&
            event.target.closest("button, a") &&
            ref.current
          ) {
            ref.current.open = false;
            ref.current.querySelector("summary")?.focus();
          }
        }}
      >
        {children}
      </div>
    </details>
  );
}
