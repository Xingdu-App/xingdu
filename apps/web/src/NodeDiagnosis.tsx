import { useEffect, useRef, useState } from "react";
import { request, type ManagedNode } from "./api";
import { localServiceLabel } from "./runtime-api";
import { t, localeTag } from "./i18n";

type Step = {
  status: "passed" | "failed" | "unknown" | "not_applicable";
  code: string;
  latency_ms?: number;
  exit_ip?: string;
};
type Report = {
  revision: number;
  checked_at: string;
  agent_status: string;
  service_status: string | null;
  service_checked_at: string | null;
  network: {
    dns: Step;
    addresses: {
      address: string;
      family: string;
      tcp: Step;
      tls: Step;
      forwarding: Step;
    }[];
  };
};
const reasons: Record<string, string> = {
  resolved: "地址解析成功",
  dns_failed: "地址解析失败，请检查域名的 A / AAAA 记录",
  target_blocked: "目标包含非公开地址，无法从控制端诊断",
  connected: "端口连接成功",
  tcp_failed: "端口连接失败，请检查服务监听、安全组和防火墙",
  probe_network_unavailable:
    "控制端没有到此地址的可用路由，请从支持该网络的客户端验证",
  certificate_verified: "证书及 TLS 域名验证通过",
  tls_failed: "TLS 验证失败，请检查证书有效期、域名和服务配置",
  tcp_required: "端口不可连接，后续检查未执行",
  tls_required: "TLS 未通过，转发检查未执行",
  udp_transport: "此协议使用 UDP，TCP / 常规 TLS 检查不适用",
  no_tls: "此配置不使用 TLS",
  reality_handshake: "REALITY 需由兼容客户端验证",
  runtime_unavailable: "控制端未提供可验证的官方客户端运行时，未测试协议转发",
  client_unsupported: "诊断客户端暂不支持此协议或配置，请使用兼容客户端验证",
  https_forwarded: "通过节点成功访问 HTTPS 并获得出口地址",
  protocol_or_egress_failed:
    "协议转发失败，可能涉及认证、传输配置或出口网络，请结合端口和 TLS 结果排查",
  agent_online: "最近的 Agent 心跳在线，仅表示管理连接正常",
  agent_offline: "未收到近期在线心跳，请检查 Agent 和服务器网络",
  not_checked: "未检查",
};
function Result({ label, step }: { label: string; step: Step }) {
  const labels = {
    passed: "通过",
    failed: "失败",
    unknown: "未验证",
    not_applicable: "不适用",
  };
  return (
    <div className="diagnosis-step">
      <div>
        <strong>{t(label)}</strong>
        <span className={`diagnosis-status diagnosis-${step.status}`}>
          {t(labels[step.status])}
        </span>
      </div>
      <p>
        {t(reasons[step.code] ?? "未检查")}
        {step.latency_ms !== undefined && <> · {step.latency_ms} ms</>}
        {step.exit_ip && (
          <>
            {" "}
            · {t("出口地址")}：{step.exit_ip}
          </>
        )}
      </p>
    </div>
  );
}
export default function NodeDiagnosis({
  node,
  onClose,
}: {
  node: ManagedNode;
  onClose: () => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const controller = useRef<AbortController | null>(null);
  const [report, setReport] = useState<Report | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const run = () => {
    controller.current?.abort();
    const abort = new AbortController();
    controller.current = abort;
    setBusy(true);
    setReport(null);
    setError("");
    request<Report>(
      `/api/v1/hosts/${node.host_id}/deployments/${node.id}/diagnosis`,
      "POST",
      {},
      abort.signal,
    )
      .then((value) => {
        if (!abort.signal.aborted) setReport(value);
      })
      .catch((err: unknown) => {
        if (!abort.signal.aborted)
          setError(
            err instanceof Error ? err.message : t("诊断失败，请稍后重试"),
          );
      })
      .finally(() => {
        if (!abort.signal.aborted) setBusy(false);
      });
  };
  useEffect(() => {
    dialog.current?.showModal();
    return () => controller.current?.abort();
  }, []);
  return (
    <dialog
      ref={dialog}
      className="host-dialog machine-dialog node-diagnosis-dialog"
      onCancel={onClose}
      onClose={onClose}
      aria-labelledby="node-diagnosis-title"
    >
      <div className="dialog-heading">
        <div>
          <h2 id="node-diagnosis-title">{t("节点诊断")}</h2>
          <p>{node.name}</p>
        </div>
        <button
          className="icon-button"
          aria-label={t("关闭")}
          onClick={onClose}
        >
          ×
        </button>
      </div>
      <p className="muted">
        {t(
          "从控制端检查节点，结果取决于控制端网络。IPv4、IPv6 分别测试；不代表本地客户端线路或 UDP 转发已通过验收。",
        )}
      </p>
      {!report && !busy && !error && (
        <p>
          {t(
            "点击开始诊断，检查 Agent、服务、地址解析、端口、TLS 和协议转发。",
          )}
        </p>
      )}
      {busy && <p role="status">{t("正在诊断，最多约 25 秒…")}</p>}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {report && (
        <div aria-live="polite">
          <p className="muted">
            {t("配置版本")} #{report.revision} ·{" "}
            {new Date(report.checked_at).toLocaleString(localeTag())}
          </p>
          <Result
            label="Agent"
            step={{
              status: report.agent_status === "online" ? "passed" : "unknown",
              code:
                report.agent_status === "online"
                  ? "agent_online"
                  : "agent_offline",
            }}
          />
          <p>
            {t("本机服务")}：
            {t(
              localServiceLabel({
                service_status: report.service_status ?? undefined,
                service_checked_at: report.service_checked_at,
              }),
            )}
            {report.service_checked_at && (
              <>
                {" "}
                ·{" "}
                {new Date(report.service_checked_at).toLocaleString(
                  localeTag(),
                )}
              </>
            )}
          </p>
          <Result label="DNS / 地址" step={report.network.dns} />
          {report.network.addresses.map((address) => (
            <section className="diagnosis-address" key={address.address}>
              <h3>
                {address.family} · {address.address}
              </h3>
              <Result label="TCP 端口" step={address.tcp} />
              <Result label="TLS" step={address.tls} />
              <Result label="协议认证与 HTTPS 转发" step={address.forwarding} />
            </section>
          ))}
        </div>
      )}
      <div className="dialog-actions">
        <button className="secondary" onClick={onClose}>
          {t("关闭")}
        </button>
        <button className="primary" disabled={busy} onClick={run}>
          {t(report || error ? "重新诊断" : "开始诊断")}
        </button>
      </div>
    </dialog>
  );
}
