import { t, useLocale } from "./i18n";
import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { deleteHost, errorMessage, saveHost } from "./api";
import type { Host, HostInput } from "./api";

type Props = { host: Host | null; onClose: () => void; onSaved: () => void };
export default function HostDialog({ host, onClose, onSaved }: Props) {
  useLocale();
  const dialog = useRef<HTMLDialogElement>(null);
  const [input, setInput] = useState<HostInput>(
    host
      ? {
          name: host.name,
          address: host.address,
          ssh_port: host.ssh_port,
          ssh_user: host.ssh_user,
          tags: host.tags,
          notes: host.notes,
        }
      : {
          name: "",
          address: "",
          ssh_port: 22,
          ssh_user: "root",
          tags: [],
          notes: "",
        },
  );
  const [tags, setTags] = useState(host?.tags.join(", ") ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  useEffect(() => {
    const element = dialog.current;
    element?.showModal();
    element?.querySelector<HTMLInputElement>("#host-name")?.focus();
    return () => element?.close();
  }, []);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setError("");
    setBusy(true);
    try {
      await saveHost(
        {
          ...input,
          tags: tags
            .split(/[,，]/)
            .map((tag) => tag.trim())
            .filter(Boolean),
        },
        host?.id,
      );
      onSaved();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    if (!host || busy) return;
    setBusy(true);
    setError("");
    try {
      await deleteHost(host.id);
      onSaved();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  }
  return (
    <dialog
      className="host-dialog"
      ref={dialog}
      aria-labelledby="host-dialog-title"
      onCancel={(event) => {
        event.preventDefault();
        if (!busy) onClose();
      }}
    >
      <div className="dialog-heading">
        <div>
          <p className="eyebrow">SERVER INVENTORY</p>
          <h2 id="host-dialog-title">
            {confirmDelete
              ? t("删除服务器资料")
              : host
                ? t("编辑服务器")
                : t("添加服务器")}
          </h2>
        </div>
        <button
          className="icon-button"
          aria-label={t("关闭弹窗")}
          onClick={onClose}
          disabled={busy}
        >
          ×
        </button>
      </div>
      {confirmDelete ? (
        <div className="delete-confirm">
          <p>
            {t("确定删除“")}
            {host?.name}
            {t("”的资料？")}
          </p>
          <p>
            {t(
              "此操作会删除资料并撤销 Agent 身份、安装令牌和保存的凭据，不会关机或停止实际 VPS 上的协议服务。如需停止服务，请先在「协议部署」中卸载并确认完成，再删除资料。",
            )}
          </p>
          {error && (
            <p role="alert" className="form-error">
              {t(error)}
            </p>
          )}
          <div className="dialog-actions">
            <button
              className="secondary"
              disabled={busy}
              onClick={() => setConfirmDelete(false)}
            >
              {t("返回编辑")}
            </button>
            <button className="danger" disabled={busy} onClick={remove}>
              {busy ? t("正在删除…") : t("确认删除资料")}
            </button>
          </div>
        </div>
      ) : (
        <form onSubmit={submit}>
          <p className="form-hint">
            {t(
              "资料保存后不会自动连接 VPS。请通过“接入 / 状态”安装 Agent；修改 SSH 连接信息后，需要重新提供凭据。",
            )}
          </p>
          <fieldset disabled={busy}>
            <label htmlFor="host-name">{t("服务器名称")}</label>
            <input
              id="host-name"
              autoFocus
              required
              maxLength={64}
              placeholder={t("例如：东京主机")}
              value={input.name}
              onChange={(event) =>
                setInput({ ...input, name: event.target.value })
              }
            />
            <label htmlFor="host-address">{t("IP 地址或主机名")}</label>
            <input
              id="host-address"
              required
              maxLength={253}
              placeholder={t("例如：vps.example.com")}
              value={input.address}
              onChange={(event) =>
                setInput({ ...input, address: event.target.value })
              }
            />
            <div className="field-grid">
              <div>
                <label htmlFor="host-port">{t("SSH 端口")}</label>
                <input
                  id="host-port"
                  type="number"
                  required
                  min={1}
                  max={65535}
                  value={input.ssh_port || ""}
                  onChange={(event) =>
                    setInput({ ...input, ssh_port: Number(event.target.value) })
                  }
                />
              </div>
              <div>
                <label htmlFor="host-user">{t("SSH 用户名")}</label>
                <input
                  id="host-user"
                  required
                  maxLength={32}
                  value={input.ssh_user}
                  onChange={(event) =>
                    setInput({ ...input, ssh_user: event.target.value })
                  }
                />
              </div>
            </div>
            <label htmlFor="host-tags">
              {t("标签")}
              <small>{t("可选，用逗号分隔")}</small>
            </label>
            <input
              id="host-tags"
              placeholder={t("例如：日本, 测试")}
              value={tags}
              onChange={(event) => setTags(event.target.value)}
            />
            <label htmlFor="host-notes">
              {t("备注")}
              <small>{t("可选，请勿填写密码或私钥")}</small>
            </label>
            <textarea
              id="host-notes"
              rows={3}
              maxLength={1000}
              value={input.notes}
              onChange={(event) =>
                setInput({ ...input, notes: event.target.value })
              }
            />
          </fieldset>
          {error && (
            <p role="alert" className="form-error">
              {t(error)}
            </p>
          )}
          <div className="dialog-actions">
            {host && (
              <button
                type="button"
                className="text-danger"
                disabled={busy}
                onClick={() => setConfirmDelete(true)}
              >
                {t("删除资料")}
              </button>
            )}
            <button
              type="button"
              className="secondary"
              onClick={onClose}
              disabled={busy}
            >
              {t("取消")}
            </button>
            <button className="primary" disabled={busy}>
              {busy ? t("正在保存…") : t("保存服务器")}
            </button>
          </div>
        </form>
      )}
    </dialog>
  );
}
