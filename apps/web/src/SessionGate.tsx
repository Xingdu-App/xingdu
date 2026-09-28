import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import {
  APIError,
  authConfig,
  register,
  errorMessage,
  getSession,
  login,
  logout,
  setSessionToken,
} from "./api";
import type { Session } from "./api";
import OrganizationGate from "./OrganizationGate";

export default function SessionGate() {
  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  const [registering, setRegistering] = useState(false);
  const [organization, setOrganization] = useState("");
  useEffect(() => {
    authConfig()
      .then((c) => setRegistrationEnabled(c.registration_enabled))
      .catch(() => {});
  }, []);
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
      setError("登录已过期，请重新登录。");
    };
    window.addEventListener("xingdu:unauthorized", expire);
    return () => window.removeEventListener("xingdu:unauthorized", expire);
  }, []);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      if (registering) await register(username.trim(), password, organization);
      accept(await login(username.trim(), password));
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  }
  async function signOut() {
    await logout();
    setSessionToken("");
    setSession(null);
    setPassword("");
    setError("");
  }
  if (session) return <OrganizationGate session={session} onLogout={signOut} />;
  return (
    <div className="auth-page">
      <a className="auth-home-link" href="/">
        ← 返回星渡官网
      </a>
      <section className="auth-card">
        <img className="auth-logo" src="/xingdu-logo.png" alt="星渡 Logo" />
        <p className="eyebrow">XINGDU · 星渡</p>
        <h1>连接，从这里开始。</h1>
        <p className="auth-subtitle">
          {registering ? "创建账号与组织" : "登录你的星渡控制台"}
        </p>
        {window.location.hash.startsWith("#invite=") && (
          <p>登录或注册后，可接受组织邀请。</p>
        )}
        {checking ? (
          <p role="status">正在检查登录状态…</p>
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
              重新连接
            </button>
          </div>
        ) : (
          <form onSubmit={submit}>
            <label htmlFor="username">用户名</label>
            <input
              id="username"
              name="username"
              autoComplete="username"
              required
              maxLength={32}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              disabled={busy}
            />
            <label htmlFor="password">密码</label>
            <input
              id="password"
              name="password"
              type="password"
              autoComplete={registering ? "new-password" : "current-password"}
              minLength={registering ? 12 : undefined}
              maxLength={72}
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              disabled={busy}
            />
            {registering && (
              <>
                <label htmlFor="initial-org">组织名称</label>
                <input
                  id="initial-org"
                  value={organization}
                  onChange={(e) => setOrganization(e.target.value)}
                  maxLength={64}
                  required
                  disabled={busy}
                />
                <small>创建你的组织后，也可以加入受邀组织。</small>
              </>
            )}
            {error && (
              <p role="alert" className="form-error">
                {error}
              </p>
            )}
            <button className="primary auth-submit" disabled={busy}>
              {busy ? "正在处理…" : registering ? "注册并登录" : "登录控制台"}
            </button>
            {registrationEnabled && (
              <button
                className="secondary auth-submit"
                type="button"
                disabled={busy}
                onClick={() => {
                  setRegistering(!registering);
                  setError("");
                }}
              >
                {registering ? "已有账号，去登录" : "创建账号"}
              </button>
            )}
          </form>
        )}
        <details className="setup-help">
          <summary>首次使用？</summary>
          <p>
            请在本地终端创建账号，再使用该账号登录。密码会隐藏输入，没有默认密码。
          </p>
          <code>docker compose exec api admin --username admin</code>
          <p>
            组织管理员可以分享邀请链接。公开注册由部署者控制。当前支持机器接入与探针，协议部署仍在开发中。
          </p>
        </details>
        <div className="auth-footer">开源 · 自托管 · 自由连接</div>
        <nav className="auth-public-links" aria-label="公开信息">
          <a href="/pricing">价格</a>
          <a href="/privacy">隐私说明</a>
          <a href="/security">安全设计</a>
        </nav>
      </section>
    </div>
  );
}
