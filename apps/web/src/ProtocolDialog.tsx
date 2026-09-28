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
  { value: "trojan", label: "Trojan", description: "TCP + TLS · 密码认证" },
  { value: "vless", label: "VLESS", description: "TCP + TLS · UUID 认证" },
  { value: "vmess", label: "VMess", description: "TCP + TLS · UUID 认证" },
  {
    value: "hysteria2",
    label: "Hysteria 2",
    description: "QUIC / UDP + TLS · 密码认证",
  },
  { value: "tuic", label: "TUIC v5", description: "QUIC / UDP + TLS" },
];
const labels: Record<Deployment["state"], string> = {
  queued: "等待 Agent",
  running: "正在执行",
  succeeded: "部署完成",
  failed: "执行失败",
  interrupted: "执行中断",
  cancelled: "已取消",
  removed: "已卸载",
};
const resultLabels: Record<string, string> = {
  deployed: "服务已安装并启动。请继续验证实际客户端连通性。",
  removed: "此部署的服务与配置已卸载。",
  manage_required: "需要托管模式 Agent。",
  invalid_task: "任务格式无效，未执行。",
  unsafe_state: "机器目录或服务状态不安全，操作已停止。",
  ownership_mismatch: "服务归属不匹配，未修改现有服务。",
  stop_failed: "停止服务失败，请检查机器状态。",
  remove_failed: "卸载未完成，请核实机器状态后重试卸载。",
  reload_failed: "系统服务配置重载失败。",
  invalid_spec: "协议配置或 TLS 证书不符合要求。",
  port_in_use: "监听端口已被占用，请更换端口。",
  instance_exists: "已有同名部署实例，请先核实机器状态。",
  write_failed: "配置写入失败，请检查磁盘和权限。",
  runtime_unavailable: "运行时不可用，请检查下载连通性和机器架构。",
  config_rejected: "运行时拒绝了此配置。",
  rollback_failed: "服务停止未确认，已保留配置，请核实机器状态后重试卸载。",
  start_failed: "服务启动失败，请检查机器上的服务状态。",
  journal_conflict: "任务记录冲突，未重复执行。",
  interrupted: "操作中断，需人工核实机器状态。",
  journal_unavailable: "无法保存本地执行记录，操作已停止。",
  interrupted_or_expired: "任务已中断或过期，不会自动重试。",
  authorization_revoked: "提交者的组织管理权限已撤销。",
  revoked: "机器接入已撤销。",
};
const protocolName = (value: string) =>
  protocols.find((p) => p.value === value)?.label ?? value;

export default function ProtocolDialog({
  host,
  manage,
  onClose,
  onAccess,
}: {
  host: Host;
  manage: boolean;
  onClose: () => void;
  onAccess: () => void;
}) {
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
  const [protocol, setProtocol] = useState<Protocol>("trojan");
  const [port, setPort] = useState("443");
  const [serverName, setServerName] = useState("");
  const [certificate, setCertificate] = useState("");
  const [privateKey, setPrivateKey] = useState("");
  const [confirmed, setConfirmed] = useState(false);
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
          setLoaded(false);
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
  const eligible =
    loaded &&
    host.status === "online" &&
    agent?.mode === "manage" &&
    !agent.revoked_at &&
    agent.metrics.version === "0.5.0-dev";
  const pending = rows.some(
    (r) => r.state === "queued" || r.state === "running",
  );
  const blocked = !manage
    ? "只有组织所有者和管理员可以部署协议或查看连接凭据。"
    : !loaded
      ? "正在读取机器状态…"
      : !agent || agent.revoked_at
        ? "请先通过「接入 / 状态」安装并注册托管模式 Agent。"
        : agent.mode !== "manage"
          ? "当前为只读探针。请在「接入 / 状态」中明确授权并重新接入托管模式 Agent。"
          : agent.metrics.version !== "0.5.0-dev"
            ? "需要 0.5.0-dev Agent。请按项目升级说明更新机器上的 Agent 后再部署。"
            : host.status !== "online"
              ? "机器暂未在线，请恢复 Agent 心跳后再操作。"
              : pending
                ? "这台机器已有任务等待完成，请稍后再创建或卸载。"
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
            server_name: serverName,
            certificate,
            private_key: privateKey,
            confirm_install: confirmed,
          },
          signal,
        );
        if (!signal.aborted) {
          setCreating(false);
          setCertificate("");
          setName("");
          setNotice("任务已提交，Agent 将安装并启动服务。状态每 5 秒更新。");
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
          <p className="eyebrow">PROTOCOL DEPLOYMENT</p>
          <h2 id="protocol-title">{host.name} · 协议部署</h2>
        </div>
        <button
          className="icon-button"
          aria-label="关闭协议部署"
          disabled={busy}
          onClick={onClose}
        >
          ×
        </button>
      </div>
      <div className="machine-content">
        <p className="form-hint">
          由托管 Agent 在机器上安装并管理独立服务。QUIC 是传输方式，Hysteria 2
          和 TUIC 使用 UDP。
        </p>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        {notice && (
          <p className="notice" role="status">
            {notice}
          </p>
        )}
        <section className="machine-summary">
          <strong>{host.address}</strong>
          <p>{blocked || "托管 Agent 已就绪 · 可创建协议服务"}</p>
          {manage && loaded && !eligible && (
            <button
              className="secondary compact"
              disabled={busy}
              onClick={onAccess}
            >
              接入 / 状态
            </button>
          )}
        </section>
        <div className="deployment-heading">
          <div>
            <h3>部署记录</h3>
            <p className="form-hint">
              完成状态表示安装任务成功，不代表公网线路已验证。
            </p>
          </div>
          <button
            className="primary compact"
            disabled={busy || !eligible || !manage || pending}
            onClick={() => {
              setCreating(true);
              setConnection(null);
            }}
          >
            ＋ 新建部署
          </button>
        </div>
        {loaded && rows.length === 0 && (
          <div className="deployment-empty">
            还没有协议服务。准备好域名与 TLS 证书后，即可创建第一个部署。
          </div>
        )}
        {rows.map((row) => (
          <article className="machine-job deployment-row" key={row.id}>
            <div className="deployment-row-heading">
              <strong>{row.name}</strong>
              <span className={`deployment-state deployment-${row.state}`}>
                {row.action === "remove" && row.state !== "removed"
                  ? "卸载 · "
                  : ""}
                {labels[row.state] ?? row.state}
              </span>
            </div>
            <p>
              {protocolName(row.protocol)} · {row.port}/
              {row.protocol === "hysteria2" || row.protocol === "tuic"
                ? "UDP"
                : "TCP"}{" "}
              · {row.server_name}
            </p>
            <small>{new Date(row.created_at).toLocaleString()}</small>
            {row.result && (
              <p className="deployment-result">
                结果：
                {resultLabels[row.result] ?? "任务未完成，请检查机器状态。"}
              </p>
            )}
            {row.state === "interrupted" && (
              <p>操作结果不确定，请先核实机器状态；不会自动重复执行安装。</p>
            )}
            {manage && (
              <div className="deployment-actions">
                {row.state === "succeeded" && row.action === "deploy" && (
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
                    连接信息
                  </button>
                )}
                {["succeeded", "failed", "interrupted"].includes(row.state) && (
                  <button
                    className="text-danger"
                    disabled={busy || !eligible || pending}
                    onClick={() => {
                      setRemoving(row.id);
                      setConnection(null);
                    }}
                  >
                    卸载服务
                  </button>
                )}
              </div>
            )}
            {removing === row.id && (
              <div className="delete-confirm">
                <p>
                  确认停止并卸载「{row.name}
                  」？该服务的现有连接将中断。只删除此部署的服务与配置，不修改其他服务及防火墙。
                </p>
                <button
                  className="danger"
                  disabled={busy || !eligible || pending}
                  onClick={() =>
                    void act(async (signal) => {
                      await removeDeployment(host.id, row.id, signal);
                      if (!signal.aborted) {
                        setRemoving(null);
                        setNotice("卸载任务已提交，请等待 Agent 回报结果。");
                      }
                    })
                  }
                >
                  确认卸载
                </button>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => setRemoving(null)}
                >
                  取消
                </button>
              </div>
            )}
            {connection?.id === row.id && (
              <div className="enrollment-result deployment-connection">
                <div className="deployment-row-heading">
                  <strong>连接信息</strong>
                  <button
                    className="secondary compact"
                    onClick={() => setConnection(null)}
                  >
                    隐藏
                  </button>
                </div>
                <p>
                  包含访问凭据，请仅分享给可信使用者。客户端需支持此协议，订阅导出尚未开放。
                </p>
                <label>服务器 / 端口 / TLS SNI</label>
                <p>
                  {connection.data.server}:{connection.data.port}
                  <br />
                  {connection.data.server_name}
                </p>
                <label htmlFor={`credential-${row.id}`}>认证凭据</label>
                <input
                  id={`credential-${row.id}`}
                  readOnly
                  value={connection.data.credential}
                  onFocus={(e) => e.target.select()}
                  autoComplete="off"
                />
                {connection.data.password && (
                  <>
                    <label htmlFor={`password-${row.id}`}>TUIC 密码</label>
                    <input
                      id={`password-${row.id}`}
                      readOnly
                      value={connection.data.password}
                      onFocus={(e) => e.target.select()}
                      autoComplete="off"
                    />
                  </>
                )}
                <label htmlFor={`certificate-${row.id}`}>
                  公开证书（用于核对信任，不含私钥）
                </label>
                <textarea
                  id={`certificate-${row.id}`}
                  readOnly
                  rows={3}
                  value={connection.data.certificate}
                  onFocus={(e) => e.target.select()}
                />
                <button
                  className="secondary compact"
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(
                        JSON.stringify(connection.data, null, 2),
                      );
                      if (!lifecycle.current?.signal.aborted)
                        setNotice("连接信息已复制，使用后请清理剪贴板。");
                    } catch {
                      setError("无法访问剪贴板，请选择字段手动复制。");
                    }
                  }}
                >
                  复制连接信息
                </button>
              </div>
            )}
          </article>
        ))}
        {creating && (
          <form className="deployment-form" onSubmit={submit}>
            <h3>新建协议服务</h3>
            <fieldset disabled={busy}>
              <div className="deployment-fields">
                <label htmlFor="deployment-name">
                  名称
                  <input
                    autoFocus
                  id="deployment-name"
                    required
                    maxLength={80}
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="例如：东京 · Trojan"
                  />
                </label>
                <div>
                  <label htmlFor="deployment-protocol">协议</label>
                  <Select
                    id="deployment-protocol"
                    label="协议"
                    options={protocols}
                    value={protocol}
                    disabled={busy}
                    onChange={(v) => setProtocol(v as Protocol)}
                  />
                </div>
                <label htmlFor="deployment-port">
                  监听端口
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
                <label htmlFor="deployment-sni">
                  TLS 域名 / SNI
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
              </div>
              <label htmlFor="deployment-cert">
                TLS 证书 / 完整证书链（PEM）
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
              <label htmlFor="deployment-key">匹配的 TLS 私钥（PEM）</label>
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
              <p className="form-hint">
                所有协议均启用
                TLS。提供匹配域名、在有效期内的证书与私钥；不自动申请或续签证书，不关闭客户端证书验证。私钥提交后不再回显。认证凭据由服务端随机生成。
              </p>
              <p className="form-hint">
                请自行在云安全组和机器防火墙放行 {port || "所选端口"}/
                {protocol === "hysteria2" || protocol === "tuic"
                  ? "UDP"
                  : "TCP"}
                。星渡不会自动修改防火墙。
              </p>
              <label className="check-row">
                <input
                  type="checkbox"
                  checked={confirmed}
                  onChange={(e) => setConfirmed(e.target.checked)}
                />
                我授权 Agent 安装运行时、创建系统服务并启动此端口的协议监听。
              </label>
            </fieldset>
            <div className="deployment-actions">
              <button
                className="primary"
                disabled={busy || !eligible || !manage || pending || !confirmed}
              >
                {busy ? "正在提交…" : "安装并启动"}
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
                取消
              </button>
            </div>
          </form>
        )}
      </div>
    </dialog>
  );
}
