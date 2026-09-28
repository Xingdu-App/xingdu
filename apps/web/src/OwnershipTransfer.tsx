import { t, useLocale } from "./i18n";
import { useState } from "react";
import { errorMessage, request } from "./api";
import type { Member } from "./api";
import Select from "./Select";
import "./AccountForms.css";
export default function OwnershipTransfer({
  members,
  organizationName,
}: {
  members: Member[];
  organizationName: string;
}) {
  useLocale();
  const [open, setOpen] = useState(false);
  const [target, setTarget] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const options = members
    .filter((m) => m.role !== "owner")
    .map((m) => ({ value: m.id, label: m.username }));
  const selected = options.find((m) => m.value === target);
  if (!open)
    return (
      <button
        className="secondary"
        disabled={!options.length}
        onClick={() => setOpen(true)}
      >
        {t("转移组织所有权")}
      </button>
    );
  return (
    <form
      className="account-form"
      onSubmit={async (e) => {
        e.preventDefault();
        if (!selected || confirm !== selected.label) return;
        setBusy(true);
        setError("");
        try {
          await request<void>("/api/v1/organization/ownership", "POST", {
            target_id: target,
            current_password: password,
            confirmation: "TRANSFER",
          });
          setPassword("");
          window.location.reload();
        } catch (e) {
          setError(errorMessage(e));
          setBusy(false);
        }
      }}
    >
      <h3>{t("转移「{0}」的所有权", { 0: organizationName })}</h3>
      <p className="form-hint">
        {t(
          "新所有者将获得最高组织权限，你将变为管理员。转移后只有新所有者可以再次转移所有权。",
        )}
      </p>
      <Select
        label={t("新所有者")}
        value={target}
        options={options}
        onChange={(value) => {
          setTarget(value);
          setConfirm("");
        }}
        disabled={busy}
      />
      <label>
        {t("输入新所有者用户名确认")}
        <input
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          required
          disabled={busy}
          autoComplete="off"
        />
      </label>
      <label>
        {t("你的当前密码")}
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          disabled={busy}
          autoComplete="current-password"
        />
      </label>
      {error && <p role="alert">{error}</p>}
      <div className="member-actions">
        <button
          className="secondary"
          type="button"
          disabled={busy}
          onClick={() => {
            setOpen(false);
            setPassword("");
            setConfirm("");
            setError("");
          }}
        >
          {t("取消")}
        </button>
        <button
          className="secondary"
          disabled={busy || !selected || confirm !== selected.label}
        >
          {busy ? t("转移中…") : t("确认转移所有权")}
        </button>
      </div>
    </form>
  );
}
