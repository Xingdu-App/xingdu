import { useEffect, useRef, useState } from "react";
import { request, errorMessage, protocolNames, type ManagedNode } from "./api";
import { t, useLocale } from "./i18n";
import Select from "./Select";
import ExternalProxyPanel, { type ExternalProxy } from "./ExternalProxyPanel";

export default function RoutePanel({
  nodes,
  manage,
  onChanged,
}: {
  nodes: ManagedNode[];
  manage: boolean;
  onChanged: () => void;
}) {
  useLocale();
  const [proxies, setProxies] = useState<ExternalProxy[]>([]);
  const [proxyLoading, setProxyLoading] = useState(true);
  const [proxyError, setProxyError] = useState("");
  const [reload, setReload] = useState(0);
  useEffect(() => {
    const c = new AbortController();
    setProxyLoading(true);
    setProxyError("");
    request<ExternalProxy[]>(
      "/api/v1/external-proxies",
      "GET",
      undefined,
      c.signal,
    )
      .then((result) => {
        if (!c.signal.aborted) setProxies(result);
      })
      .catch((e) => {
        if (!c.signal.aborted) {
          setProxies([]);
          setProxyError(errorMessage(e));
        }
      })
      .finally(() => {
        if (!c.signal.aborted) setProxyLoading(false);
      });
    return () => c.abort();
  }, [reload]);
  const [entry, setEntry] = useState("");
  const [exit, setExit] = useState("");
  const [confirm, setConfirm] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const lifecycle = useRef<AbortController | null>(null);
  useEffect(() => {
    const c = new AbortController();
    lifecycle.current = c;
    return () => c.abort();
  }, []);
  const eligible = nodes.filter(
    (n) =>
      n.state === "succeeded" &&
      n.action === "deploy" &&
      n.protocol !== "trusttunnel",
  );
  const source = eligible.find((n) => n.id === entry);
  const exits = eligible.filter(
    (n) =>
      n.host_id !== source?.host_id &&
      !n.relay_exit_id &&
      !n.external_exit_id &&
      n.protocol !== "trusttunnel",
  );
  const destination = exits.find((n) => n.id === exit);
  const externalDestination = proxies.find((p) => `external:${p.id}` === exit);
  const valid = !!source && (!exit || !!destination || !!externalDestination);
  const unchanged =
    !!source &&
    (source.external_exit_id
      ? `external:${source.external_exit_id}`
      : source.relay_exit_id || "") === exit;
  return (
    <section className="panel routes-panel">
      <header className="routes-heading">
        <div>
          <h2>{t("直连与中转线路")}</h2>
          <p>
            {t(
              "客户端连接入口，由入口转发到自管节点或外部代理出口。单层中转仅转发 TCP；自管节点中转需要 Agent 0.12.0-dev，外部出口需要 Agent 0.18.0-dev。",
            )}
          </p>
        </div>
        <span className="badge">TCP</span>
      </header>
      <ExternalProxyPanel
        proxies={proxies}
        loading={proxyLoading}
        error={proxyError}
        manage={manage}
        onChanged={() => setReload((n) => n + 1)}
      />
      {manage && (
        <form
          className="route-editor"
          onSubmit={async (e) => {
            e.preventDefault();
            const controller = lifecycle.current;
            if (
              !valid ||
              !source ||
              !confirm ||
              busy ||
              unchanged ||
              !controller ||
              controller.signal.aborted
            )
              return;
            setBusy(true);
            setMessage("");
            setError("");
            try {
              await request(
                `/api/v1/hosts/${source.host_id}/deployments/${source.id}`,
                "PUT",
                {
                  exit_node_id: externalDestination ? "" : exit,
                  external_exit_id: externalDestination?.id || "",
                  confirm,
                },
                controller.signal,
              );
              if (!controller.signal.aborted) {
                setMessage(t("线路更新已排队，请等待 Agent 确认后刷新订阅。"));
                setConfirm(false);
                onChanged();
              }
            } catch (e) {
              if (!controller.signal.aborted) setError(errorMessage(e));
            } finally {
              if (!controller.signal.aborted) setBusy(false);
            }
          }}
        >
          <h3>{t("配置线路")}</h3>
          <div className="route-fields">
            <div className="route-field">
              <label htmlFor="route-entry">{t("入口节点")}</label>
              <Select
                id="route-entry"
                label={t("入口节点")}
                value={source ? entry : ""}
                disabled={busy}
                options={[
                  { value: "", label: t("选择节点") },
                  ...eligible.map((n) => ({
                    value: n.id,
                    label: n.name,
                    description: n.host_name,
                  })),
                ]}
                onChange={(value) => {
                  setEntry(value);
                  setExit(
                    (() => {
                      const selected = nodes.find((n) => n.id === value);
                      return selected?.external_exit_id
                        ? `external:${selected.external_exit_id}`
                        : selected?.relay_exit_id || "";
                    })(),
                  );
                  setConfirm(false);
                  setError("");
                  setMessage("");
                }}
              />
            </div>
            <span className="route-direction" aria-hidden="true">
              →
            </span>
            <div className="route-field">
              <label htmlFor="route-exit">{t("出口节点")}</label>
              <Select
                id="route-exit"
                label={t("出口节点")}
                value={exit}
                disabled={busy || !source}
                options={[
                  { value: "", label: t("直连（取消中转）") },
                  ...exits.map((n) => ({
                    value: n.id,
                    label: n.name,
                    description: n.host_name,
                  })),
                  ...proxies.map((p) => ({
                    value: `external:${p.id}`,
                    label: p.name,
                    description: `${t("外部出口")} · ${p.protocol === "socks" ? "SOCKS5" : "HTTP CONNECT"}`,
                  })),
                  ...(exit && !destination && !externalDestination
                    ? [{ value: exit, label: t("出口不可用") }]
                    : []),
                ]}
                onChange={(value) => {
                  setExit(value);
                  setConfirm(false);
                }}
              />
            </div>
          </div>
          {!eligible.length && (
            <p className="form-hint">
              {t("暂无可配置节点，请先完成节点部署。")}
            </p>
          )}
          {source && (
            <div className="route-preview">
              <span>{t("客户端")}</span>
              <span aria-hidden="true">→</span>
              <strong>{source.name}</strong>
              <span aria-hidden="true">→</span>
              <strong>
                {exit
                  ? (destination?.name ??
                    externalDestination?.name ??
                    t("出口不可用"))
                  : t("直连出口")}
              </strong>
            </div>
          )}
          <p className="form-hint">
            {t(
              "出口被线路使用时不能编辑或卸载。请先将入口改为直连或切换出口，并等待更新完成。",
            )}
          </p>
          <div className="route-editor-footer">
            <label className="check-row">
              <input
                type="checkbox"
                checked={confirm}
                disabled={busy || !valid || unchanged}
                onChange={(e) => setConfirm(e.target.checked)}
              />
              {t("确认更新，连接会短暂中断")}
            </label>
            <button
              className="primary"
              disabled={busy || !confirm || !valid || unchanged}
            >
              {busy ? t("正在保存…") : t("应用线路")}
            </button>
          </div>
          {error && (
            <p className="form-error" role="alert">
              {t(error)}
            </p>
          )}
          {message && (
            <p className="notice" role="status">
              {t(message)}
            </p>
          )}
        </form>
      )}
      <div className="routes-list-heading">
        <h3>{t("当前线路")}</h3>
        <span className="badge">{nodes.length}</span>
      </div>
      {!nodes.length ? (
        <div className="empty-state">
          <h3>{t("暂无线路")}</h3>
          <p>{t("完成节点部署后，可在这里查看直连线路或配置中转。")}</p>
        </div>
      ) : (
        <div className="route-grid">
          {nodes.map((n) => {
            const target = nodes.find((x) => x.id === n.relay_exit_id);
            const external = proxies.find((p) => p.id === n.external_exit_id);
            return (
              <article className="node-card route-card" key={n.id}>
                <div className="node-card-top">
                  <span className="node-protocol">
                    {protocolNames[n.protocol]}
                  </span>
                  <span className="badge">
                    {n.relay_exit_id || n.external_exit_id
                      ? t("中转线路")
                      : t("直连线路")}
                  </span>
                </div>
                <div className="route-path">
                  <div>
                    <small>{t("入口节点")}</small>
                    <h3>{n.name}</h3>
                    <p>{n.host_name}</p>
                  </div>
                  <span className="route-direction" aria-hidden="true">
                    →
                  </span>
                  <div>
                    <small>{t("出口节点")}</small>
                    <h3>
                      {n.relay_exit_id || n.external_exit_id
                        ? (target?.name ?? external?.name ?? t("出口不可用"))
                        : t("直连出口")}
                    </h3>
                    {target && <p>{target.host_name}</p>}
                    {external && <p>{t("外部出口")}</p>}
                  </div>
                </div>
                <p className="route-status">
                  {n.action === "update"
                    ? t("配置更新中或待核实")
                    : n.state === "succeeded" && n.action === "deploy"
                      ? t("配置已部署，连接需探测确认")
                      : t("节点状态需核实")}
                </p>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
