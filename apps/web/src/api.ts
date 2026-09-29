import { t } from "./i18n";
import type { OAuthProvider, OAuthProviders } from "./oauth";
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
  agent_version: string | null;
};
export type Session = { id: string; username: string; csrf_token: string };
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
  code?: string;
  constructor(message: string, status: number, code?: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}
export async function request<T>(
  path: string,
  method = "GET",
  body?: unknown,
  signal?: AbortSignal,
  quiet401 = false,
): Promise<T> {
  const timeout = AbortSignal.timeout(
    path.startsWith("/api/v1/billing") ? 30000 : 10000,
  );
  const combinedSignal = signal ? AbortSignal.any([signal, timeout]) : timeout;
  const headers: Record<string, string> = {};
  if (organizationID) headers["X-Xingdu-Organization"] = organizationID;
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
      error?: { message?: string; code?: string };
    } | null;
    if (response.status === 401 && !quiet401)
      window.dispatchEvent(new Event("xingdu:unauthorized"));
    throw new APIError(
      data?.error?.message ??
        t("服务暂时不可用（HTTP {0}）", { 0: response.status }),
      response.status,
      data?.error?.code,
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
    return t("请求超时，请刷新确认当前状态后再试。");
  return error instanceof Error ? error.message : t("操作失败，请稍后重试。");
}

export type Role = "owner" | "admin" | "member" | "viewer";
export type Organization = { id: string; name: string; role: Role };
export type Member = { id: string; username: string; role: Role };
export type Invitation = {
  id: string;
  role: Role;
  expires_at: string;
  accepted_at: string | null;
  revoked_at: string | null;
};
export const roleNames: Record<Role, string> = {
  get owner() {
    return t("所有者");
  },
  get admin() {
    return t("管理员");
  },
  get member() {
    return t("成员");
  },
  get viewer() {
    return t("只读成员");
  },
};
let organizationID = "";
export const setOrganization = (id: string) => {
  organizationID = id;
};
export type AuthConfig = {
  mode: "cloud" | "self_hosted";
  multi_organization: boolean;
  registration_enabled: boolean;
  email_verification_required: boolean;
  email_delivery_configured: boolean;
  oauth_providers: OAuthProviders;
};
export const authConfig = (signal?: AbortSignal) =>
  request<AuthConfig>("/api/v1/auth/config", "GET", undefined, signal, true);
export type PendingRegistration = {
  registration_token: string;
  expires_in: number;
};
export const register = (
  email: string,
  password: string,
  organization: string,
) =>
  request<PendingRegistration>(
    "/api/v1/auth/register",
    "POST",
    { email, password, organization },
    undefined,
    true,
  );
export const verifyRegistration = (registration_token: string, code: string) =>
  request<{ registered: boolean }>(
    "/api/v1/auth/register/verify",
    "POST",
    { registration_token, code },
    undefined,
    true,
  );
export const startOAuth = (provider: OAuthProvider, mode: "login" | "link") =>
  request<{ authorization_url: string }>(
    `/api/v1/auth/oauth/${provider}/start`,
    "POST",
    { mode },
    undefined,
    mode === "login",
  );
export const listOrganizations = () =>
  request<Organization[]>("/api/v1/organizations");
export const createOrganization = (name: string) =>
  request<Organization>("/api/v1/organizations", "POST", { name });
export const listMembers = () => request<Member[]>("/api/v1/members");
export const changeMember = (id: string, role: Role) =>
  request<void>(`/api/v1/members/${id}`, "PUT", { role });
export const removeMember = (id: string) =>
  request<void>(`/api/v1/members/${id}`, "DELETE");
export const listInvitations = () =>
  request<Invitation[]>("/api/v1/invitations");
export const createInvitation = (role: Role, email = "") =>
  request<{ invitation: Invitation; url: string; email_delivery: string }>(
    "/api/v1/invitations",
    "POST",
    { role, email },
  );
export const revokeInvitation = (id: string) =>
  request<void>(`/api/v1/invitations/${id}`, "DELETE");
export const acceptInvitation = (token: string) =>
  request<{ organization_id: string }>("/api/v1/invitations/accept", "POST", {
    token,
  });

export type MachineMetrics = {
  hostname: string;
  os: string;
  arch: string;
  version: string;
  uptime_seconds: number;
  memory_total_bytes: number;
  memory_available_bytes: number;
  load_1: number;
  cpus: number;
};
export type MachineState = {
  latest_agent_version?: string;
  required_agent_version: string;
  required_agent_versions?: Record<string, string>;
  agent: null | {
    self_update?: boolean;
    mode: "monitor" | "manage";
    enrolled_at: string;
    revoked_at: string | null;
    metrics: MachineMetrics;
  };
  credential: null | { method: string; fingerprint: string; saved_at: string };
  jobs: {
    action: "install" | "upgrade";
    target_version?: string;
    id: string;
    host_id: string;
    mode: string;
    state: string;
    result: string;
    created_at: string;
    finished_at: string | null;
  }[];
};
export type Enrollment = {
  token: string;
  expires_at: string;
  origin: string;
  command: string;
};
export const getMachine = (id: string, signal?: AbortSignal) =>
  request<MachineState>(
    `/api/v1/hosts/${id}/machine`,
    "GET",
    undefined,
    signal,
  );
export const issueEnrollment = (
  id: string,
  mode: string,
  confirm_manage: boolean,
) =>
  request<Enrollment>(`/api/v1/hosts/${id}/enrollment`, "POST", {
    mode,
    confirm_manage,
  });
export const revokeMachine = (id: string) =>
  request<void>(`/api/v1/hosts/${id}/agent`, "DELETE");
export const forgetCredential = (id: string) =>
  request<void>(`/api/v1/hosts/${id}/credential`, "DELETE");
export const inspectSSH = (id: string) =>
  request<{ fingerprint: string }>(
    `/api/v1/hosts/${id}/ssh/fingerprint`,
    "POST",
  );
export const installSSH = (
  id: string,
  input: {
    method: string;
    password: string;
    private_key: string;
    passphrase: string;
    fingerprint: string;
    mode: string;
    retain: boolean;
    use_saved: boolean;
    confirm_manage: boolean;
    confirm_fingerprint: boolean;
  },
) =>
  request<{ id: string; state: string }>(
    `/api/v1/hosts/${id}/ssh/install`,
    "POST",
    input,
  );

export const upgradeAgent = (id: string) =>
  request<{ id: string; state: string }>(
    `/api/v1/hosts/${id}/agent/upgrade`,
    "POST",
    { confirm_upgrade: true },
  );

export const upgradeSSH = (id: string, input: Record<string, unknown>) =>
  request<{ id: string; state: string }>(
    `/api/v1/hosts/${id}/ssh/upgrade`,
    "POST",
    { ...input, confirm_upgrade: true },
  );

export type Protocol =
  | "trojan"
  | "vless"
  | "vmess"
  | "hysteria2"
  | "tuic"
  | "shadowsocks"
  | "shadowsocks2022"
  | "anytls"
  | "http";
export const isShadowsocks = (p: string) =>
  p === "shadowsocks" || p === "shadowsocks2022";
export type Deployment = {
  runtime_version?: string;
  id: string;
  host_id: string;
  name: string;
  protocol: Protocol;
  port: number;
  server_name: string;
  state:
    | "queued"
    | "running"
    | "succeeded"
    | "failed"
    | "interrupted"
    | "cancelled"
    | "removed";
  action: "deploy" | "remove" | "restart" | "update";
  probe_ok?: boolean | null;
  probe_at?: string | null;
  probe_latency_ms?: number | null;
  probe_exit_ip?: string | null;
  relay_exit_id?: string;
  revision?: number;
  pending_revision?: number | null;
  certificate_expires_at?: string | null;
  service_status?: string;
  service_checked_at?: string | null;
  result: string;
  created_at: string;
  finished_at: string | null;
};
export type DeploymentConnection = {
  cipher?: string;
  protocol: Protocol;
  server: string;
  port: number;
  server_name: string;
  credential: string;
  password?: string;
  certificate: string;
};
const deploymentPath = (host: string, id?: string) =>
  `/api/v1/hosts/${host}/deployments${id ? `/${id}` : ""}`;
export const listDeployments = (host: string, signal?: AbortSignal) =>
  request<Deployment[]>(deploymentPath(host), "GET", undefined, signal);
export const createDeployment = (
  host: string,
  input: {
    name: string;
    protocol: Protocol;
    port: number;
    server_name: string;
    certificate: string;
    private_key: string;
    confirm_install: boolean;
  },
  signal?: AbortSignal,
) =>
  request<{ id: string; state: "queued" }>(
    deploymentPath(host),
    "POST",
    input,
    signal,
  );
export const removeDeployment = (
  host: string,
  id: string,
  signal?: AbortSignal,
) =>
  request<{ id: string; state: "queued" }>(
    deploymentPath(host, id),
    "DELETE",
    undefined,
    signal,
  );
export const deploymentConnection = (
  host: string,
  id: string,
  signal?: AbortSignal,
) =>
  request<DeploymentConnection>(
    `${deploymentPath(host, id)}/connection`,
    "POST",
    undefined,
    signal,
  );

export type ManagedNode = Deployment & {
  host_name: string;
  address: string;
  host_status: Host["status"];
  installed_at: string;
};
export const loadNodes = (signal: AbortSignal) =>
  request<ManagedNode[]>("/api/v1/nodes", "GET", undefined, signal);

export type SubscriptionRule = {
  type: "domain" | "domain_suffix" | "ip_cidr";
  value: string;
  target: "proxy" | "direct" | "reject" | `group:${string}`;
};
export type RoutingGroup = {
  id: string;
  name: string;
  type: "select" | "url-test" | "fallback";
  node_ids: string[];
};
export type SubscriptionRouting = {
  preset: string;
  groups: RoutingGroup[];
  targets: Record<string, string>;
  final: string;
};
export type RoutingPreset = {
  id: string;
  recommended?: boolean;
  name: string;
  name_en: string;
  description: string;
  description_en: string;
  reference: string;
  groups: (Omit<RoutingGroup, "node_ids"> & { name_en: string })[];
  bindings: { source: string; target: string }[];
  final: string;
};
export type RoutingCatalog = {
  sources: { id: string; name: string; name_en: string; url: string }[];
  presets: RoutingPreset[];
};
export const loadRoutingCatalog = (signal: AbortSignal) =>
  request<RoutingCatalog>(
    "/api/v1/subscription-presets",
    "GET",
    undefined,
    signal,
  );
export type SubscriptionInput = {
  format: "stash" | "mihomo" | "surge" | "loon" | "hysteria2_uri";
  name: string;
  node_ids: string[];
  rules: SubscriptionRule[];
  final_action: "proxy" | "direct";
  enabled: boolean;
  routing?: SubscriptionRouting | null;
};
export type Subscription = SubscriptionInput & {
  subscription_path?: string;
  link_state?: "available" | "legacy" | "unavailable";
  id: string;
  created_at: string;
  updated_at: string;
};
export const listSubscriptions = (signal: AbortSignal) =>
  request<Subscription[]>("/api/v1/subscriptions", "GET", undefined, signal);
export const createSubscription = (
  input: SubscriptionInput,
  signal: AbortSignal,
) =>
  request<{ subscription: Subscription; subscription_path: string }>(
    "/api/v1/subscriptions",
    "POST",
    input,
    signal,
  );
export const updateSubscription = (
  id: string,
  input: SubscriptionInput,
  signal: AbortSignal,
) => request<Subscription>(`/api/v1/subscriptions/${id}`, "PUT", input, signal);
export const deleteSubscription = (id: string, signal: AbortSignal) =>
  request<void>(`/api/v1/subscriptions/${id}`, "DELETE", undefined, signal);
export const rotateSubscription = (id: string, signal: AbortSignal) =>
  request<{ subscription_path: string }>(
    `/api/v1/subscriptions/${id}/rotate`,
    "POST",
    undefined,
    signal,
  );

export const protocolNames: Record<Protocol, string> = {
  shadowsocks: "Shadowsocks",
  shadowsocks2022: "Shadowsocks 2022",
  anytls: "AnyTLS",
  http: "HTTPS",
  trojan: "Trojan",
  vless: "VLESS",
  vmess: "VMess",
  hysteria2: "Hysteria 2",
  tuic: "TUIC v5",
};

export const getAvatar = (signal: AbortSignal) =>
  request<{ image: string }>(
    "/api/v1/account/avatar",
    "GET",
    undefined,
    signal,
  );
export const saveAvatar = (image: string, signal: AbortSignal) =>
  request<{ image: string }>(
    "/api/v1/account/avatar",
    "PUT",
    { image },
    signal,
  );
export const removeAvatar = (signal: AbortSignal) =>
  request<void>("/api/v1/account/avatar", "DELETE", undefined, signal);

export type RuleTemplateInput = Pick<SubscriptionInput, "name" | "rules" | "final_action">;
export type RuleTemplate = RuleTemplateInput & { id: string; created_at: string; updated_at: string };
export const listRuleTemplates = (signal: AbortSignal) => request<RuleTemplate[]>("/api/v1/rule-templates", "GET", undefined, signal);
export const saveRuleTemplate = (id: string | undefined, input: RuleTemplateInput, signal: AbortSignal) => request<RuleTemplate>(id ? `/api/v1/rule-templates/${id}` : "/api/v1/rule-templates", id ? "PUT" : "POST", input, signal);
export const deleteRuleTemplate = (id: string, signal: AbortSignal) => request<void>(`/api/v1/rule-templates/${id}`, "DELETE", undefined, signal);
