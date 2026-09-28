import { useEffect, useState } from "react";
import { errorMessage, loadHosts, loadSystem } from "./api";
import HostDialog from "./HostDialog";
import type { Host, System } from "./api";
import "./App.css";

const pages = [
  { id: "overview", label: "概览", icon: "◈" },
  { id: "hosts", label: "服务器", icon: "▤" },
  { id: "routes", label: "线路", icon: "⌁" },
  { id: "subscriptions", label: "订阅", icon: "▧" },
  { id: "deployments", label: "部署记录", icon: "◷" },
  { id: "settings", label: "设置", icon: "⚙" },
] as const;

type Page = (typeof pages)[number]["id"];
type LoadState = "loading" | "ready" | "error";
const pendingCopy: Record<string, { title: string; description: string }> = {
  routes: {
    title: "从一个入口，连接更多可能",
    description: "线路编排将在后续版本开放，支持直连与单层中转。",
  },
  subscriptions: {
    title: "一次管理，在你喜欢的客户端使用",
    description:
      "Stash、Surge、Loon、Shadowrocket 的订阅输出将在兼容性验证后开放。",
  },
  deployments: {
    title: "每一次变更，都有迹可循",
    description: "部署任务、进度记录和配置回滚将在后续版本开放。",
  },
  settings: {
    title: "让星渡成为你的控制中心",
    description: "当前支持单管理员登录。团队权限、通知与凭据管理将在后续开放。",
  },
};

function App({
  username,
  onLogout,
}: {
  username: string;
  onLogout: () => Promise<void>;
}) {
  const [editing, setEditing] = useState<Host | null | undefined>(undefined);
  const [notice, setNotice] = useState("");
  const [loggingOut, setLoggingOut] = useState(false);
  const [page, setPage] = useState<Page>("overview");
  const [system, setSystem] = useState<System | null>(null);
  const [hosts, setHosts] = useState<Host[]>([]);
  const [state, setState] = useState<LoadState>("loading");
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => controller.abort(), 8000);
    let active = true;
    Promise.all([loadSystem(controller.signal), loadHosts(controller.signal)])
      .then(([systemData, hostData]) => {
        if (!active) return;
        setSystem(systemData);
        setHosts(hostData);
        setState("ready");
      })
      .catch((reason: unknown) => {
        if (!active) return;
        setSystem(null);
        setHosts([]);
        setError(
          controller.signal.aborted
            ? "连接超时，请检查本地服务是否已启动。"
            : reason instanceof Error
              ? reason.message
              : "无法连接控制端。",
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

  const refresh = () => {
    setState("loading");
    setAttempt((value) => value + 1);
  };
  const title = pages.find((item) => item.id === page)?.label;
  const connected = state === "ready" && system?.database_ready;
  const pending = pendingCopy[page];

  return (
    <div className="shell">
      <aside className="sidebar">
        <a
          className="brand"
          href="#"
          onClick={(event) => {
            event.preventDefault();
            setPage("overview");
          }}
          aria-label="星渡首页"
        >
          <img className="brand-logo" src="/xingdu-logo.png" alt="" />
          <span>
            星渡<small>XINGDU</small>
          </span>
        </a>
        <div className="workspace">
          <span className="workspace-avatar">X</span>
          <div>
            {username}
            <small>管理员</small>
          </div>
          <span className="workspace-dot" />
        </div>
        <p className="nav-label">工作台</p>
        <nav aria-label="主导航">
          {pages.map((item) => (
            <button
              key={item.id}
              className={page === item.id ? "nav-item active" : "nav-item"}
              aria-current={page === item.id ? "page" : undefined}
              onClick={() => setPage(item.id)}
            >
              <span aria-hidden="true">{item.icon}</span>
              {item.label}
              {page === item.id && <i />}
            </button>
          ))}
        </nav>
        <div className="sidebar-footer">
          <span className="little-star">✧</span>
          <p>
            连点成网
            <br />
            <strong>一键抵达。</strong>
          </p>
          <span className="version">{system?.version ?? "0.2.0-dev"}</span>
        </div>
      </aside>
      <div className="body">
        <header className="topbar">
          <div>
            工作台 <span>/</span> <strong>{title}</strong>
          </div>
          <div className="account-actions">
            <span className="environment">LOCAL / 本地环境</span>
            <button
              className="secondary"
              disabled={loggingOut}
              onClick={async () => {
                setLoggingOut(true);
                try {
                  await onLogout();
                } catch (reason) {
                  setNotice(errorMessage(reason));
                } finally {
                  setLoggingOut(false);
                }
              }}
            >
              {loggingOut ? "正在退出…" : "退出登录"}
            </button>
          </div>
        </header>
        <main>
          <div className="page-heading">
            <div>
              <p className="eyebrow">YOUR NETWORK, TOGETHER</p>
              <h1>{page === "overview" ? "一切连接，从这里开始。" : title}</h1>
              <p className="subtitle">
                {page === "overview"
                  ? "将分散的服务器，变成触手可及的网络。"
                  : "星渡 · VPS 与线路自动化管理"}
              </p>
            </div>
            <button
              className="secondary"
              disabled={state === "loading"}
              onClick={refresh}
            >
              {state === "loading" ? "正在连接…" : "刷新状态"}{" "}
              <span aria-hidden="true">↻</span>
            </button>
          </div>
          {notice && (
            <div className="notice" role="status">
              {notice}
              <button aria-label="关闭提示" onClick={() => setNotice("")}>
                ×
              </button>
            </div>
          )}
          {state === "error" && (
            <div role="alert" className="alert">
              <strong>无法获取当前状态</strong>
              <span>{error}</span>
              <button onClick={refresh}>重试</button>
            </div>
          )}
          <div className="status-line" role="status">
            <span className={`dot ${connected ? "healthy" : ""}`} />
            {state === "loading"
              ? "正在连接控制端…"
              : connected
                ? "控制端与数据库已连接"
                : "控制端或数据库不可用"}
            <span className="status-divider">/</span>
            <span>开发预览 · Agent 接入尚未开放</span>
          </div>
          {(page === "overview" || page === "hosts") && (
            <>
              {page === "overview" && (
                <div className="stats">
                  <Stat
                    label="服务器"
                    value={state === "ready" ? String(hosts.length) : "—"}
                    note="已登记的服务器资料"
                  />
                  <Stat
                    label="在线服务器"
                    value={
                      state === "ready"
                        ? String(
                            hosts.filter((host) => host.status === "online")
                              .length,
                          )
                        : "—"
                    }
                    note="以实际心跳状态为准"
                  />
                  <Stat label="线路编排" value="待开放" note="直连 / 中转" />
                  <Stat
                    label="客户端订阅"
                    value="待开放"
                    note="多客户端格式输出"
                  />
                </div>
              )}
              <section className="panel server-panel">
                <div className="section-heading">
                  <div>
                    <h2>服务器</h2>
                    <p>你的网络，从第一台服务器开始。</p>
                  </div>
                  <div className="inventory-actions">
                    <button
                      className="primary compact"
                      disabled={state !== "ready"}
                      onClick={() => setEditing(null)}
                    >
                      ＋ 添加服务器
                    </button>
                    <span className="badge">
                      {state === "ready"
                        ? `${hosts.length} 台服务器`
                        : "状态待确认"}
                    </span>
                  </div>
                </div>
                {state === "ready" && hosts.length > 0 ? (
                  <>
                    <div className="table-scroll inventory-table">
                      <table>
                        <thead>
                          <tr>
                            <th>名称</th>
                            <th>地址 / SSH</th>
                            <th>标签</th>
                            <th>状态</th>
                            <th>操作</th>
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
                                    online: "在线",
                                    offline: "离线",
                                    pending: "待接入",
                                  }[host.status]
                                }
                              </td>
                              <td>
                                <button
                                  className="secondary"
                                  aria-label={`编辑 ${host.name}`}
                                  onClick={() => setEditing(host)}
                                >
                                  编辑
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
                              className="secondary"
                              aria-label={`编辑 ${host.name}`}
                              onClick={() => setEditing(host)}
                            >
                              编辑
                            </button>
                          </div>
                          <p className="host-card-address">{host.address}</p>
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
                                  online: "在线",
                                  offline: "离线",
                                  pending: "待接入",
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
                        ? "还没有添加服务器"
                        : state === "loading"
                          ? "正在获取服务器列表"
                          : "服务器列表暂时不可用"}
                    </h3>
                    <p>
                      {state === "ready"
                        ? "添加第一台 VPS 的连接资料，为后续接入和部署做好准备。"
                        : "连接恢复后，这里会显示真实的服务器状态。"}
                    </p>
                    <button
                      className="primary"
                      disabled={state !== "ready"}
                      onClick={() => setEditing(null)}
                    >
                      ＋ 添加服务器
                    </button>
                  </div>
                )}
              </section>
              {page === "overview" && (
                <div className="bottom-grid">
                  <section className="panel journey">
                    <p className="eyebrow">A SIMPLE JOURNEY</p>
                    <h2>把复杂留给星渡。</h2>
                    <div className="steps">
                      {["接入服务器", "编排线路", "连接客户端"].map(
                        (step, index) => (
                          <div key={step}>
                            <span>0{index + 1}</span>
                            <strong>{step}</strong>
                          </div>
                        ),
                      )}
                    </div>
                    <p>部署、验证与订阅，将成为一条完整的工作流。</p>
                  </section>
                  <section className="panel clients">
                    <p className="eyebrow">BUILT TO CONNECT</p>
                    <h2>与你习惯的客户端相遇。</h2>
                    <div className="client-tags">
                      {["Stash", "Surge", "Loon", "Shadowrocket"].map(
                        (name) => (
                          <span key={name}>{name}</span>
                        ),
                      )}
                    </div>
                    <p>计划支持 · 协议与版本兼容性待验证</p>
                  </section>
                </div>
              )}
            </>
          )}
          {pending && (
            <section className="panel pending">
              <span className="pending-star" aria-hidden="true">
                ✧
              </span>
              <span className="badge">规划中</span>
              <h2>{pending.title}</h2>
              <p>{pending.description}</p>
              <button className="secondary" onClick={() => setPage("overview")}>
                返回概览
              </button>
            </section>
          )}
          <footer>
            星渡 Xingdu <span>独立部署 · 自由连接</span>
            <span>MIT LICENSE</span>
          </footer>
        </main>
      </div>
      {editing !== undefined && (
        <HostDialog
          host={editing}
          onClose={() => setEditing(undefined)}
          onSaved={() => {
            setEditing(undefined);
            setNotice("服务器资料已更新。");
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
  return (
    <section className="stat">
      <h2>{label}</h2>
      <strong className={value.length > 2 ? "text-value" : ""}>{value}</strong>
      <p>{note}</p>
    </section>
  );
}
export default App;
