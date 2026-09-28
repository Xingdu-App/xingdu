import { request } from "./api";
import type { OAuthProvider } from "./oauth";
export type AccountSession = {
  id: string;
  created_at: string;
  expires_at: string;
  current: boolean;
};
export const getProfile = (signal?: AbortSignal) =>
  request<{ display_name: string }>(
    "/api/v1/account/profile",
    "GET",
    undefined,
    signal,
  );
export const saveProfile = (display_name: string) =>
  request<void>("/api/v1/account/profile", "PUT", { display_name });
export const getSessions = (signal?: AbortSignal) =>
  request<AccountSession[]>(
    "/api/v1/account/sessions",
    "GET",
    undefined,
    signal,
  );
export const revokeSession = (id: string) =>
  request<void>(`/api/v1/account/sessions/${encodeURIComponent(id)}`, "DELETE");
export const changePassword = (
  current_password: string,
  new_password: string,
) =>
  request<void>("/api/v1/account/password", "POST", {
    current_password,
    new_password,
  });

export type LoginIdentities = {
  has_password: boolean;
  identities: { provider: OAuthProvider; email: string; created_at: string }[];
};
export const getLoginIdentities = (signal?: AbortSignal) =>
  request<LoginIdentities>(
    "/api/v1/account/identities",
    "GET",
    undefined,
    signal,
  );
export const unlinkLoginIdentity = (provider: OAuthProvider) =>
  request<void>(`/api/v1/account/identities/${provider}`, "DELETE");
