import { TableSkeleton } from "./LoadingSkeleton";
import RoutePanel from "./RoutePanel";
import APIDocsPage from "./APIDocsPage";
import APIKeysPanel from "./APIKeysPanel";
import { t, useLocale, localeTag } from "./i18n";
import type { ReactNode } from "react";
import LanguageSwitch from "./LanguageSwitch";
import TeamPanel from "./TeamPanel";
import AccountMenu from "./AccountMenu";
import AccountPage from "./AccountPage";
import BillingPage from "./BillingPage";
import type { Organization } from "./api";
import { useEffect, useState, useRef } from "react";
import { loadHosts, loadSystem, loadNodes } from "./api";
import HostDialog from "./HostDialog";
import MachineDialog from "./MachineDialog";
import ProtocolDialog from "./ProtocolDialog";
import NodePanel from "./NodePanel";
import SubscriptionPanel from "./SubscriptionPanel";
import type { Host, ManagedNode } from "./api";
import "./App.css";

import { pages, pagePath, pageFromURL } from "./routes";
import type { Page } from "./routes";
import {
  clearDetail,
  detailFromURL,
  detailURL,
  organizationURL,
  writeURL,
} from "./navigation";
const currentPage = () => pageFromURL(new URL(window.location.href));
const accountPages = new Set<string>([
  "billing",
  "profile",
  "security",
  "settings",
  "members",
]);
type LoadState = "loading" | "ready" | "error";
const pendingCopy: Record<string, { title: string; description: string }> = {};

function App({
  cloud,
  username,
  onLogout,
  organization,
  organizationControls,
  organizationBanner,
}: {
  cloud: boolean;
  organization: Organization;
  organizationControls: ReactNode;
  organizationBanner: ReactNode;
  username: string;
  onLogout: () => Promise<void>;
}) {
  const locale = useLocale();
  const canWrite = organization.role !== "viewer";
  const manageMachines =
    organization.role === "owner" || organization.role === "admin";
  const [routeURL, setRouteURL] = useState(() => new URL(window.location.href));
  const [nodes, setNodes] = useState<ManagedNode[]>([]);
  const [editing, setEditing] = useState<Host | null | undefined>(undefined);
  const [notice, setNotice] = useState("");
  const [page, updatePage] = useState<Page>(currentPage);
  const body = useRef<HTMLDivElement>(null);
  const syncRoute = () => {
    updatePage(currentPage());
    setRouteURL(new URL(window.location.href));
  };
  const navigateDetail = (detail?: {
    host?: string;
    node?: string;
    protocols?: boolean;
  }) => {
    writeURL(detailURL(new URL(window.location.href), detail));
    syncRoute();
  };
  const setProtocolHost = (host: Host) =>
    navigateDetail({ host: host.id, protocols: true });
  const setMachineHost = (host: Host) => navigateDetail({ host: host.id });
  const scopedPagePath = (next: Page) =>
    `${pagePath(next)}?organization=${encodeURIComponent(organization.id)}`;
  const setPage = (next: Page) => {
    const url = clearDetail(
      organizationURL(new URL(window.location.href), organization.id, false),
    );
    url.pathname = pagePath(next);
    url.searchParams.delete("page");
    writeURL(url);
    syncRoute();
  };
  useEffect(() => {
    const url = new URL(window.location.href);
    url.pathname = pagePath(currentPage());
    url.searchParams.delete("page");
    writeURL(url, true);
    window.addEventListener("popstate", syncRoute);
    return () => window.removeEventListener("popstate", syncRoute);
  }, []);
  useEffect(() => {
    body.current?.scrollTo({ top: 0 });
    document.title = t("{0} · 星渡 Xingdu", {
      0: pages.find((item) => item.id === page)?.label,
    });
  }, [page, locale]);
  const [hosts, setHosts] = useState<Host[]>([]);
  const detail = detailFromURL(routeURL);
  const nodeID = detail.node || undefined;
  const detailHostID = detail.node
    ? nodes.find((node) => node.id === detail.node)?.host_id
    : detail.host;
  const detailHost = hosts.find((host) => host.id === detailHostID) ?? null;
  const protocolHost = detail.protocols ? detailHost : null;
  const machineHost = detail.protocols ? null : detailHost;
  const [state, setState] = useState<LoadState>("loading");
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 8000);
    let active = true;
    Promise.all([
      loadSystem(controller.signal),
      loadHosts(controller.signal),
      loadNodes(controller.signal),
    ])
      .then(([, hostData, nodeData]) => {
        if (!active) return;
        setHosts(hostData);
        setNodes(nodeData);
        setState("ready");
      })
      .catch((reason: unknown) => {
        if (!active) return;
        setHosts([]);
        setNodes([]);
        setError(
          controller.signal.aborted
            ? t("连接超时，请检查网络后重试；持续失败时请联系服务管理员。")
            : reason instanceof Error
              ? reason.message
              : t("无法连接控制端。"),
        );
        setState("error");
      })
      .finally(() => window.clearTimeout(timeout));
    return () => {
      active = false;
      controller.abort();
      window.clearTimeout(timeout);
    };
  }, [attempt]);

  useEffect(() => {
    const timer = window.setInterval(() => setAttempt((v) => v + 1), 15000);
    return () => window.clearInterval(timer);
  }, []);
  const refresh = () => {
    setState("loading");
    setAttempt((value) => value + 1);
  };
  const title = pages.find((item) => item.id === page)?.label;
  const pending = accountPages.has(page) ? undefined : pendingCopy[page];

  return (
    <div className="shell">
      <aside className="sidebar">
        <a
          className="brand"
          href={scopedPagePath("overview")}
          onClick={(event) => {
            if (
              event.metaKey ||
              event.ctrlKey ||
              event.shiftKey ||
              event.altKey
            )
              return;
            event.preventDefault();
            setPage("overview");
          }}
          aria-label={t("星渡首页")}
        >
          <img className="brand-logo" src="/xingdu-logo.png" alt="" />
          <span>
            {t("星渡")}
            <small>XINGDU</small>
          </span>
        </a>
        {organizationControls}
        <p className="nav-label">{t("工作台")}</p>
        <nav aria-label={t("主导航")}>
          {pages
            .filter((item) => !accountPages.has(item.id))
            .map((item) => (
              <a
                href={scopedPagePath(item.id)}
                key={item.id}
                className={page === item.id ? "nav-item active" : "nav-item"}
                aria-current={page === item.id ? "page" : undefined}
                onClick={(event) => {
                  if (
                    event.metaKey ||
                    event.ctrlKey ||
                    event.shiftKey ||
                    event.altKey
                  )
                    return;
                  event.preventDefault();
                  setPage(item.id);
                }}
              >
                <span aria-hidden="true">{item.icon}</span>
                {item.label}
                {page === item.id && <i />}
              </a>
            ))}
        </nav>
        <div className="sidebar-account">
          <a className="nav-item" href="/help">
            {t("帮助中心")}
          </a>
          <LanguageSwitch />
          <AccountMenu
            cloud={cloud}
            username={username}
            organization={organization}
            onNavigate={setPage}
            onLogout={onLogout}
          />
        </div>
      </aside>
      <div className="body" ref={body}>
        <main>
          {organizationBanner}

          <div className="page-heading" hidden={accountPages.has(page)}>
            <div>
              <h1>
                {page === "overview" ? t("一切连接，从这里开始。") : title}
              </h1>
              <p className="subtitle">
                {page === "overview"
                  ? t("将分散的服务器，变成触手可及的网络。")
                  : page === "api-docs"
                    ? t("通过 API 连接你的脚本与星渡。")
                    : page === "api-keys"
                      ? t(
                          "为自动化脚本签发组织专属密钥。权限独立授权，随时撤销。",
                        )
                      : t("管理当前组织的服务器、节点与客户端配置。")}
              </p>
            </div>
            <button
              hidden={page === "api-keys" || page === "api-docs"}
              className="secondary"
              disabled={state === "loading"}
              onClick={refresh}
            >
              {state === "loading" ? t("正在连接…") : t("刷新状态")}{" "}
              <span aria-hidden="true">↻</span>
            </button>
          </div>
          {notice && (
            <div className="notice" role="status">
              {t(notice)}
              <button aria-label={t("关闭提示")} onClick={() => setNotice("")}>
                ×
              </button>
            </div>
          )}
          {state === "error" && (
            <div role="alert" className="alert">
              <strong>{t("无法获取当前状态")}</strong>
              <span>{t(error)}</span>
              <button onClick={refresh}>{t("重试")}</button>
            </div>
          )}
          {page === "api-docs" && (
            <APIDocsPage onKeys={() => setPage("api-keys")} />
          )}
          {page === "api-keys" && (
            <APIKeysPanel
              key={organization.id + organization.role}
              organization={organization}
              onDocs={() => setPage("api-docs")}
            />
          )}
          {page === "members" && <TeamPanel organization={organization} />}
          {page === "billing" && (
            <BillingPage key={organization.id} organization={organization} />
          )}
          {(page === "profile" ||
            page === "security" ||
            page === "settings") && (
            <AccountPage
              section={page}
              username={username}
              organization={organization}
              onMembers={() => setPage("members")}
            />
          )}
          {(page === "overview" || page === "hosts") && (
            <>
              {page === "overview" && (
                <div className="stats">
                  <Stat
                    label={t("服务器")}
                    value={state === "ready" ? String(hosts.length) : "—"}
                    note={t("已登记的服务器资料")}
                  />
                  <Stat
                    label={t("在线服务器")}
                    value={
                      state === "ready"
                        ? String(
                            hosts.filter((host) => host.status === "online")
                              .length,
                          )
                        : "—"
                    }
                    note={t("90 秒无心跳视为离线")}
                  />
                  <Stat
                    label={t("节点")}
                    value={state === "ready" ? String(nodes.length) : "—"}
                    note={t("已部署且尚未卸载的协议服务")}
                  />
                  <Stat
                    label={t("客户端订阅")}
                    value={t("已开放")}
                    note={t("多种客户端格式，按支持范围导出")}
                  />
                </div>
              )}
              <section className="panel server-panel">
                <div className="section-heading">
                  <div>
                    <h2>{t("服务器")}</h2>
                    <p>
                      {t("添加服务器资料后，通过「接入 / 状态」安装 Agent。")}
                    </p>
                  </div>
                  <div className="inventory-actions">
                    <button
                      className="primary compact"
                      disabled={state !== "ready" || !canWrite}
                      onClick={() => setEditing(null)}
                    >
                      {t("＋ 添加服务器")}
                    </button>
                    <span className="badge">
                      {state === "ready"
                        ? t("{0} 台服务器", { 0: hosts.length })
                        : t("状态待确认")}
                    </span>
                  </div>
                </div>
                {state === "loading" ? (
                  <TableSkeleton
                    headers={[
                      t("名称"),
                      t("地址 / SSH"),
                      t("标签"),
                      t("状态"),
                      t("Agent 版本"),
                      t("操作"),
                    ]}
                  />
                ) : state === "ready" && hosts.length > 0 ? (
                  <>
                    <div className="table-scroll inventory-table">
                      <table>
                        <thead>
                          <tr>
                            <th>{t("名称")}</th>
                            <th>{t("地址 / SSH")}</th>
                            <th>{t("标签")}</th>
                            <th>{t("状态")}</th>
                            <th>{t("Agent 版本")}</th>
                            <th>{t("操作")}</th>
                          </tr>
                        </thead>
                        <tbody>
                          {hosts.map((host) => (
                            <tr key={host.id}>
                              <td>
                                <strong>{host.name}</strong>
                                {host.notes && (
                                  <small className="host-note">
                                    {host.notes}
                                  </small>
                                )}
                              </td>
                              <td>
                                <span className="host-address">
                                  {host.address}
                                </span>
                                <small className="host-note">
                                  {host.ssh_user} · {host.ssh_port}
                                </small>
                              </td>
                              <td>
                                <div className="host-tags">
                                  {host.tags.map((tag) => (
                                    <span className="badge" key={tag}>
                                      {tag}
                                    </span>
                                  ))}
                                  {host.tags.length === 0 && "—"}
                                </div>
                              </td>
                              <td>
                                {
                                  {
                                    online: t("在线"),
                                    offline: t("离线"),
                                    pending: t("待接入"),
                                  }[host.status]
                                }
                                {host.last_seen_at && (
                                  <small className="heartbeat-time">
                                    {new Date(host.last_seen_at).toLocaleString(
                                      localeTag(),
                                    )}
                                  </small>
                                )}
                              </td>
                              <td>{host.agent_version || "—"}</td>
                              <td>
                                <button
                                  className="secondary compact"
                                  onClick={() => setMachineHost(host)}
                                >
                                  {t("接入 / 状态")}
                                </button>
                                <button
                                  className="secondary compact"
                                  onClick={() => setProtocolHost(host)}
                                >
                                  {t("协议部署")}
                                </button>
                                <button
                                  className="secondary"
                                  aria-label={t("编辑 {0}", { 0: host.name })}
                                  disabled={!manageMachines}
                                  onClick={() => setEditing(host)}
                                >
                                  {t("编辑")}
                                </button>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                    <div className="inventory-cards">
                      {hosts.map((host) => (
                        <article className="host-card" key={host.id}>
                          <div className="host-card-heading">
                            <h3>{host.name}</h3>
                            <button
                              className="secondary compact"
                              onClick={() => setMachineHost(host)}
                            >
                              {t("接入 / 状态")}
                            </button>
                            <button
                              className="secondary compact"
                              onClick={() => setProtocolHost(host)}
                            >
                              {t("协议部署")}
                            </button>
                            <button
                              className="secondary"
                              aria-label={t("编辑 {0}", { 0: host.name })}
                              disabled={!manageMachines}
                              onClick={() => setEditing(host)}
                            >
                              {t("编辑")}
                            </button>
                          </div>
                          <p className="host-card-address">{host.address}</p>
                          {host.last_seen_at && (
                            <p className="host-card-ssh">
                              {t("最近心跳：")}
                              {new Date(host.last_seen_at).toLocaleString(
                                localeTag(),
                              )}
                            </p>
                          )}
                          <p className="host-card-ssh">
                            {t("Agent 版本")} · {host.agent_version || "—"}
                          </p>
                          <p className="host-card-ssh">
                            SSH · {host.ssh_user} · {host.ssh_port}
                          </p>
                          {host.notes && (
                            <p className="host-card-notes">{host.notes}</p>
                          )}
                          <div className="host-card-footer">
                            <div className="host-tags">
                              {host.tags.map((tag) => (
                                <span className="badge" key={tag}>
                                  {tag}
                                </span>
                              ))}
                            </div>
                            <span className="badge">
                              {
                                {
                                  online: t("在线"),
                                  offline: t("离线"),
                                  pending: t("待接入"),
                                }[host.status]
                              }
                            </span>
                          </div>
                        </article>
                      ))}
                    </div>
                  </>
                ) : (
                  <div className="empty-state">
                    <div className="constellation" aria-hidden="true">
                      <span>✦</span>
                      <i />
                      <span>✧</span>
                      <i />
                      <span>✦</span>
                    </div>
                    <h3>
                      {state === "ready"
                        ? t("还没有添加服务器")
                        : t("服务器列表暂时不可用")}
                    </h3>
                    <p>
                      {state === "ready"
                        ? t(
                            "添加第一台 VPS 的连接资料，为后续接入和部署做好准备。",
                          )
                        : t("连接恢复后，这里会显示真实的服务器状态。")}
                    </p>
                    <button
                      className="primary"
                      disabled={state !== "ready" || !canWrite}
                      onClick={() => setEditing(null)}
                    >
                      {t("＋ 添加服务器")}
                    </button>
                  </div>
                )}
              </section>
              {page === "overview" && (
                <div className="bottom-grid">
                  <section className="panel journey">
                    <p className="eyebrow">A SIMPLE JOURNEY</p>
                    <h2>{t("把复杂留给星渡。")}</h2>
                    <div className="steps">
                      {[t("接入服务器"), t("部署协议"), t("连接客户端")].map(
                        (step, index) => (
                          <div key={step}>
                            <span>0{index + 1}</span>
                            <strong>{step}</strong>
                          </div>
                        ),
                      )}
                    </div>
                    <p>
                      {t(
                        "接入托管 Agent，部署节点后创建订阅，在兼容的客户端中导入使用。",
                      )}
                    </p>
                  </section>
                  <section className="panel clients">
                    <p className="eyebrow">BUILT TO CONNECT</p>
                    <h2>{t("与你习惯的客户端相遇。")}</h2>
                    <div className="client-tags">
                      {["Stash", "Mihomo", "Surge", "Loon"].map((name) => (
                        <span key={name}>{name}</span>
                      ))}
                    </div>
                    <p>{t("已提供格式导出 · 实际 App 导入与联网需验证")}</p>
                  </section>
                </div>
              )}
            </>
          )}
          {page === "subscriptions" && (
            <SubscriptionPanel
              key={organization.id + organization.role}
              nodes={nodes}
              manage={manageMachines}
              refreshKey={attempt}
              onNodes={() => setPage("nodes")}
            />
          )}
          {page === "routes" && (
            <RoutePanel
              key={organization.id + organization.role}
              nodes={nodes}
              manage={manageMachines}
              onChanged={refresh}
            />
          )}
          {page === "nodes" && (
            <NodePanel
              nodes={nodes}
              ready={state === "ready"}
              loading={state === "loading"}
              manage={manageMachines}
              onCreate={() => setPage("deployments")}
              onOpen={(node) => {
                const host = hosts.find((h) => h.id === node.host_id);
                if (host) {
                  navigateDetail({ node: node.id });
                }
              }}
            />
          )}
          {page === "deployments" && (
            <section className="panel deployment-panel">
              <div className="section-heading">
                <div>
                  <h2>{t("按服务器管理协议")}</h2>
                  <p>{t("查看部署记录、安装协议服务或卸载已有服务。")}</p>
                </div>
                <span className="badge">{t("托管 Agent")}</span>
              </div>
              <div className="deployment-hosts">
                {hosts.map((host) => (
                  <button
                    key={host.id}
                    className="deployment-host"
                    onClick={() => setProtocolHost(host)}
                  >
                    <span>
                      <strong>{host.name}</strong>
                      <small>{host.address}</small>
                    </span>
                    <span>
                      {host.status === "online" ? t("在线") : t("待连接")} →
                    </span>
                  </button>
                ))}
                {state === "ready" && !hosts.length && (
                  <p className="form-hint">
                    {t("先在服务器页面添加机器并接入 Agent。")}
                  </p>
                )}
              </div>
            </section>
          )}
          {pending && (
            <section className="panel pending">
              <span className="pending-star" aria-hidden="true">
                ✧
              </span>
              <span className="badge">{t("规划中")}</span>
              <h2>{pending.title}</h2>
              <p>{pending.description}</p>
              <button className="secondary" onClick={() => setPage("overview")}>
                {t("返回概览")}
              </button>
            </section>
          )}
        </main>
      </div>
      {state === "ready" && (detail.node || detail.host) && !detailHost && (
        <div className="team-banner" role="alert">
          {t("该服务器或节点不存在，或不属于当前组织。")}
          <button className="secondary" onClick={() => navigateDetail()}>
            {t("关闭详情")}
          </button>
        </div>
      )}
      {protocolHost && (
        <ProtocolDialog
          key={`${organization.id}:${protocolHost.id}:${nodeID ?? "all"}`}
          nodeID={nodeID}
          host={hosts.find((h) => h.id === protocolHost.id) ?? protocolHost}
          manage={manageMachines}
          onClose={() => {
            navigateDetail();
            refresh();
          }}
          onAccess={() => {
            setMachineHost(protocolHost);
          }}
        />
      )}
      {machineHost && (
        <MachineDialog
          host={machineHost}
          manage={manageMachines}
          onClose={() => navigateDetail()}
          onChanged={refresh}
        />
      )}
      {editing !== undefined && (
        <HostDialog
          host={editing}
          onClose={() => setEditing(undefined)}
          onSaved={() => {
            setEditing(undefined);
            setNotice(t("服务器资料已更新。"));
            refresh();
          }}
        />
      )}
    </div>
  );
}
function Stat({
  label,
  value,
  note,
}: {
  label: string;
  value: string;
  note: string;
}) {
  useLocale();
  return (
    <section className="stat">
      <h2>{label}</h2>
      <strong className={value.length > 2 ? "text-value" : ""}>{value}</strong>
      <p>{note}</p>
    </section>
  );
}
export default App;
