import { useEffect, useState } from "react";
import { request, isShadowsocks, type Deployment } from "./api";
import { t } from "./i18n";

type Revision = {
  revision: number;
  name: string;
  current: boolean;
  pending: boolean;
  created_at: string;
};
export default function RevisionEditor({
  host,
  node,
  onClose,
  onSaved,
}: {
  host: string;
  node: Deployment;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [port, setPort] = useState(node.port);
  const [name, setName] = useState(node.name),
    [serverName, setServerName] = useState(node.server_name);
  const [certificate, setCertificate] = useState(""),
    [privateKey, setPrivateKey] = useState("");
  const [rotate, setRotate] = useState(false),
    [restore, setRestore] = useState(0),
    [confirm, setConfirm] = useState(false);
  const [revisions, setRevisions] = useState<Revision[]>([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const path = `/api/v1/hosts/${host}/deployments/${node.id}`;
  useEffect(() => {
    const controller = new AbortController();
    void request<Revision[]>(
      path + "/revisions",
      "GET",
      undefined,
      controller.signal,
    )
      .then(setRevisions)
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message);
      });
    return () => controller.abort();
  }, [path]);
  return (
    <form
      className="enrollment-result"
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        setError("");
        try {
          await request(path, "PUT", {
            name,
            server_name: serverName,
            port,
            certificate,
            private_key: privateKey,
            rotate_credential: rotate,
            restore_revision: restore,
            confirm,
          });
          setPrivateKey("");
          onSaved();
          onClose();
        } catch (e) {
          setError(e instanceof Error ? e.message : String(e));
        } finally {
          setBusy(false);
        }
      }}
    >
      <h4>{t("更新节点配置")}</h4>
      <p>
        {t(
          "更新需要 Agent 0.12.0-dev 或更新版本。启动失败会尝试恢复原配置；任务成功后请更新客户端订阅。",
        )}
      </p>
      <label>
        {t("恢复历史版本")}
        <select
          value={restore}
          onChange={(e) => setRestore(Number(e.target.value))}
        >
          <option value={0}>{t("编辑当前配置")}</option>
          {revisions
            .filter((r) => !r.current && !r.pending)
            .map((r) => (
              <option key={r.revision} value={r.revision}>
                v{r.revision} · {r.name} ·{" "}
                {new Date(r.created_at).toLocaleString()}
              </option>
            ))}
        </select>
      </label>
      {!restore && (
        <>
          <label>
            {t("名称")}
            <input
              value={name}
              required
              maxLength={80}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label>
            {t("端口")}
            <input
              type="number"
              required
              min={1}
              max={65535}
              value={port}
              onChange={(e) => setPort(Number(e.target.value))}
            />
          </label>
          {!isShadowsocks(node.protocol) && (
            <>
              <label>
                {t("TLS 域名")}
                <input
                  value={serverName}
                  onChange={(e) => setServerName(e.target.value)}
                />
              </label>
              <label>
                {t("替换证书（留空保留）")}
                <textarea
                  value={certificate}
                  onChange={(e) => setCertificate(e.target.value)}
                />
              </label>
              <label>
                {t("替换私钥（不会回显）")}
                <textarea
                  value={privateKey}
                  autoComplete="off"
                  onChange={(e) => setPrivateKey(e.target.value)}
                />
              </label>
            </>
          )}
          <label>
            <input
              type="checkbox"
              checked={rotate}
              onChange={(e) => setRotate(e.target.checked)}
            />
            {t("重新生成连接凭据，旧客户端配置将失效")}
          </label>
        </>
      )}
      <label>
        <input
          type="checkbox"
          checked={confirm}
          onChange={(e) => setConfirm(e.target.checked)}
        />
        {t("确认更新，连接会短暂中断")}
      </label>
      {error && <p role="alert">{error}</p>}
      <button className="primary" disabled={busy || !confirm}>
        {t("应用配置")}
      </button>{" "}
      <button
        type="button"
        className="secondary"
        disabled={busy}
        onClick={onClose}
      >
        {t("取消")}
      </button>
    </form>
  );
}
