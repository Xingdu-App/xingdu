import { t, useLocale } from "./i18n";
import { useEffect, useId, useRef, useState } from "react";
import type { KeyboardEvent } from "react";

type Option = { value: string; label: string; description?: string };
type Props = {
  id?: string;
  label: string;
  value: string;
  options: Option[];
  onChange: (value: string) => void;
  disabled?: boolean;
  variant?: "default" | "organization";
  action?: { label: string; onClick: () => void };
};

// Native popovers live above dialogs and avoid clipping inside scrolling panels.
export default function Select({
  id,
  label,
  value,
  options,
  onChange,
  disabled = false,
  variant = "default",
  action,
}: Props) {
  useLocale();
  const uid = useId();
  const trigger = useRef<HTMLButtonElement>(null);
  const popup = useRef<HTMLDivElement>(null);
  const actionButton = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const search = useRef({ text: "", time: 0 });
  const selected = options.find((option) => option.value === value);
  const listId = `${uid}-list`;
  const close = () => popup.current?.hidePopover();
  function show(
    index = Math.max(
      0,
      options.findIndex((option) => option.value === value),
    ),
  ) {
    if (disabled || !options.length || !trigger.current || !popup.current)
      return;
    const rect = trigger.current.getBoundingClientRect();
    const below = window.innerHeight - rect.bottom - 12;
    const above = rect.top - 12;
    const upward = below < 220 && above > below;
    const width = Math.min(
      Math.max(rect.width, variant === "organization" ? 260 : 220),
      window.innerWidth - 24,
    );
    Object.assign(popup.current.style, {
      left: `${Math.max(12, Math.min(rect.left, window.innerWidth - width - 12))}px`,
      top: upward ? "auto" : `${rect.bottom + 6}px`,
      bottom: upward ? `${window.innerHeight - rect.top + 6}px` : "auto",
      width: `${width}px`,
      maxHeight: `${Math.max(80, Math.min(340, upward ? above : below))}px`,
    });
    setActive(index);
    popup.current.showPopover();
  }
  function choose(index: number) {
    if (disabled || !options[index]) return;
    close();
    trigger.current?.focus();
    if (options[index].value !== value) onChange(options[index].value);
  }
  function keydown(event: KeyboardEvent<HTMLButtonElement>) {
    if (event.key === "Tab") {
      if (open && action && !event.shiftKey) {
        event.preventDefault();
        actionButton.current?.focus();
        return;
      }
      close();
      return;
    }
    if (event.key === "Escape") {
      if (open) {
        event.preventDefault();
        event.stopPropagation();
        close();
      }
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      if (open) choose(active);
      else show();
    } else if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
      event.preventDefault();
      const next =
        event.key === "Home"
          ? 0
          : event.key === "End"
            ? options.length - 1
            : Math.max(
                0,
                Math.min(
                  options.length - 1,
                  active + (event.key === "ArrowDown" ? 1 : -1),
                ),
              );
      if (!open)
        show(
          event.key === "Home"
            ? 0
            : event.key === "End" || event.key === "ArrowUp"
              ? options.length - 1
              : undefined,
        );
      else setActive(next);
    } else if (
      event.key.length === 1 &&
      !event.ctrlKey &&
      !event.metaKey &&
      !event.altKey
    ) {
      search.current.text =
        (Date.now() - search.current.time < 700 ? search.current.text : "") +
        event.key.toLocaleLowerCase();
      search.current.time = Date.now();
      const index = options.findIndex((option) =>
        option.label.toLocaleLowerCase().startsWith(search.current.text),
      );
      if (index >= 0) {
        if (open) setActive(index);
        else show(index);
      }
    }
  }
  useEffect(() => {
    if (open)
      popup.current
        ?.querySelector(`[data-index="${active}"]`)
        ?.scrollIntoView({ block: "nearest" });
  }, [active, open]);
  useEffect(() => {
    if (!open) return;
    const dismiss = (event: Event) => {
      if (
        !(event.target instanceof Node) ||
        !popup.current?.contains(event.target)
      )
        popup.current?.hidePopover();
    };
    window.addEventListener("resize", dismiss);
    document.addEventListener("scroll", dismiss, true);
    return () => {
      window.removeEventListener("resize", dismiss);
      document.removeEventListener("scroll", dismiss, true);
    };
  }, [open]);
  useEffect(() => {
    if (disabled) popup.current?.hidePopover();
  }, [disabled]);
  return (
    <div className={`select-control select-${variant}`}>
      <button
        ref={trigger}
        id={id}
        type="button"
        className="select-trigger"
        role="combobox"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        aria-activedescendant={open ? `${uid}-${active}` : undefined}
        disabled={disabled || !options.length}
        onKeyDown={keydown}
        onClick={() => (open ? close() : show())}
      >
        {variant === "organization" && (
          <span className="organization-mark" aria-hidden="true">
            {selected?.label.slice(0, 1) || "✧"}
          </span>
        )}
        <span className="select-value">
          <span>{selected?.label || t("请选择")}</span>
          {selected?.description && <small>{selected.description}</small>}
        </span>
        <svg
          className="select-chevron"
          width="16"
          height="16"
          viewBox="0 0 20 20"
          fill="none"
          aria-hidden="true"
        >
          <path
            d="m6 8 4 4 4-4"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>
      <div
        ref={popup}
        popover="auto"
        className="select-popover"
        onToggle={(event) => setOpen(event.newState === "open")}
      >
        <div className="select-menu-label">{label}</div>
        <div id={listId} role="listbox" aria-label={label}>
          {options.map((option, index) => (
            <div
              key={option.value}
              id={`${uid}-${index}`}
              role="option"
              aria-selected={option.value === value}
              data-index={index}
              className={`select-option${active === index ? " is-active" : ""}`}
              onPointerMove={() => setActive(index)}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => choose(index)}
            >
              <span className="select-value">
                <span>{option.label}</span>
                {option.description && <small>{option.description}</small>}
              </span>
              {option.value === value && (
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 20 20"
                  fill="none"
                  aria-hidden="true"
                >
                  <path
                    d="m4 10 4 4 8-8"
                    stroke="currentColor"
                    strokeWidth="1.8"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              )}
            </div>
          ))}
        </div>
        {action && (
          <div className="select-menu-footer">
            <button
              ref={actionButton}
              type="button"
              className="select-menu-action"
              disabled={disabled}
              onClick={() => {
                close();
                trigger.current?.focus();
                action.onClick();
              }}
              onKeyDown={(event) => {
                if (event.key === "Escape") {
                  event.preventDefault();
                  event.stopPropagation();
                  close();
                  trigger.current?.focus();
                } else if (event.key === "Tab") {
                  close();
                  trigger.current?.focus();
                  if (event.shiftKey) event.preventDefault();
                }
              }}
            >
              {action.label}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
