import { useEffect, useState } from "react";
import {
  request,
  requiresTLS,
  handshakeHosts,
  type Deployment,
  type V2RayOptions,
  deploymentConnection,
} from "./api";
import { t } from "./i18n";
import Select from "./Select";
import V2RayFields, { defaultV2Ray } from "./V2RayFields";

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
  const advanced = ["vless", "vmess", "trojan"].includes(node.protocol);
  const [v2ray, setV2ray] = useState<V2RayOptions>();
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
      className="enrollment-result revision-editor"
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
            v2ray,
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
      <div className="revision-field">
        <label htmlFor={`revision-${node.id}`}>{t("恢复历史版本")}</label>
        <Select
          id={`revision-${node.id}`}
          label={t("恢复历史版本")}
          value={String(restore)}
          onChange={(value) => setRestore(Number(value))}
          disabled={busy}
          options={[
            { value: "0", label: t("编辑当前配置") },
            ...revisions
              .filter((r) => !r.current && !r.pending)
              .map((r) => ({
                value: String(r.revision),
                label: `v${r.revision} · ${r.name}`,
                description: new Date(r.created_at).toLocaleString(),
              })),
          ]}
        />
      </div>
      {!restore && (
        <>
          <label className="revision-field">
            {t("名称")}
            <input
              value={name}
              required
              maxLength={80}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label className="revision-field">
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
          {advanced && !v2ray && (
            <button
              type="button"
              className="secondary"
              disabled={busy}
              onClick={async () => {
                setBusy(true);
                setError("");
                try {
                  const config = await deploymentConnection(host, node.id);
                  setV2ray(config.v2ray || defaultV2Ray);
                  setServerName(config.server_name);
                } catch (e) {
                  setError(e instanceof Error ? e.message : String(e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              {t("编辑传输配置")}
            </button>
          )}
          {v2ray && (
            <V2RayFields
              protocol={node.protocol}
              value={v2ray}
              onChange={(value) => {
                setV2ray(value);
                if (value.reality && !handshakeHosts.includes(serverName))
                  setServerName(handshakeHosts[0]);
              }}
            />
          )}
          {v2ray && (v2ray.reality || node.protocol === "trojan") && (
            <p>
              {t(
                "REALITY 与 Trojan 传输配置需要 Agent 0.17.0-dev 或更新版本。",
              )}
            </p>
          )}
          {(node.protocol === "shadowtls" || v2ray?.reality) && (
            <div className="revision-field">
              <label htmlFor={`handshake-${node.id}`}>{t("握手域名")}</label>
              <Select
                id={`handshake-${node.id}`}
                label={t("握手域名")}
                value={serverName}
                onChange={setServerName}
                disabled={busy}
                options={handshakeHosts.map((host) => ({
                  value: host,
                  label: host,
                }))}
              />
            </div>
          )}
          {requiresTLS(node.protocol) &&
            !v2ray?.reality &&
            (!advanced || !!v2ray) &&
            v2ray?.tls !== false && (
              <>
                <label className="revision-field">
                  {t("TLS 域名")}
                  <input
                    value={serverName}
                    onChange={(e) => setServerName(e.target.value)}
                  />
                </label>
                <label className="revision-field">
                  {t("替换证书（留空保留）")}
                  <textarea
                    value={certificate}
                    onChange={(e) => setCertificate(e.target.value)}
                  />
                </label>
                <label className="revision-field">
                  {t("替换私钥（不会回显）")}
                  <textarea
                    value={privateKey}
                    autoComplete="off"
                    onChange={(e) => setPrivateKey(e.target.value)}
                  />
                </label>
              </>
            )}
          <label className="check-row">
            <input
              type="checkbox"
              checked={rotate}
              onChange={(e) => setRotate(e.target.checked)}
            />
            {t("重新生成连接凭据，旧客户端配置将失效")}
          </label>
        </>
      )}
      <label className="check-row">
        <input
          type="checkbox"
          checked={confirm}
          onChange={(e) => setConfirm(e.target.checked)}
        />
        {t("确认更新，连接会短暂中断")}
      </label>
      {error && <p role="alert">{error}</p>}
      <div className="deployment-actions">
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
      </div>
    </form>
  );
}
