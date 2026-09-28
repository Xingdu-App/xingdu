export type Host = {
  id: string;
  name: string;
  status: "pending" | "online" | "offline";
  last_seen_at: string | null;
};
export type System = {
  name: string;
  version: string;
  stage: string;
  database_ready: boolean;
  capabilities: {
    host_enrollment: boolean;
    deployment: boolean;
    subscription_export: boolean;
  };
};

async function get<T>(path: string, signal: AbortSignal): Promise<T> {
  const response = await fetch(path, { signal });
  if (!response.ok)
    throw new Error(`服务暂时不可用（HTTP ${response.status}）`);
  return ((await response.json()) as { data: T }).data;
}
export const loadSystem = (signal: AbortSignal) =>
  get<System>("/api/v1/system", signal);
export const loadHosts = (signal: AbortSignal) =>
  get<Host[]>("/api/v1/hosts", signal);
