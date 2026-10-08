import { useEffect, useRef, useState } from "react";
import { request, errorMessage } from "./api";
import { t, useLocale } from "./i18n";
import Select from "./Select";
import "./ExternalProxyPanel.css";

export type ExternalProxy = {
  id: string;
  name: string;
  address: string;
  port: number;
  protocol: "socks" | "http";
  revision: number;
};
const empty = {
  name: "",
  address: "",
  port: "1080",
  protocol: "socks",
  username: "",
  password: "",
};
export default function ExternalProxyPanel({
  proxies,
  loading,
  error,
  manage,
  onChanged,
}: {
  proxies: ExternalProxy[];
  loading: boolean;
  error: string;
  manage: boolean;
  onChanged: () => void;
}) {
  useLocale();
  const dialog = useRef<HTMLDialogElement>(null);
  const controller = useRef<AbortController | null>(null);
  const [editing, setEditing] = useState<ExternalProxy | null>(null);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(empty);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState("");
  const [deleting, setDeleting] = useState("");
  useEffect(() => {
    const c = new AbortController();
    controller.current = c;
    return () => c.abort();
  }, []);
  useEffect(() => {
    if (open) dialog.current?.showModal();
    else dialog.current?.close();
  }, [open]);
  function close() {
    if (busy) return;
    setOpen(false);
    setForm(empty);
    setEditing(null);
    setFailure("");
  }
  async function remove(id: string) {
    const signal = controller.current?.signal;
    if (!signal || busy) return;
    setBusy(true);
    setFailure("");
    try {
      await request(
        `/api/v1/external-proxies/${id}`,
        "DELETE",
        undefined,
        signal,
      );
      if (!signal.aborted) {
        setDeleting("");
        onChanged();
      }
    } catch (e) {
      if (!signal.aborted) setFailure(errorMessage(e));
    } finally {
      if (!signal.aborted) setBusy(false);
    }
  }
  const complete =
    !!form.name.trim() &&
    !!form.address.trim() &&
    Number(form.port) >= 1 &&
    Number(form.port) <= 65535 &&
    (editing
      ? !!form.username === !!form.password
      : !!form.username && !!form.password);
  return (
    <section className="external-proxy-panel">
      <header className="routes-heading">
        <div>
          <h3>{t("外部代理出口")}</h3>
          <p>
            {t(
              "接入购买的 SOCKS5 或 HTTP CONNECT 代理，再为入口节点选择这个出口。仅转发 TCP，需要 Agent 0.18.0-dev 或更新版本。",
            )}
          </p>
        </div>
        {manage && (
          <button
            type="button"
            className="primary"
            disabled={busy}
            onClick={() => {
              setEditing(null);
              setForm(empty);
              setFailure("");
              setOpen(true);
            }}
          >
            {t("添加出口")}
          </button>
        )}
      </header>
      {error && (
        <p role="alert" className="form-error">
          {t(error)}
        </p>
      )}
      {!open && failure && (
        <p role="alert" className="form-error">
          {t(failure)}
        </p>
      )}
      {loading ? (
        <div
          className="external-proxy-skeleton"
          aria-busy="true"
          aria-label={t("正在加载…")}
        >
          <span />
          <span />
        </div>
      ) : !proxies.length ? (
        <p className="form-hint">{t("暂无外部出口")}</p>
      ) : (
        <div className="external-proxy-list">
          {proxies.map((p) => (
            <article key={p.id} className="external-proxy-row">
              <div>
                <strong>{p.name}</strong>
                <p>
                  {p.protocol === "socks" ? "SOCKS5" : "HTTP CONNECT"} ·{" "}
                  {p.address.includes(":") ? `[${p.address}]` : p.address}:
                  {p.port}
                </p>
              </div>
              {manage && (
                <div className="external-proxy-actions">
                  {deleting === p.id ? (
                    <>
                      <span>{t("确认删除此出口？")}</span>
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => void remove(p.id)}
                      >
                        {t("确认删除")}
                      </button>
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => setDeleting("")}
                      >
                        {t("取消")}
                      </button>
                    </>
                  ) : (
                    <>
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => {
                          setEditing(p);
                          setForm({
                            name: p.name,
                            address: p.address,
                            port: String(p.port),
                            protocol: p.protocol,
                            username: "",
                            password: "",
                          });
                          setFailure("");
                          setOpen(true);
                        }}
                      >
                        {t("编辑")}
                      </button>
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => setDeleting(p.id)}
                      >
                        {t("删除")}
                      </button>
                    </>
                  )}
                </div>
              )}
            </article>
          ))}
        </div>
      )}
      <dialog
        ref={dialog}
        className="host-dialog machine-dialog external-proxy-dialog"
        aria-labelledby="external-proxy-title"
        onCancel={(e) => {
          e.preventDefault();
          close();
        }}
        onClose={() => {
          if (!busy) close();
        }}
      >
        <div className="dialog-heading">
          <h2 id="external-proxy-title">
            {t(editing ? "编辑外部出口" : "添加外部出口")}
          </h2>
          <button
            type="button"
            className="icon-button"
            disabled={busy}
            aria-label={t("关闭")}
            onClick={close}
          >
            ×
          </button>
        </div>
        <form
          id="external-proxy-form"
          className="external-proxy-form"
          onSubmit={async (e) => {
            e.preventDefault();
            const signal = controller.current?.signal;
            if (!signal || !complete || busy) return;
            setBusy(true);
            setFailure("");
            const body = {
              name: form.name.trim(),
              address: form.address.trim(),
              port: Number(form.port),
              protocol: form.protocol,
              ...(form.username && form.password
                ? { username: form.username, password: form.password }
                : {}),
            };
            try {
              await request(
                editing
                  ? `/api/v1/external-proxies/${editing.id}`
                  : "/api/v1/external-proxies",
                editing ? "PUT" : "POST",
                body,
                signal,
              );
              if (!signal.aborted) {
                setOpen(false);
                setForm(empty);
                setEditing(null);
                onChanged();
              }
            } catch (e) {
              if (!signal.aborted) setFailure(errorMessage(e));
            } finally {
              if (!signal.aborted) setBusy(false);
            }
          }}
        >
          <label>
            {t("名称")}
            <input
              required
              maxLength={64}
              value={form.name}
              disabled={busy}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
            />
          </label>
          <div className="external-proxy-fields">
            <label>
              {t("公网 IP")}
              <input
                required
                value={form.address}
                placeholder="IPv4 / IPv6"
                disabled={busy}
                onChange={(e) => setForm({ ...form, address: e.target.value })}
              />
            </label>
            <label>
              {t("端口")}
              <input
                required
                type="number"
                min={1}
                max={65535}
                value={form.port}
                disabled={busy}
                onChange={(e) => setForm({ ...form, port: e.target.value })}
              />
            </label>
          </div>
          <div>
            <label htmlFor="external-proxy-protocol">{t("协议")}</label>
            <Select
              id="external-proxy-protocol"
              label={t("协议")}
              value={form.protocol}
              disabled={busy}
              options={[
                { value: "socks", label: "SOCKS5" },
                { value: "http", label: "HTTP CONNECT" },
              ]}
              onChange={(value) => setForm({ ...form, protocol: value })}
            />
          </div>
          <label>
            {t("代理用户名")}
            <input
              autoComplete="off"
              required={!editing}
              maxLength={255}
              value={form.username}
              disabled={busy}
              onChange={(e) => setForm({ ...form, username: e.target.value })}
            />
          </label>
          <label>
            {t("代理密码")}
            <input
              type="password"
              autoComplete="new-password"
              required={!editing}
              maxLength={255}
              value={form.password}
              disabled={busy}
              onChange={(e) => setForm({ ...form, password: e.target.value })}
            />
          </label>
          <p className="form-hint">
            {t(
              editing
                ? "账号密码均留空时保留原凭据；替换时请同时填写两项。"
                : "账号密码加密保存，不会显示在列表或客户端订阅中。",
            )}
          </p>
          <p className="form-hint">
            {t(
              "正在使用的出口不能修改或删除。请先切换相关入口线路，并等待更新完成。",
            )}
          </p>
          {failure && (
            <p className="form-error" role="alert">
              {t(failure)}
            </p>
          )}
        </form>
        <div className="dialog-footer">
          <button type="button" disabled={busy} onClick={close}>
            {t("取消")}
          </button>
          <button
            type="submit"
            form="external-proxy-form"
            className="primary"
            disabled={busy || !complete}
          >
            {busy ? t("正在保存…") : t("保存")}
          </button>
        </div>
      </dialog>
    </section>
  );
}
