export type OAuthProvider = "google" | "github";
export type OAuthProviders = Record<OAuthProvider, boolean>;
export const oauthProviderNames: Record<OAuthProvider, string> = {
  google: "Google",
  github: "GitHub",
};
export const oauthErrors: Record<string, string> = {
  provider_disabled: "此登录服务尚未配置，请选择其他登录方式。",
  invalid_state: "登录验证已过期或状态不匹配，请重新开始。",
  access_denied: "你已取消第三方授权，可选择其他登录方式。",
  provider_failed: "第三方登录暂时失败，请稍后重试。",
  account_exists: "此邮箱与已有账号信息冲突，请使用原登录方式或联系管理员。",
  identity_in_use: "该第三方身份已绑定其他账号，无法重复绑定。",
  registration_disabled: "此实例尚未开放新账号注册，请联系管理员。",
  link_session_expired: "绑定时的登录会话已失效，请重新登录后再试。",
  unavailable: "第三方登录暂不可用，请稍后重试。",
};

// Never display an arbitrary provider error, code, state, or token from a URL.
export function oauthFeedback(url: URL): {
  message: string;
  error: boolean;
  cleanPath: string;
  consumed: boolean;
} {
  const error = url.searchParams.get("oauth_error");
  const linked = url.searchParams.get("oauth") === "linked";
  const clean = new URL(url);
  let consumed = false;
  for (const key of [
    "oauth",
    "oauth_error",
    "code",
    "state",
    "error",
    "error_description",
    "error_uri",
    "access_token",
    "id_token",
  ]) {
    if (clean.searchParams.has(key)) consumed = true;
    clean.searchParams.delete(key);
  }
  return {
    message:
      error !== null
        ? Object.hasOwn(oauthErrors, error)
          ? oauthErrors[error]
          : oauthErrors.unavailable
        : linked
          ? "登录方式已绑定。"
          : "",
    error: error !== null,
    cleanPath: clean.pathname + clean.search + clean.hash,
    consumed,
  };
}

// Backend chooses the authorize URL; reject unexpected redirects even if a
// misconfigured reverse proxy or response unexpectedly supplies another origin.
export function oauthAuthorizationURL(
  provider: OAuthProvider,
  value: string,
): string {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new Error("第三方登录地址无效，请联系部署管理员。");
  }
  const expected =
    provider === "google"
      ? ["accounts.google.com", "/o/oauth2/v2/auth"]
      : ["github.com", "/login/oauth/authorize"];
  if (
    url.protocol !== "https:" ||
    url.hostname !== expected[0] ||
    url.pathname !== expected[1] ||
    url.port !== "" ||
    url.username ||
    url.password ||
    url.hash
  ) {
    throw new Error("第三方登录地址无效，请联系部署管理员。");
  }
  return url.href;
}

export function canUnlinkIdentity(
  provider: OAuthProvider,
  hasPassword: boolean,
  identities: { provider: OAuthProvider }[],
  providers: OAuthProviders,
): boolean {
  return (
    hasPassword ||
    identities.some(
      (identity) =>
        identity.provider !== provider && providers[identity.provider],
    )
  );
}
