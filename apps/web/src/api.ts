export type HostInput = {
  name: string;
  address: string;
  ssh_port: number;
  ssh_user: string;
  tags: string[];
  notes: string;
};
export type Host = HostInput & {
  id: string;
  status: "pending" | "online" | "offline";
  last_seen_at: string | null;
};
export type Session = { username: string; csrf_token: string };
export type System = {
  name: string;
  version: string;
  stage: string;
  database_ready: boolean;
  capabilities: {
    host_inventory: boolean;
    host_enrollment: boolean;
    deployment: boolean;
    subscription_export: boolean;
  };
};
let csrfToken = "";
export function setSessionToken(token: string) {
  csrfToken = token;
}
export class APIError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}
async function request<T>(
  path: string,
  method = "GET",
  body?: unknown,
  signal?: AbortSignal,
  quiet401 = false,
): Promise<T> {
  const timeout = AbortSignal.timeout(10000);
  const combinedSignal = signal ? AbortSignal.any([signal, timeout]) : timeout;
  const headers: Record<string, string> = {};
  if (method !== "GET") {
    headers["Content-Type"] = "application/json";
    headers["X-Xingdu-Request"] = "1";
    headers["X-CSRF-Token"] = csrfToken;
  }
  const response = await fetch(path, {
    method,
    headers,
    credentials: "same-origin",
    signal: combinedSignal,
    body: method === "GET" ? undefined : JSON.stringify(body ?? {}),
  });
  if (!response.ok) {
    const data = (await response.json().catch(() => null)) as {
      error?: { message?: string };
    } | null;
    if (response.status === 401 && !quiet401)
      window.dispatchEvent(new Event("xingdu:unauthorized"));
    throw new APIError(
      data?.error?.message ?? `服务暂时不可用（HTTP ${response.status}）`,
      response.status,
    );
  }
  if (response.status === 204) return undefined as T;
  return ((await response.json()) as { data: T }).data;
}
export const loadSystem = (signal: AbortSignal) =>
  request<System>("/api/v1/system", "GET", undefined, signal);
export const loadHosts = (signal: AbortSignal) =>
  request<Host[]>("/api/v1/hosts", "GET", undefined, signal);
export const getSession = (signal: AbortSignal) =>
  request<Session>("/api/v1/auth/session", "GET", undefined, signal, true);
export const login = (username: string, password: string) =>
  request<Session>(
    "/api/v1/auth/login",
    "POST",
    { username, password },
    undefined,
    true,
  );
export const logout = () => request<void>("/api/v1/auth/logout", "POST");
export const saveHost = (input: HostInput, id?: string) =>
  request<Host>(
    id ? `/api/v1/hosts/${id}` : "/api/v1/hosts",
    id ? "PUT" : "POST",
    input,
  );
export const deleteHost = (id: string) =>
  request<void>(`/api/v1/hosts/${id}`, "DELETE");
export function errorMessage(error: unknown) {
  if (
    error instanceof DOMException &&
    (error.name === "TimeoutError" || error.name === "AbortError")
  )
    return "请求超时，请刷新确认当前状态后再试。";
  return error instanceof Error ? error.message : "操作失败，请稍后重试。";
}
