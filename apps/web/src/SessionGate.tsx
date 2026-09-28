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
} from "./api";
import type { PendingRegistration, Session } from "./api";
import AvatarProvider from "./AvatarProvider";
import OrganizationGate from "./OrganizationGate";

type Verification = PendingRegistration & {
  email: string;
  expiresAt: number;
  resendAt: number;
};

export default function SessionGate() {
  useLocale();
  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  const [mailConfigured, setMailConfigured] = useState(false);
  const [configError, setConfigError] = useState(false);
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
  const [error, setError] = useState("");
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
  function showError(reason: unknown) {
    if (reason instanceof APIError && reason.code === "email_unavailable") {
      setMailConfigured(false);
      setError(t("邮件注册暂不可用，请联系部署管理员配置邮件服务。"));
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
          setError(t("邮件注册暂不可用，请联系部署管理员配置邮件服务。"));
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
              ? t("创建账号与组织")
              : t("登录你的星渡控制台")}
        </p>
        {window.location.hash.startsWith("#invite=") && (
          <p>{t("登录或注册后，可接受组织邀请。")}</p>
        )}
        {checking ? (
          <p role="status">{t("正在检查登录状态…")}</p>
        ) : checkError ? (
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
                    <small>{t("创建你的组织后，也可以加入受邀组织。")}</small>
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
                {t("邮件注册暂不可用，请联系部署管理员配置邮件服务。")}
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
              "开放注册时，使用邮箱接收验证码后即可创建账号。自托管管理员也可以通过本地终端创建初始账号，没有默认密码。",
            )}
          </p>
          <code>docker compose exec api admin --username admin</code>
          <p>
            {t(
              "已有用户名账号仍可登录。组织管理员可以分享邀请链接，验证注册后即可接受邀请。",
            )}
          </p>
        </details>
        <div className="auth-footer">{t("开源 · 自托管 · 自由连接")}</div>
        <nav className="auth-public-links" aria-label={t("公开信息")}>
          <a href="/pricing">{t("价格")}</a>
          <a href="/privacy">{t("隐私说明")}</a>
          <a href="/security">{t("安全设计")}</a>
        </nav>
      </section>
    </div>
  );
}
