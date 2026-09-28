import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { deleteHost, errorMessage, saveHost } from "./api";
import type { Host, HostInput } from "./api";

type Props = { host: Host | null; onClose: () => void; onSaved: () => void };
export default function HostDialog({ host, onClose, onSaved }: Props) {
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
              ? "删除服务器资料"
              : host
                ? "编辑服务器"
                : "添加服务器"}
          </h2>
        </div>
        <button
          className="icon-button"
          aria-label="关闭弹窗"
          onClick={onClose}
          disabled={busy}
        >
          ×
        </button>
      </div>
      {confirmDelete ? (
        <div className="delete-confirm">
          <p>确定删除“{host?.name}”的资料？</p>
          <p>
            此操作会删除资料并撤销 Agent
            身份、安装令牌和保存的凭据，不会关机或卸载实际 VPS 上的服务。
          </p>
          {error && (
            <p role="alert" className="form-error">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <button
              className="secondary"
              disabled={busy}
              onClick={() => setConfirmDelete(false)}
            >
              返回编辑
            </button>
            <button className="danger" disabled={busy} onClick={remove}>
              {busy ? "正在删除…" : "确认删除资料"}
            </button>
          </div>
        </div>
      ) : (
        <form onSubmit={submit}>
          <p className="form-hint">
            资料保存后不会自动连接 VPS。请通过“接入 / 状态”安装 Agent；修改 SSH
            连接信息后，需要重新提供凭据。
          </p>
          <fieldset disabled={busy}>
            <label htmlFor="host-name">服务器名称</label>
            <input
              id="host-name"
              autoFocus
              required
              maxLength={64}
              placeholder="例如：东京主机"
              value={input.name}
              onChange={(event) =>
                setInput({ ...input, name: event.target.value })
              }
            />
            <label htmlFor="host-address">IP 地址或主机名</label>
            <input
              id="host-address"
              required
              maxLength={253}
              placeholder="例如：vps.example.com"
              value={input.address}
              onChange={(event) =>
                setInput({ ...input, address: event.target.value })
              }
            />
            <div className="field-grid">
              <div>
                <label htmlFor="host-port">SSH 端口</label>
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
                <label htmlFor="host-user">SSH 用户名</label>
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
              标签 <small>可选，用逗号分隔</small>
            </label>
            <input
              id="host-tags"
              placeholder="例如：日本, 测试"
              value={tags}
              onChange={(event) => setTags(event.target.value)}
            />
            <label htmlFor="host-notes">
              备注 <small>可选，请勿填写密码或私钥</small>
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
              {error}
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
                删除资料
              </button>
            )}
            <button
              type="button"
              className="secondary"
              onClick={onClose}
              disabled={busy}
            >
              取消
            </button>
            <button className="primary" disabled={busy}>
              {busy ? "正在保存…" : "保存服务器"}
            </button>
          </div>
        </form>
      )}
    </dialog>
  );
}
