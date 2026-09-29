import ConsoleSkeleton from "./LoadingSkeleton";
import PasswordRecovery from "./PasswordRecovery";
import { t, useLocale } from "./i18n";
import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import {
  APIError,
  authConfig,
  register,
  verifyRegistration,
  errorMessage,
  getSession,
  login,
  logout,
  setSessionToken,
  startOAuth,
} from "./api";
import type { PendingRegistration, Session } from "./api";
import AvatarProvider from "./AvatarProvider";
import OrganizationGate from "./OrganizationGate";
import {
  oauthAuthorizationURL,
  oauthFeedback,
  oauthProviderNames,
} from "./oauth";
import type { OAuthProvider, OAuthProviders } from "./oauth";
import "./AccountForms.css";

type Verification = PendingRegistration & {
  email: string;
  expiresAt: number;
  resendAt: number;
};

export default function SessionGate() {
  useLocale();
  const [providers, setProviders] = useState<OAuthProviders>({
    google: false,
    github: false,
  });
  const [authConfigLoaded, setAuthConfigLoaded] = useState(false);
  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  const [mailConfigured, setMailConfigured] = useState(false);
  const [configError, setConfigError] = useState(false);
  const [recovering, setRecovering] = useState(false);
  const [registering, setRegistering] = useState(false);
  const [organization, setOrganization] = useState("");
  const [pending, setPending] = useState<Verification | null>(null);
  const [code, setCode] = useState("");
  const [now, setNow] = useState(Date.now);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    authConfig(controller.signal)
      .then((config) => {
        if (controller.signal.aborted) return;
        setProviders(
          config.oauth_providers ?? { google: false, github: false },
        );
        setAuthConfigLoaded(true);
        setRegistrationEnabled(config.registration_enabled);
        setMailConfigured(config.email_delivery_configured === true);
      })
      .catch(() => {
        if (!controller.signal.aborted) setConfigError(true);
      });
    return () => controller.abort();
  }, []);
  useEffect(() => {
    if (!pending) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [pending]);
  const [session, setSession] = useState<Session | null>(null);
  const [checking, setChecking] = useState(true);
  const [checkError, setCheckError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [error, setError] = useState(() =>
    window.location.pathname.replace(/\/+$/, "") === "/app/security"
      ? ""
      : t(oauthFeedback(new URL(window.location.href)).message),
  );
  const [busy, setBusy] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const accept = (value: Session) => {
    setSessionToken(value.csrf_token);
    setSession(value);
    setPassword("");
    setPending(null);
    setCode("");
    setNotice("");
    setError("");
  };
  useEffect(() => {
    const controller = new AbortController();
    getSession(controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) accept(value);
      })
      .catch((reason) => {
        if (
          !controller.signal.aborted &&
          !(reason instanceof APIError && reason.status === 401)
        )
          setCheckError(errorMessage(reason));
      })
      .finally(() => {
        if (!controller.signal.aborted) setChecking(false);
      });
    return () => controller.abort();
  }, [attempt]);
  useEffect(() => {
    const expire = () => {
      setSessionToken("");
      setSession(null);
      setPassword("");
      setError(t("登录已过期，请重新登录。"));
    };
    window.addEventListener("xingdu:unauthorized", expire);
    return () => window.removeEventListener("xingdu:unauthorized", expire);
  }, []);
  useEffect(() => {
    if (window.location.pathname.replace(/\/+$/, "") === "/app/security")
      return;
    const feedback = oauthFeedback(new URL(window.location.href));
    if (feedback.consumed)
      window.history.replaceState(window.history.state, "", feedback.cleanPath);
  }, []);
  async function socialLogin(provider: OAuthProvider) {
    if (busy || !providers[provider]) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const result = await startOAuth(provider, "login");
      const url = oauthAuthorizationURL(provider, result.authorization_url);
      setPassword("");
      window.location.assign(url);
    } catch (reason) {
      showError(reason);
      setBusy(false);
    }
  }
  function showError(reason: unknown) {
    if (reason instanceof APIError && reason.code === "email_unavailable") {
      setMailConfigured(false);
      setError(t("邮箱注册暂不可用，请稍后重试或联系服务管理员。"));
    } else if (
      reason instanceof APIError &&
      reason.code === "invalid_verification"
    ) {
      setError(t("验证码无效、已过期或尝试次数过多，请检查后重试或重新发送。"));
    } else setError(errorMessage(reason));
  }
  async function sendCode() {
    const email = username.trim().toLowerCase();
    const result = await register(email, password, organization.trim());
    // oxlint-disable-next-line react/purity -- Runs after an explicit registration request, never during render.
    const sentAt = Date.now();
    setNow(sentAt);
    setPending({
      ...result,
      email,
      expiresAt: sentAt + result.expires_in * 1000,
      resendAt: sentAt + 60_000,
    });
    setCode("");
    setNotice(
      t("若该邮箱可以注册，验证码会发送到邮箱。请检查收件箱和垃圾邮件。"),
    );
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      if (pending) {
        // oxlint-disable-next-line react/purity -- Submit event needs current wall time even when the tab timer was throttled.
        if (Date.now() >= pending.expiresAt) {
          setError(t("验证码已过期，请重新发送。"));
          return;
        }
        await verifyRegistration(pending.registration_token, code);
        const email = pending.email;
        const verifiedPassword = password;
        setPending(null);
        setCode("");
        setPassword("");
        setRegistering(false);
        setUsername(email);
        setNotice(
          t("邮箱验证成功，账号已创建。若未自动登录，请使用邮箱和密码登录。"),
        );
        accept(await login(email, verifiedPassword));
      } else if (registering) {
        if (!registrationEnabled || !mailConfigured) {
          setError(t("邮箱注册暂不可用，请稍后重试或联系服务管理员。"));
          return;
        }
        await sendCode();
      } else accept(await login(username.trim(), password));
    } catch (reason) {
      showError(reason);
    } finally {
      setBusy(false);
    }
  }
  async function resend() {
    // oxlint-disable-next-line react/purity -- Resend click handler, not a render-time clock read.
    if (busy || !pending || Date.now() < pending.resendAt || !mailConfigured)
      return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await sendCode();
    } catch (reason) {
      if (reason instanceof APIError && reason.status === 429) {
        setPending((current) =>
          current ? { ...current, resendAt: Date.now() + 60_000 } : current,
        );
      }
      showError(reason);
    } finally {
      setBusy(false);
    }
  }
  function returnToLogin() {
    setRegistering(false);
    setPending(null);
    setCode("");
    setPassword("");
    setError("");
    setNotice("");
  }
  async function signOut() {
    await logout();
    setSessionToken("");
    setSession(null);
    setPassword("");
    setPending(null);
    setCode("");
    setError("");
  }
  const expired = pending !== null && now >= pending.expiresAt;
  const resendSeconds = pending
    ? Math.max(0, Math.ceil((pending.resendAt - now) / 1000))
    : 0;
  if (session)
    return (
      <AvatarProvider key={session.id}>
        <OrganizationGate session={session} onLogout={signOut} />
      </AvatarProvider>
    );
  if (checking) return <ConsoleSkeleton />;
  if (recovering)
    return <PasswordRecovery onBack={() => setRecovering(false)} />;
  return (
    <div className="auth-page">
      <a className="auth-home-link" href="/">
        {t("← 返回星渡官网")}
      </a>
      <section className="auth-card">
        <img
          className="auth-logo"
          src="/xingdu-logo.png"
          alt={t("星渡 Logo")}
        />
        <p className="eyebrow">{t("XINGDU · 星渡")}</p>
        <h1>{t("连接，从这里开始。")}</h1>
        <p className="auth-subtitle">
          {pending
            ? t("验证邮箱，完成注册")
            : registering
              ? t("创建账号，开始管理你的服务器")
              : t("登录你的星渡控制台")}
        </p>
        {!checking && !registering && !pending && mailConfigured && (
          <button
            type="button"
            className="secondary"
            disabled={busy}
            onClick={() => {
              returnToLogin();
              setRecovering(true);
            }}
          >
            {t("忘记密码？")}
          </button>
        )}
        {window.location.hash.startsWith("#invite=") && (
          <p>{t("登录或注册后，可接受组织邀请。")}</p>
        )}
        {!checking && !checkError && !pending && authConfigLoaded && (
          <div className="social-login-options" aria-label={t("第三方登录")}>
            <p className="form-hint">{t("使用第三方账号继续")}</p>
            {(["google", "github"] as const).map((provider) => (
              <button
                className="secondary social-login-button"
                key={provider}
                type="button"
                disabled={busy || !providers[provider]}
                onClick={() => void socialLogin(provider)}
              >
                <span className="social-provider-mark" aria-hidden="true">
                  {provider === "google" ? "G" : "GH"}
                </span>
                {providers[provider]
                  ? t("使用 {0} 继续", { 0: oauthProviderNames[provider] })
                  : t("{0} 暂不可用", { 0: oauthProviderNames[provider] })}
              </button>
            ))}
            <p className="form-hint">
              {t("第三方已验证邮箱与已有账号一致时，将自动关联并登录。")}
            </p>
          </div>
        )}
        {checkError ? (
          <div className="auth-error" role="alert">
            <p>{checkError}</p>
            <button
              className="secondary"
              onClick={() => {
                setCheckError("");
                setChecking(true);
                setAttempt((value) => value + 1);
              }}
            >
              {t("重新连接")}
            </button>
          </div>
        ) : (
          <form onSubmit={submit}>
            {!pending && <p className="form-hint">{t("或使用邮箱和密码")}</p>}
            {pending ? (
              <>
                <p>{t("待验证邮箱：{0}", { 0: pending.email })}</p>
                <p className="form-hint">
                  {t(
                    "验证完成前不会创建账号。验证码有效期为 10 分钟，重发后旧码失效。刷新页面后需重新开始注册。",
                  )}
                </p>
                <label htmlFor="verification-code">{t("邮箱验证码")}</label>
                <input
                  id="verification-code"
                  name="code"
                  type="text"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]{8}"
                  minLength={8}
                  maxLength={8}
                  required
                  autoFocus
                  value={code}
                  onChange={(event) =>
                    setCode(event.target.value.replace(/\D/g, ""))
                  }
                  disabled={busy || expired}
                />
                {expired && (
                  <p className="form-error" role="status">
                    {t("验证码已过期，请重新发送。")}
                  </p>
                )}
              </>
            ) : (
              <>
                <label htmlFor="username">
                  {registering ? t("邮箱") : t("邮箱或用户名")}
                </label>
                <input
                  id="username"
                  name={registering ? "email" : "username"}
                  type={registering ? "email" : "text"}
                  autoComplete={registering ? "email" : "username"}
                  autoCapitalize="none"
                  spellCheck={false}
                  required
                  maxLength={254}
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  disabled={busy}
                />
                <label htmlFor="password">{t("密码")}</label>
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete={
                    registering ? "new-password" : "current-password"
                  }
                  minLength={registering ? 12 : undefined}
                  maxLength={72}
                  required
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  disabled={busy}
                />
                {registering && (
                  <p className="form-hint">
                    {t(
                      "密码至少 12 位，最长 72 字节；建议使用密码管理器生成并保存。",
                    )}
                  </p>
                )}
                {registering && (
                  <>
                    <label htmlFor="initial-org">{t("组织名称")}</label>
                    <input
                      id="initial-org"
                      value={organization}
                      onChange={(e) => setOrganization(e.target.value)}
                      maxLength={64}
                      required
                      disabled={busy}
                    />
                    <small>
                      {t(
                        "组织就是你的资源工作空间，个人使用也可创建，无需邀请成员。",
                      )}
                    </small>
                  </>
                )}
              </>
            )}
            {notice && (
              <p role="status" className="form-hint">
                {notice}
              </p>
            )}
            {error && (
              <p role="alert" className="form-error">
                {t(error)}
              </p>
            )}
            <button
              className="primary auth-submit"
              disabled={
                busy || expired || (!pending && registering && !mailConfigured)
              }
            >
              {busy
                ? t("正在处理…")
                : pending
                  ? t("验证并登录")
                  : registering
                    ? t("发送验证码")
                    : t("登录控制台")}
            </button>
            {pending && (
              <>
                <button
                  className="secondary auth-submit"
                  type="button"
                  disabled={busy || resendSeconds > 0 || !mailConfigured}
                  onClick={() => void resend()}
                >
                  {resendSeconds > 0
                    ? t("{0} 秒后可重发", { 0: resendSeconds })
                    : t("重新发送验证码")}
                </button>
                <button
                  className="secondary auth-submit"
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    setPending(null);
                    setCode("");
                    setNotice("");
                    setError("");
                  }}
                >
                  {t("更换邮箱")}
                </button>
                <button
                  className="secondary auth-submit"
                  type="button"
                  disabled={busy}
                  onClick={returnToLogin}
                >
                  {t("返回登录")}
                </button>
              </>
            )}
            {!pending && (registering || registrationEnabled) && (
              <button
                className="secondary auth-submit"
                type="button"
                disabled={busy || (!registering && !mailConfigured)}
                onClick={() => {
                  if (registering) returnToLogin();
                  else {
                    setRegistering(true);
                    setPassword("");
                    setError("");
                    setNotice("");
                  }
                }}
              >
                {registering ? t("已有账号，去登录") : t("创建账号")}
              </button>
            )}
            {registrationEnabled && !mailConfigured && (
              <p className="form-hint" role="status">
                {t("邮箱注册暂不可用，请稍后重试或联系服务管理员。")}
              </p>
            )}
            {configError && (
              <p className="form-hint">
                {t("暂无法获取注册配置，请稍后刷新重试；已有账号仍可登录。")}
              </p>
            )}
          </form>
        )}
        <details className="setup-help">
          <summary>{t("首次使用？")}</summary>
          <p>
            {t(
              "创建账号后，即可建立自己的工作空间来管理 VPS；也可以登录后接受他人的组织邀请。遇到注册、登录或权限问题，请查看帮助中心。",
            )}
          </p>
          <p>
            <a href="/help">{t("查看入门指引")}</a>
          </p>
        </details>
        <p className="form-hint">
          {t(
            "开始使用前，请阅读服务范围与隐私说明，了解当前预览能力和数据处理方式。",
          )}
        </p>
        <div className="auth-footer">{t("你的 VPS 与节点，一处管理")}</div>
        <nav className="auth-public-links" aria-label={t("公开信息")}>
          <a href="/help">{t("帮助中心")}</a>
          <a href="/service">{t("服务范围")}</a>
          <a href="/pricing">{t("价格")}</a>
          <a href="/privacy">{t("隐私说明")}</a>
          <a href="/security">{t("安全设计")}</a>
        </nav>
      </section>
    </div>
  );
}
