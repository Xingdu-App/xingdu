import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import {
  APIError,
  errorMessage,
  getSession,
  login,
  logout,
  setSessionToken,
} from "./api";
import type { Session } from "./api";
import App from "./App";

export default function SessionGate() {
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
  if (session) return <App username={session.username} onLogout={signOut} />;
  return (
    <div className="auth-page">
      <section className="auth-card">
        <img className="auth-logo" src="/xingdu-logo.png" alt="星渡 Logo" />
        <p className="eyebrow">XINGDU · 星渡</p>
        <h1>连接，从这里开始。</h1>
        <p className="auth-subtitle">登录你的星渡控制台</p>
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
              autoComplete="current-password"
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              disabled={busy}
            />
            {error && (
              <p role="alert" className="form-error">
                {error}
              </p>
            )}
            <button className="primary auth-submit" disabled={busy}>
              {busy ? "正在登录…" : "登录控制台"}
            </button>
          </form>
        )}
        <details className="setup-help">
          <summary>首次使用？</summary>
          <p>
            请在本地终端创建管理员，再使用该账号登录。密码会隐藏输入，没有默认密码。
          </p>
          <code>docker compose exec api admin --username admin</code>
          <p>当前版本支持服务器资料管理，Agent 接入与协议部署仍在开发中。</p>
        </details>
        <div className="auth-footer">开源 · 自托管 · 自由连接</div>
      </section>
    </div>
  );
}
