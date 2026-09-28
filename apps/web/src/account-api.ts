import { request } from "./api";
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
