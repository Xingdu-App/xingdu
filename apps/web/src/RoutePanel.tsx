import { useState } from "react";
import { request, type ManagedNode } from "./api";
import { t } from "./i18n";
export default function RoutePanel({
  nodes,
  manage,
  onChanged,
}: {
  nodes: ManagedNode[];
  manage: boolean;
  onChanged: () => void;
}) {
  const [entry, setEntry] = useState(""),
    [exit, setExit] = useState(""),
    [confirm, setConfirm] = useState(false),
    [busy, setBusy] = useState(false),
    [message, setMessage] = useState("");
  const source = nodes.find((n) => n.id === entry);
  return (
    <section className="panel">
      <h2>{t("直连与中转线路")}</h2>
      <p>
        {t(
          "客户端连接入口，由入口转发到出口。单层中转当前仅转发 TCP；两端都需要已部署节点，入口需要 Agent 0.12.0-dev。",
        )}
      </p>
      <div className="node-grid">
        {nodes.map((n) => (
          <article className="node-card" key={n.id}>
            <strong>{n.name}</strong>
            <p>
              {n.host_name} →{" "}
              {n.relay_exit_id
                ? (nodes.find((x) => x.id === n.relay_exit_id)?.name ??
                  t("出口不可用"))
                : t("直连出口")}
            </p>
            <p>
              {n.action === "update"
                ? t("配置更新中或待核实")
                : t("配置已部署，连接需探测确认")}
            </p>
          </article>
        ))}
      </div>
      {manage && (
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            if (!source) return;
            setBusy(true);
            setMessage("");
            try {
              await request(
                `/api/v1/hosts/${source.host_id}/deployments/${source.id}`,
                "PUT",
                { exit_node_id: exit, confirm },
              );
              setMessage(t("线路更新已排队，请等待 Agent 确认后刷新订阅。"));
              setConfirm(false);
              onChanged();
            } catch (e) {
              setMessage(e instanceof Error ? e.message : String(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          <label>
            {t("入口节点")}
            <select
              required
              value={entry}
              onChange={(e) => {
                setEntry(e.target.value);
                setExit("");
              }}
            >
              <option value="">{t("选择节点")}</option>
              {nodes
                .filter((n) => n.state === "succeeded" && n.action === "deploy")
                .map((n) => (
                  <option key={n.id} value={n.id}>
                    {n.name} · {n.host_name}
                  </option>
                ))}
            </select>
          </label>
          <label>
            {t("出口节点")}
            <select value={exit} onChange={(e) => setExit(e.target.value)}>
              <option value="">{t("直连（取消中转）")}</option>
              {nodes
                .filter(
                  (n) =>
                    n.host_id !== source?.host_id &&
                    !n.relay_exit_id &&
                    n.state === "succeeded" &&
                    n.action === "deploy",
                )
                .map((n) => (
                  <option key={n.id} value={n.id}>
                    {n.name} · {n.host_name}
                  </option>
                ))}
            </select>
          </label>
          <p>
            {t(
              "出口被线路使用时不能编辑或卸载。请先将入口改为直连或切换出口，并等待更新完成。",
            )}
          </p>
          <label>
            <input
              type="checkbox"
              checked={confirm}
              onChange={(e) => setConfirm(e.target.checked)}
            />
            {t("确认更新，连接会短暂中断")}
          </label>
          <button className="primary" disabled={busy || !confirm || !entry}>
            {t("应用线路")}
          </button>
        </form>
      )}
      {message && <p role="status">{message}</p>}
    </section>
  );
}
