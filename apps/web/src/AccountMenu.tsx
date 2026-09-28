import { t, useLocale } from "./i18n";
import { useId, useRef, useState, useEffect } from "react";
import type { KeyboardEvent } from "react";
import { errorMessage, roleNames } from "./api";
import type { Organization } from "./api";

import Avatar from "./Avatar";
import type { AccountSection } from "./AccountPage";

export default function AccountMenu({
  username,
  organization,
  onNavigate,
  onLogout,
}: {
  username: string;
  organization: Organization;
  onNavigate: (page: AccountSection | "members") => void;
  onLogout: () => Promise<void>;
}) {
  useLocale();
  const id = useId();
  const trigger = useRef<HTMLButtonElement>(null);
  const popup = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const close = () => {
    popup.current?.hidePopover();
    trigger.current?.focus();
  };
  function show() {
    if (open) {
      close();
      return;
    }
    const rect = trigger.current?.getBoundingClientRect();
    const menu = popup.current;
    if (!rect || !menu) return;
    const width = Math.min(264, window.innerWidth - 24);
    const above = rect.top > 330;
    Object.assign(menu.style, {
      width: `${width}px`,
      left: `${Math.max(12, Math.min(rect.left, window.innerWidth - width - 12))}px`,
      top: above ? "auto" : `${rect.bottom + 8}px`,
      bottom: above ? `${window.innerHeight - rect.top + 8}px` : "auto",
      maxHeight: `${Math.max(120, above ? rect.top - 20 : window.innerHeight - rect.bottom - 20)}px`,
    });
    menu.showPopover();
  }
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
  function keydown(event: KeyboardEvent<HTMLDivElement>) {
    const buttons = Array.from(
      popup.current?.querySelectorAll<HTMLButtonElement>(
        '[role="menuitem"]:not(:disabled)',
      ) ?? [],
    );
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
      event.preventDefault();
      const next =
        event.key === "Home"
          ? 0
          : event.key === "End"
            ? buttons.length - 1
            : (index + (event.key === "ArrowDown" ? 1 : -1) + buttons.length) %
              buttons.length;
      buttons[next]?.focus();
    } else if (event.key === "Escape") {
      event.preventDefault();
      close();
    } else if (event.key === "Tab") {
      close();
    }
  }
  const choose = (value: AccountSection) => {
    close();
    onNavigate(value);
  };
  return (
    <>
      <button
        type="button"
        ref={trigger}
        className="account-trigger"
        aria-label={t("账户菜单：{0}", { 0: username })}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={id}
        onClick={show}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown" || e.key === "ArrowUp") {
            e.preventDefault();
            show();
          }
        }}
      >
        <Avatar username={username} />
        <span className="account-trigger-name">{username}</span>
      </button>
      <div
        ref={popup}
        id={id}
        popover="auto"
        className="account-menu"
        onToggle={(event) => {
          const visible = event.newState === "open";
          setOpen(visible);
          if (visible)
            popup.current
              ?.querySelector<HTMLButtonElement>('[role="menuitem"]')
              ?.focus();
        }}
      >
        <div className="account-menu-heading">
          <strong>{username}</strong>
          <small>
            {organization.name} · {roleNames[organization.role]}
          </small>
        </div>
        <div role="menu" aria-label={t("账户操作")} onKeyDown={keydown}>
          <button role="menuitem" onClick={() => choose("profile")}>
            <span aria-hidden="true">○</span>
            {t("个人中心")}
          </button>
          <button role="menuitem" onClick={() => choose("security")}>
            <span aria-hidden="true">◇</span>
            {t("安全中心")}
          </button>
          <button
            role="menuitem"
            onClick={() => {
              close();
              onNavigate("members");
            }}
          >
            <span aria-hidden="true">♧</span>
            {t("人员管理")}
          </button>
          <button role="menuitem" onClick={() => choose("settings")}>
            <span aria-hidden="true">⚙</span>
            {t("设置")}
          </button>
          <div role="separator" className="account-menu-divider" />
          <button
            role="menuitem"
            className="account-menu-logout"
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              setError("");
              try {
                await onLogout();
              } catch (e) {
                setError(errorMessage(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            <span aria-hidden="true">↪</span>
            {busy ? t("正在退出…") : t("退出登录")}
          </button>
        </div>
        {error && (
          <p className="form-error" role="alert">
            {t(error)}
          </p>
        )}
      </div>
    </>
  );
}
