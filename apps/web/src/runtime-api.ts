import { request } from "./api";
export function restartNode(host: string, id: string, signal?: AbortSignal) {
  return request(
    `/api/v1/hosts/${host}/deployments/${id}/restart`,
    "POST",
    { confirm: true },
    signal,
  );
}
export function localServiceLabel(
  node: { service_status?: string; service_checked_at?: string | null },
  now = Date.now(),
) {
  if (
    !node.service_checked_at ||
    now - new Date(node.service_checked_at).getTime() > 90000
  )
    return "等待检查";
  return (
    (
      {
        active: "运行中",
        inactive: "未运行",
        missing: "服务缺失",
        unknown: "待核实",
      } as Record<string, string>
    )[node.service_status ?? "unknown"] ?? "待核实"
  );
}
export function certificateLabel(expires?: string | null, now = Date.now()) {
  if (!expires) return "未记录";
  const days = Math.ceil((new Date(expires).getTime() - now) / 86400000);
  return days <= 0
    ? "已过期"
    : days <= 30
      ? `${days} 天后到期`
      : new Date(expires).toLocaleDateString();
}

export function preflightNode(
  host: string,
  input: {
    name: string;
    protocol: string;
    port: number;
    server_name: string;
    certificate: string;
    private_key: string;
  },
  signal?: AbortSignal,
) {
  return request<{ certificate_expires_at: string }>(
    `/api/v1/hosts/${host}/deployments/preflight`,
    "POST",
    input,
    signal,
  );
}
