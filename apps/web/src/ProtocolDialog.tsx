import RevisionEditor from "./RevisionEditor";
import { requiresTLS, isQUIC, handshakeHosts } from "./api";
import { agentVersionAtLeast } from "./agent-version";
import {
  restartNode,
  preflightNode,
  localServiceLabel,
  certificateLabel,
} from "./runtime-api";
import { t, useLocale, localeTag } from "./i18n";
import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import Select from "./Select";
import {
  createDeployment,
  deploymentConnection,
  errorMessage,
  getMachine,
  listDeployments,
  removeDeployment,
} from "./api";
import type {
  Deployment,
  DeploymentConnection,
  Host,
  MachineState,
  Protocol,
} from "./api";

const protocols = [
  {
    value: "socks",
    label: "SOCKS5",
    get description() {
      return t("用户名与密码认证 · 不加密 · TCP");
    },
  },
  {
    value: "mixed",
    label: "HTTP / SOCKS5",
    get description() {
      return t("同一端口接入 HTTP 和 SOCKS5 · 不加密 · TCP");
    },
  },
  {
    value: "hysteria",
    label: "Hysteria 1",
    description: "QUIC / UDP + TLS · 100 Mbps",
  },
  {
    value: "shadowtls",
    label: "ShadowTLS v3 + SS 2022",
    get description() {
      return t("TLS 握手伪装 + AES-256-GCM · TCP");
    },
  },
  {
    value: "snell",
    label: "Snell v4 compatible",
    get description() {
      return t("无需证书 · 客户端使用 v4 · TCP");
    },
  },
  {
    value: "snell6",
    label: "Snell v6 (beta)",
    get description() {
      return t("无需证书 · 默认流量整形 · TCP");
    },
  },
  {
    value: "anytls",
    label: "AnyTLS",
    get description() {
      return t("TCP + TLS · 密码认证");
    },
  },
  {
    value: "http",
    label: "HTTPS",
    get description() {
      return t("TCP + TLS · 密码认证");
    },
  },
  {
    value: "shadowsocks",
    label: "Shadowsocks",
    get description() {
      return t("无需域名或证书 · ChaCha20-Poly1305 · TCP");
    },
  },
  {
    value: "shadowsocks2022",
    label: "Shadowsocks 2022",
    get description() {
      return t("无需域名或证书 · 2022 AES-256-GCM · TCP");
    },
  },
  {
    value: "trojan",
    label: "Trojan",
    get description() {
      return t("TCP + TLS · 密码认证");
    },
  },
  {
    value: "vless",
    label: "VLESS",
    get description() {
      return t("TCP + TLS · UUID 认证");
    },
  },
  {
    value: "vmess",
    label: "VMess",
    get description() {
      return t("TCP + TLS · UUID 认证");
    },
  },
  {
    value: "hysteria2",
    label: "Hysteria 2",
    get description() {
      return t("QUIC / UDP + TLS · 密码认证");
    },
  },
  { value: "tuic", label: "TUIC v5", description: "QUIC / UDP + TLS" },
];
const labels: Record<Deployment["state"], string> = {
  get queued() {
    return t("等待 Agent");
  },
  get running() {
    return t("正在执行");
  },
  get succeeded() {
    return t("部署完成");
  },
  get failed() {
    return t("执行失败");
  },
  get interrupted() {
    return t("执行中断");
  },
  get cancelled() {
    return t("已取消");
  },
  get removed() {
    return t("已卸载");
  },
};
const resultLabels: Record<string, string> = {
  get updated() {
    return t("配置更新成功");
  },
  get update_rolled_back() {
    return t("更新失败，已恢复原配置");
  },

  get restarted() {
    return t("服务已重启，本机启动检查通过。仍需验证客户端连通性。");
  },
  get deployed() {
    return t("服务已安装并启动。请继续验证实际客户端连通性。");
  },
  get removed() {
    return t("此部署的服务与配置已卸载。");
  },
  get manage_required() {
    return t("需要托管模式 Agent。");
  },
  get invalid_task() {
    return t("任务格式无效，未执行。");
  },
  get unsafe_state() {
    return t("机器目录或服务状态不安全，操作已停止。");
  },
  get ownership_mismatch() {
    return t("服务归属不匹配，未修改现有服务。");
  },
  get stop_failed() {
    return t("停止服务失败，请检查机器状态。");
  },
  get remove_failed() {
    return t("卸载未完成，请核实机器状态后重试卸载。");
  },
  get reload_failed() {
    return t("系统服务配置重载失败。");
  },
  get invalid_spec() {
    return t("协议配置或 TLS 证书不符合要求。");
  },
  get port_in_use() {
    return t("监听端口已被占用，请更换端口。");
  },
  get instance_exists() {
    return t("已有同名部署实例，请先核实机器状态。");
  },
  get write_failed() {
    return t("配置写入失败，请检查磁盘和权限。");
  },
  get runtime_unavailable() {
    return t("运行时不可用，请检查下载连通性和机器架构。");
  },
  get selinux_detection_failed() { return t("无法检测 SELinux 状态，操作已停止。"); },
  get selinux_tools_missing() { return t("缺少 SELinux 策略工具，请安装发行版的 selinux-policy-devel 和 policycoreutils。"); },
  get selinux_tools_install_failed() { return t("SELinux 策略工具安装失败，请检查系统软件源后重试。"); },
  get selinux_policy_failed() { return t("SELinux 专用策略安装失败，未关闭系统安全保护。"); },
  get selinux_label_failed() { return t("SELinux 文件标签修复失败，请检查文件系统和权限。"); },
  get selinux_label_conflict() { return t("SELinux 文件标签与管理员策略冲突，未覆盖现有策略。"); },
  get selinux_domain_failed() { return t("协议服务未进入专用 SELinux 域，请检查安全策略后重试。"); },
  get config_rejected() {
    return t("运行时拒绝了此配置。");
  },
  get rollback_failed() {
    return t("服务停止未确认，已保留配置，请核实机器状态后重试卸载。");
  },
  get start_failed() {
    return t("服务启动失败，请检查机器上的服务状态。");
  },
  get journal_conflict() {
    return t("任务记录冲突，未重复执行。");
  },
  get interrupted() {
    return t("操作中断，需人工核实机器状态。");
  },
  get journal_unavailable() {
    return t("无法保存本地执行记录，操作已停止。");
  },
  get interrupted_or_expired() {
    return t("任务已中断或过期，不会自动重试。");
  },
  get authorization_revoked() {
    return t("提交者的组织管理权限已撤销。");
  },
  get revoked() {
    return t("机器接入已撤销。");
  },
};
const protocolName = (value: string) =>
  protocols.find((p) => p.value === value)?.label ?? value;

export default function ProtocolDialog({
  host,
  nodeID,
  manage,
  onClose,
  onAccess,
}: {
  host: Host;
  nodeID?: string;
  manage: boolean;
  onClose: () => void;
  onAccess: () => void;
}) {
  useLocale();
  const dialog = useRef<HTMLDialogElement>(null);
  const lifecycle = useRef<AbortController | null>(null);
  const generation = useRef(0);
  const [machine, setMachine] = useState<MachineState | null>(null);
  const [rows, setRows] = useState<Deployment[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [protocol, setProtocol] = useState<Protocol>("shadowsocks2022");
  const [port, setPort] = useState("443");
  const [serverName, setServerName] = useState("");
  const [certificate, setCertificate] = useState("");
  const [privateKey, setPrivateKey] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const [restarting, setRestarting] = useState<string | null>(null);
  const [removing, setRemoving] = useState<string | null>(null);
  const [connection, setConnection] = useState<{
    id: string;
    data: DeploymentConnection;
  } | null>(null);

  useEffect(() => {
    const d = dialog.current;
    d?.showModal();
    const controller = new AbortController();
    lifecycle.current = controller;
    let timer: number;
    async function poll() {
      const current = ++generation.current;
      try {
        const [m, r] = await Promise.all([
          getMachine(host.id, controller.signal),
          listDeployments(host.id, controller.signal),
        ]);
        if (!controller.signal.aborted && current === generation.current) {
          setMachine(m);
          setRows(r);
          setLoaded(true);
          setConnection((c) =>
            c &&
            r.some(
              (row) =>
                row.id === c.id &&
                row.state === "succeeded" &&
                row.action === "deploy",
            )
              ? c
              : null,
          );
        }
      } catch (e) {
        if (!controller.signal.aborted) {
          setConnection(null);
          setError(errorMessage(e));
        }
      } finally {
        if (!controller.signal.aborted) timer = window.setTimeout(poll, 5000);
      }
    }
    void poll();
    return () => {
      controller.abort();
      window.clearTimeout(timer);
      d?.close();
    };
  }, [host.id]);

  const agent = machine?.agent;
  const needsTLS = requiresTLS(protocol);
  const [handshakeHost, setHandshakeHost] = useState(handshakeHosts[0]);
  const selectedProtocol = creating
    ? protocol
    : rows.find((row) => row.id === nodeID)?.protocol;
  const requiredVersionFor = (kind?: string) =>
    (kind ? machine?.required_agent_versions?.[kind] : undefined) ??
    machine?.required_agent_version;
  const requiredVersion = requiredVersionFor(selectedProtocol);
  const versionCompatible = agentVersionAtLeast(
    agent?.metrics.version,
    requiredVersion,
  );
  const eligible =
    loaded &&
    host.status === "online" &&
    agent?.mode === "manage" &&
    !agent.revoked_at &&
    versionCompatible;
  const pending = rows.some(
    (r) => r.state === "queued" || r.state === "running",
  );
  const blocked = !manage
    ? t("只有组织所有者和管理员可以部署协议或查看连接凭据。")
    : !loaded
      ? t("正在读取机器状态…")
      : !agent || agent.revoked_at
        ? t("请先通过「接入 / 状态」安装并注册托管模式 Agent。")
        : agent.mode !== "manage"
          ? t(
              "当前为只读探针。请在「接入 / 状态」中明确授权并重新接入托管模式 Agent。",
            )
          : !versionCompatible
            ? requiredVersion
              ? t(
                  "当前 Agent 版本为 {0}，部署至少需要 {1}。请更新机器上的 Agent 后再部署。",
                  { 0: agent.metrics.version || "—", 1: requiredVersion },
                )
              : t("控制端未提供 Agent 版本要求，请更新控制端后重试。")
            : host.status !== "online"
              ? t("机器暂未在线，请恢复 Agent 心跳后再操作。")
              : pending
                ? t("这台机器已有任务等待完成，请稍后再创建或卸载。")
                : "";

  async function act(fn: (signal: AbortSignal) => Promise<void>) {
    const controller = lifecycle.current;
    if (!controller || controller.signal.aborted) return;
    setBusy(true);
    setError("");
    setNotice("");
    ++generation.current;
    try {
      await fn(controller.signal);
      const current = ++generation.current;
      const r = await listDeployments(host.id, controller.signal);
      if (!controller.signal.aborted && current === generation.current)
        setRows(r);
    } catch (e) {
      if (!controller.signal.aborted) setError(errorMessage(e));
    } finally {
      if (!controller.signal.aborted) setBusy(false);
    }
  }
  function submit(event: FormEvent) {
    event.preventDefault();
    if (!eligible || !manage || pending || busy || !confirmed) return;
    void act(async (signal) => {
      try {
        await createDeployment(
          host.id,
          {
            name,
            protocol,
            port: Number(port),
            server_name:
              protocol === "shadowtls"
                ? handshakeHost
                : needsTLS
                  ? serverName
                  : "",
            certificate: needsTLS ? certificate : "",
            private_key: needsTLS ? privateKey : "",
            confirm_install: confirmed,
          },
          signal,
        );
        if (!signal.aborted) {
          setCreating(false);
          setCertificate("");
          setName("");
          setNotice(t("任务已提交，Agent 将安装并启动服务。状态每 5 秒更新。"));
        }
      } finally {
        if (!signal.aborted) {
          setPrivateKey("");
          setConfirmed(false);
        }
      }
    });
  }
  return (
    <dialog
      ref={dialog}
      className="host-dialog machine-dialog protocol-dialog"
      aria-labelledby="protocol-title"
      onCancel={(e) => {
        e.preventDefault();
        if (!busy) onClose();
      }}
    >
      <div className="dialog-heading">
        <div>
          <p className="eyebrow">
            {nodeID ? "NODE DETAILS" : "PROTOCOL DEPLOYMENT"}
          </p>
          <h2 id="protocol-title">
            {creating
              ? t("新建节点")
              : nodeID
                ? t("节点详情")
                : t("{0} · 协议部署", { 0: host.name })}
          </h2>
        </div>
        <button
          className="icon-button"
          aria-label={nodeID ? t("关闭节点详情") : t("关闭协议部署")}
          disabled={busy}
          onClick={onClose}
        >
          ×
        </button>
      </div>
      <div className="machine-content">
        {!creating && (
          <p className="form-hint">
            {t(
              "由托管 Agent 安装并管理独立服务。Hysteria 1、Hysteria 2 和 TUIC 使用 QUIC / UDP；请按所选协议查看证书和客户端要求。",
            )}
          </p>
        )}
        {error && (
          <p className="form-error" role="alert">
            {t(error)}
          </p>
        )}
        {notice && (
          <p className="notice" role="status">
            {t(notice)}
          </p>
        )}
        {pending && !creating && (
          <section className="task-progress" role="status" aria-live="polite">
            <span className="task-indicator" aria-hidden="true" />
            <div>
              <strong>{t("节点任务进行中")}</strong>
              <p>
                {rows.some((r) => r.state === "running")
                  ? t("任务正在执行，状态每 5 秒更新。")
                  : t("任务已排队，等待执行。")}
              </p>
              <small>{t("可以关闭此窗口，任务会继续执行。")}</small>
            </div>
          </section>
        )}
        <section className="machine-summary">
          <strong>
            {nodeID ? `${host.name} · ${host.address}` : host.address}
          </strong>
          <p>
            {blocked ||
              (nodeID
                ? t("托管 Agent 已连接")
                : t("托管 Agent 已就绪 · 可创建协议服务"))}
          </p>
          {manage && loaded && !eligible && (
            <button
              className="secondary compact"
              disabled={busy}
              onClick={onAccess}
            >
              {t("接入 / 状态")}
            </button>
          )}
        </section>
        {!creating && (
          <>
            <div className="deployment-heading">
              <div>
                <h3>{nodeID ? t("节点状态") : t("部署记录")}</h3>
                <p className="form-hint">
                  {t("完成状态表示安装任务成功，不代表公网线路已验证。")}
                </p>
              </div>
              {!nodeID && (
                <button
                  className="primary compact"
                  disabled={busy || !eligible || !manage || pending}
                  onClick={() => {
                    setCreating(true);
                    setConnection(null);
                  }}
                >
                  {t("＋ 新建节点")}
                </button>
              )}
            </div>
            {loaded && rows.length === 0 && (
              <div className="deployment-empty">
                {t(
                  "还没有节点。选择 Shadowsocks 即可使用服务器 IP 创建，无需准备域名和证书。",
                )}
              </div>
            )}
            {rows
              .filter((row) => !nodeID || row.id === nodeID)
              .map((row) => (
                <article className="machine-job deployment-row" key={row.id}>
                  <div className="deployment-row-heading">
                    <strong>{row.name}</strong>
                    <span
                      className={`deployment-state deployment-${row.state}`}
                    >
                      {row.action === "update" ? t("更新 · ") : ""}
                      {row.action === "restart" ? t("重启 · ") : ""}
                      {row.action === "remove" && row.state !== "removed"
                        ? t("卸载 · ")
                        : ""}
                      {labels[row.state] ?? row.state}
                    </span>
                  </div>
                  <p>
                    {protocolName(row.protocol)} · {row.port}/
                    {isQUIC(row.protocol) ? "UDP" : "TCP"} ·{" "}
                    {row.server_name || t("无需域名")}
                  </p>
                  <small>
                    {new Date(row.created_at).toLocaleString(localeTag())}
                  </small>
                  <p>
                    {t("本机服务")}：{t(localServiceLabel(row))} ·{" "}
                    {t("证书有效期")}：
                    {!requiresTLS(row.protocol)
                      ? t("无需证书")
                      : certificateLabel(row.certificate_expires_at)}
                  </p>
                  <p>
                    {t("服务端版本")}：
                    {row.runtime_version
                      ? `sing-box ${row.runtime_version}`
                      : t("未上报")}
                    {!row.runtime_version && (
                      <small>
                        {" "}
                        · {t("升级 Agent 后自动上报，无需重新部署节点。")}
                      </small>
                    )}
                  </p>
                  {row.result && (
                    <p className="deployment-result">
                      {t("结果：")}
                      {resultLabels[row.result] ??
                        t("任务未完成，请检查机器状态。")}
                    </p>
                  )}
                  {row.state === "interrupted" && (
                    <p>
                      {t(
                        "操作结果不确定，请先核实机器状态；不会自动重复执行安装。",
                      )}
                    </p>
                  )}
                  {manage && (
                    <div className="deployment-actions">
                      {row.action !== "remove" &&
                        ["succeeded", "failed"].includes(row.state) && (
                          <button
                            className="secondary compact"
                            disabled={
                              busy ||
                              pending ||
                              !agentVersionAtLeast(
                                agent?.metrics.version,
                                "0.12.0-dev",
                              )
                            }
                            onClick={() => setEditing(row.id)}
                          >
                            {t("编辑 / 版本恢复")}
                          </button>
                        )}
                      {((row.state === "succeeded" &&
                        row.action === "deploy") ||
                        (row.action === "restart" &&
                          ["failed", "interrupted"].includes(row.state))) && (
                        <button
                          className="secondary compact"
                          disabled={
                            busy ||
                            !eligible ||
                            !agentVersionAtLeast(
                              agent?.metrics.version,
                              requiredVersionFor(row.protocol),
                            ) ||
                            pending
                          }
                          onClick={() => setRestarting(row.id)}
                        >
                          {t(row.service_status === "policy_required" ? "修复安全策略并重启" : "重启服务")}
                        </button>
                      )}
                      {((row.state === "succeeded" &&
                        row.action === "deploy") ||
                        (row.action === "restart" &&
                          ["failed", "interrupted"].includes(row.state))) && (
                        <button
                          className="secondary compact"
                          disabled={busy}
                          onClick={() =>
                            void act(async (signal) => {
                              const data = await deploymentConnection(
                                host.id,
                                row.id,
                                signal,
                              );
                              if (!signal.aborted)
                                setConnection({ id: row.id, data });
                            })
                          }
                        >
                          {t("连接信息")}
                        </button>
                      )}
                      {["succeeded", "failed", "interrupted"].includes(
                        row.state,
                      ) && (
                        <button
                          className="text-danger"
                          disabled={
                            busy ||
                            !eligible ||
                            !agentVersionAtLeast(
                              agent?.metrics.version,
                              requiredVersionFor(row.protocol),
                            ) ||
                            pending
                          }
                          onClick={() => {
                            setRemoving(row.id);
                            setConnection(null);
                          }}
                        >
                          {t("卸载服务")}
                        </button>
                      )}
                    </div>
                  )}
                  {editing === row.id && (
                    <RevisionEditor
                      host={host.id}
                      node={row}
                      onClose={() => setEditing(null)}
                      onSaved={() => {
                        void act(async () => {});
                      }}
                    />
                  )}
                  {restarting === row.id && (
                    <div className="delete-confirm">
                      <p>
                        {t(
                          row.service_status === "policy_required"
                            ? "将安装或修复 Xingdu 专用安全策略并重启此节点，可能需要从系统软件源安装策略工具。当前连接会短暂中断，确认继续？"
                            : "重启会短暂中断当前连接，需要托管 Agent 0.6.0。确认重启？",
                        )}
                      </p>
                      <button
                        className="primary"
                        disabled={busy || pending}
                        onClick={() =>
                          void act(async (signal) => {
                            await restartNode(host.id, row.id, signal);
                            if (!signal.aborted) {
                              setRestarting(null);
                              setNotice(
                                t("重启任务已提交，请等待 Agent 回报结果。"),
                              );
                            }
                          })
                        }
                      >
                        {t("确认重启")}
                      </button>
                      <button
                        className="secondary"
                        onClick={() => setRestarting(null)}
                      >
                        {t("取消")}
                      </button>
                    </div>
                  )}
                  {removing === row.id && (
                    <div className="delete-confirm">
                      <p>
                        {t("确认停止并卸载「")}
                        {row.name}
                        {t(
                          "」？该服务的现有连接将中断。只删除此部署的服务与配置，不修改其他服务及防火墙。",
                        )}
                      </p>
                      <button
                        className="danger"
                        disabled={
                          busy ||
                          !eligible ||
                          !agentVersionAtLeast(
                            agent?.metrics.version,
                            requiredVersionFor(row.protocol),
                          ) ||
                          pending
                        }
                        onClick={() =>
                          void act(async (signal) => {
                            await removeDeployment(host.id, row.id, signal);
                            if (!signal.aborted) {
                              setRemoving(null);
                              setNotice(
                                t("卸载任务已提交，请等待 Agent 回报结果。"),
                              );
                            }
                          })
                        }
                      >
                        {t("确认卸载")}
                      </button>
                      <button
                        className="secondary"
                        disabled={busy}
                        onClick={() => setRemoving(null)}
                      >
                        {t("取消")}
                      </button>
                    </div>
                  )}
                  {connection?.id === row.id && (
                    <div className="enrollment-result deployment-connection">
                      <div className="deployment-row-heading">
                        <strong>{t("连接信息")}</strong>
                        <button
                          className="secondary compact"
                          onClick={() => setConnection(null)}
                        >
                          {t("隐藏")}
                        </button>
                      </div>
                      <p>
                        {t(
                          "包含访问凭据，请仅分享给可信使用者。请在客户端订阅页面选择对应格式，并确认协议与证书要求。",
                        )}
                      </p>
                      <label>{t("服务器 / 端口 / TLS SNI")}</label>
                      <p>
                        {connection.data.server}:{connection.data.port}
                        <br />
                        {connection.data.server_name || t("无需域名")}
                      </p>
                      {connection.data.username && <p>{t("用户名")}：xingdu</p>}
                      {connection.data.cipher && (
                        <p>
                          {t("加密方式")}：{connection.data.cipher}
                        </p>
                      )}
                      <label htmlFor={`credential-${row.id}`}>
                        {t("认证凭据")}
                      </label>
                      <input
                        id={`credential-${row.id}`}
                        readOnly
                        value={connection.data.credential}
                        onFocus={(e) => e.target.select()}
                        autoComplete="off"
                      />
                      {connection.data.password && (
                        <>
                          <label htmlFor={`password-${row.id}`}>
                            {row.protocol === "shadowtls"
                              ? t("ShadowTLS 密码")
                              : t("TUIC 密码")}
                          </label>
                          <input
                            id={`password-${row.id}`}
                            readOnly
                            value={connection.data.password}
                            onFocus={(e) => e.target.select()}
                            autoComplete="off"
                          />
                        </>
                      )}
                      {connection.data.certificate && (
                        <>
                          <label htmlFor={`certificate-${row.id}`}>
                            {t("公开证书（用于核对信任，不含私钥）")}
                          </label>
                          <textarea
                            id={`certificate-${row.id}`}
                            readOnly
                            rows={3}
                            value={connection.data.certificate}
                            onFocus={(e) => e.target.select()}
                          />
                        </>
                      )}
                      <button
                        className="secondary compact"
                        onClick={async () => {
                          try {
                            await navigator.clipboard.writeText(
                              JSON.stringify(connection.data, null, 2),
                            );
                            if (!lifecycle.current?.signal.aborted)
                              setNotice(
                                t("连接信息已复制，使用后请清理剪贴板。"),
                              );
                          } catch {
                            setError(t("无法访问剪贴板，请选择字段手动复制。"));
                          }
                        }}
                      >
                        {t("复制连接信息")}
                      </button>
                    </div>
                  )}
                </article>
              ))}
          </>
        )}
        {creating && (
          <form
            id="protocol-install-form"
            className="deployment-form"
            onSubmit={submit}
          >
            <h3>{t("基础配置")}</h3>
            <fieldset disabled={busy}>
              <div className="deployment-fields">
                <label htmlFor="deployment-name">
                  {t("名称")}
                  <input
                    autoFocus
                    id="deployment-name"
                    required
                    maxLength={80}
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder={t("例如：东京 · Trojan")}
                  />
                </label>
                <div>
                  <label htmlFor="deployment-protocol">{t("协议")}</label>
                  <Select
                    id="deployment-protocol"
                    label={t("协议")}
                    options={protocols}
                    value={protocol}
                    disabled={busy}
                    onChange={(v) => setProtocol(v as Protocol)}
                  />
                </div>
                <label htmlFor="deployment-port">
                  {t("监听端口")}
                  <input
                    id="deployment-port"
                    required
                    type="number"
                    min={1}
                    max={65535}
                    value={port}
                    onChange={(e) => setPort(e.target.value)}
                  />
                </label>
                {needsTLS && (
                  <label htmlFor="deployment-sni">
                    {t("TLS 域名 / SNI")}
                    <input
                      id="deployment-sni"
                      required
                      maxLength={253}
                      autoComplete="off"
                      value={serverName}
                      onChange={(e) => setServerName(e.target.value)}
                      placeholder="node.example.com"
                    />
                  </label>
                )}
              </div>
              {protocol === "shadowtls" && (
                <Select
                  id="handshake-host"
                  label={t("握手域名")}
                  value={handshakeHost}
                  onChange={setHandshakeHost}
                  options={handshakeHosts.map((value) => ({
                    value,
                    label: value,
                  }))}
                />
              )}
              {needsTLS && (
                <>
                  <label htmlFor="deployment-cert">
                    {t("TLS 证书 / 完整证书链（PEM）")}
                  </label>
                  <textarea
                    id="deployment-cert"
                    required
                    rows={4}
                    maxLength={32768}
                    spellCheck={false}
                    value={certificate}
                    onChange={(e) => setCertificate(e.target.value)}
                    placeholder="-----BEGIN CERTIFICATE-----"
                  />
                  <label htmlFor="deployment-key">
                    {t("匹配的 TLS 私钥（PEM）")}
                  </label>
                  <textarea
                    id="deployment-key"
                    required
                    rows={4}
                    maxLength={16384}
                    autoComplete="off"
                    spellCheck={false}
                    value={privateKey}
                    onChange={(e) => setPrivateKey(e.target.value)}
                    placeholder="-----BEGIN PRIVATE KEY-----"
                  />
                </>
              )}
              <p className="form-hint">
                {["socks", "mixed"].includes(protocol)
                  ? t(
                      "此协议不加密传输，账号和流量可能被链路观察者读取。请仅通过可信网络或已加密隧道接入；UDP 转发已关闭。",
                    )
                  : protocol === "shadowtls"
                    ? t(
                        "使用所选公共域名完成 TLS 握手，无需上传证书。服务器必须能访问该域名的 443 端口；客户端需要 ShadowTLS v3 和 Shadowsocks 2022 支持。",
                      )
                    : protocol === "snell6"
                      ? t(
                          "Snell v6 仍在测试阶段，需要支持 v6 的客户端。仅开放 TCP 转发。",
                        )
                      : needsTLS
                        ? t(
                            "TLS 协议需提供匹配域名的有效证书与私钥；不自动申请或续签。私钥不回显，认证凭据自动生成。",
                          )
                        : t(
                            "直接使用服务器 IP，无需域名和证书。加密凭据自动生成；请放行 TCP 端口。当前暂不开放 UDP 转发。",
                          )}
              </p>
              <p className="form-hint">
                {t("请自行在云安全组和机器防火墙放行")}
                {port || t("所选端口")}/{isQUIC(protocol) ? "UDP" : "TCP"}
                {t("。星渡不会自动修改防火墙。")}
              </p>
              <label className="check-row">
                <input
                  type="checkbox"
                  checked={confirmed}
                  onChange={(e) => setConfirmed(e.target.checked)}
                />
                {t(
                  "我授权 Agent 安装运行时、创建系统服务并启动此端口的协议监听。",
                )}
              </label>
            </fieldset>
          </form>
        )}
      </div>
      <div className="dialog-footer">
        {creating ? (
          <div className="deployment-actions">
            <button
              type="button"
              className="secondary"
              disabled={busy || !eligible || pending}
              onClick={() =>
                void act(async (signal) => {
                  const result = await preflightNode(
                    host.id,
                    {
                      name,
                      protocol,
                      port: Number(port),
                      server_name:
                        protocol === "shadowtls"
                          ? handshakeHost
                          : needsTLS
                            ? serverName
                            : "",
                      certificate: needsTLS ? certificate : "",
                      private_key: needsTLS ? privateKey : "",
                    },
                    signal,
                  );
                  if (!signal.aborted)
                    setNotice(
                      (needsTLS
                        ? t("证书与配置检查通过。证书有效期：") +
                          certificateLabel(result.certificate_expires_at)
                        : t("配置检查通过，无需证书")) +
                        t(
                          "。实际端口占用在 Agent 安装时检查，公网 DNS 与防火墙仍需验证。",
                        ),
                    );
                })
              }
            >
              {t("检查配置")}
            </button>
            <button
              type="submit"
              form="protocol-install-form"
              className="primary"
              disabled={busy || !eligible || !manage || pending || !confirmed}
            >
              {busy ? t("正在提交…") : t("安装并启动")}
            </button>
            <button
              type="button"
              className="secondary"
              disabled={busy}
              onClick={() => {
                setCreating(false);
                setPrivateKey("");
                setCertificate("");
                setConfirmed(false);
              }}
            >
              {t("取消")}
            </button>
          </div>
        ) : (
          <button className="secondary" disabled={busy} onClick={onClose}>
            {t("关闭")}
          </button>
        )}
      </div>
    </dialog>
  );
}
