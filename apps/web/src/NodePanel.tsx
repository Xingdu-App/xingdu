import { createPortal } from "react-dom";
import { CardSkeleton } from "./LoadingSkeleton";
import { localServiceLabel, certificateLabel } from "./runtime-api";
import { t, useLocale, localeTag } from "./i18n";
import { useState } from "react";
import type { ManagedNode } from "./api";
import Select from "./Select";
import { requiresTLS, isQUIC, protocolNames } from "./api";

const status = (node: ManagedNode) =>
  node.action === "update"
    ? t(
        node.state === "failed"
          ? "更新失败"
          : node.state === "interrupted"
            ? "待核实"
            : "更新中",
      )
    : node.action !== "remove"
      ? node.action === "restart"
        ? t(
            node.state === "failed"
              ? "重启失败"
              : node.state === "interrupted"
                ? "待核实"
                : "重启中",
          )
        : t("已部署")
      : node.state === "queued" || node.state === "running"
        ? t("卸载中")
        : node.state === "failed"
          ? t("卸载失败")
          : t("待核实");

export default function NodePanel({
  actionTarget,
  nodes,
  ready,
  loading,
  manage,
  onCreate,
  onOpen,
}: {
  actionTarget: HTMLElement | null;
  nodes: ManagedNode[];
  ready: boolean;
  loading: boolean;
  manage: boolean;
  onCreate: () => void;
  onOpen: (node: ManagedNode) => void;
}) {
  useLocale();
  const [query, setQuery] = useState("");
  const [protocol, setProtocol] = useState("all");
  const filtered = nodes.filter(
    (node) =>
      (protocol === "all" || node.protocol === protocol) &&
      [node.name, node.host_name, node.address, node.server_name]
        .join(" ")
        .toLocaleLowerCase()
        .includes(query.trim().toLocaleLowerCase()),
  );
  return (
    <section className="panel node-panel">
      {manage &&
        actionTarget &&
        createPortal(
          <button
            className="primary compact"
            disabled={!ready}
            onClick={onCreate}
          >
            {t("＋ 新建节点")}
          </button>,
          actionTarget,
        )}
      <div className="node-toolbar">
        <label className="node-search">
          <input
            aria-label={t("搜索节点")}
            type="search"
            placeholder={t("搜索名称、服务器或地址…")}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </label>
        <Select
          label={t("筛选协议")}
          value={protocol}
          onChange={setProtocol}
          options={[
            { value: "all", label: t("全部协议") },
            ...Object.entries(protocolNames).map(([value, label]) => ({
              value,
              label,
            })),
          ]}
        />
      </div>
      {loading ? (
        <CardSkeleton />
      ) : !ready || !filtered.length ? (
        <div className="empty-state">
          <span className="pending-star" aria-hidden="true">
            ✧
          </span>
          <h3>
            {!ready
              ? loading
                ? t("正在获取节点")
                : t("节点列表暂时不可用")
              : nodes.length
                ? t("没有匹配的节点")
                : t("从第一个节点开始")}
          </h3>
          <p>
            {!ready
              ? t("请稍后刷新。")
              : nodes.length
                ? t("试试其他关键词或协议。")
                : t("选择已接入的服务器并部署协议，成功后节点会出现在这里。")}
          </p>
          {nodes.length > 0 && ready && (
            <button
              className="secondary"
              onClick={() => {
                setQuery("");
                setProtocol("all");
              }}
            >
              {t("清除筛选")}
            </button>
          )}
        </div>
      ) : (
        <div className="node-grid">
          {filtered.map((node) => (
            <article className="node-card" key={node.id}>
              <div className="node-card-top">
                <span className="node-protocol">
                  {protocolNames[node.protocol]}
                </span>
                <span
                  className={`deployment-state deployment-${node.action === "deploy" ? "succeeded" : node.state}`}
                >
                  {status(node)}
                </span>
              </div>
              <h3>{node.name}</h3>
              <p className="node-endpoint">
                {node.address.includes(":")
                  ? `[${node.address}]`
                  : node.address}
                :{node.port}{" "}
                <small>{isQUIC(node.protocol) ? "UDP / QUIC" : "TCP"}</small>
              </p>
              <dl className="node-details">
                <div>
                  <dt>{t("服务端版本")}</dt>
                  <dd>
                    {node.runtime_version
                      ? `${node.protocol === "trusttunnel" ? "TrustTunnel" : "sing-box"} ${node.runtime_version}`
                      : t("未上报")}
                  </dd>
                </div>
                <div>
                  <dt>{t("服务器")}</dt>
                  <dd>{node.host_name}</dd>
                </div>
                <div>
                  <dt>{t("TLS 域名")}</dt>
                  <dd>{node.server_name || t("无需域名")}</dd>
                </div>
                <div>
                  <dt>{t("部署时间")}</dt>
                  <dd>
                    {new Date(node.installed_at).toLocaleString(localeTag())}
                  </dd>
                </div>
                <div>
                  <dt>{t("本机服务")}</dt>
                  <dd>{t(localServiceLabel(node))}</dd>
                </div>
                <div>
                  <dt>{t("证书有效期")}</dt>
                  <dd>
                    {!requiresTLS(node.protocol)
                      ? t("无需证书")
                      : certificateLabel(node.certificate_expires_at)}
                  </dd>
                </div>
              </dl>
              <div className="node-card-footer">
                <span className="node-agent">
                  <span
                    className={`dot ${node.host_status === "online" ? "healthy" : ""}`}
                  />
                  Agent{" "}
                  {node.host_status === "online"
                    ? t("在线")
                    : node.host_status === "offline"
                      ? t("离线")
                      : t("待接入")}
                </span>
                <button
                  className="secondary compact"
                  onClick={() => onOpen(node)}
                >
                  {t("查看节点 →")}
                </button>
              </div>
            </article>
          ))}
        </div>
      )}
      <p className="node-footnote">
        {t(
          "Agent 在线仅表示机器与控制端保持连接。节点的实际连通性需通过客户端验证；卸载成功后将移出列表。",
        )}
      </p>
    </section>
  );
}
