import { useState } from "react";
import { request, errorMessage } from "./api";
import { useLocale } from "./i18n";

export default function PasswordRecovery({ onBack }: { onBack: () => void }) {
  const locale = useLocale();
  const text = (zh: string, en: string) => (locale === "en" ? en : zh);
  const [email, setEmail] = useState("");
  const [token, setToken] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    if (
      token &&
      (password !== confirmation ||
        new TextEncoder().encode(password).length < 12 ||
        new TextEncoder().encode(password).length > 72)
    ) {
      setError(
        text(
          "两次密码须一致，长度为 12–72 字节。",
          "Passwords must match and contain 12–72 bytes.",
        ),
      );
      return;
    }
    setBusy(true);
    try {
      if (!token) {
        const result = await request<{ recovery_token: string }>(
          "/api/v1/auth/password-recovery",
          "POST",
          { email },
        );
        setToken(result.recovery_token);
      } else {
        await request("/api/v1/auth/password-recovery/complete", "POST", {
          recovery_token: token,
          code,
          new_password: password,
        });
        setPassword("");
        setConfirmation("");
        setCode("");
        setToken("");
        setDone(true);
      }
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-page">
      <section className="auth-card">
        <p className="eyebrow">XINGDU · 星渡</p>
        <h1>{text("找回密码", "Recover your password")}</h1>
        {done ? (
          <p role="status">
            {text(
              "密码已重置，所有旧登录会话已失效。请使用新密码登录。",
              "Password reset. All previous sessions have been revoked. Sign in with your new password.",
            )}
          </p>
        ) : (
          <form onSubmit={submit}>
            <p>
              {token
                ? text(
                    "验证码已发送，10 分钟内有效。仅支持已验证邮箱且启用密码登录的账号。",
                    "Code sent, valid for 10 minutes. Only verified accounts with password login can reset a password.",
                  )
                : text(
                    "请输入账号的已验证邮箱。第三方登录账号请使用原登录方式。",
                    "Enter your verified account email. Social-only accounts should use their existing sign-in provider.",
                  )}
            </p>
            <label htmlFor="recovery-email">{text("邮箱", "Email")}</label>
            <input
              id="recovery-email"
              type="email"
              autoComplete="email"
              required
              value={email}
              disabled={busy || !!token}
              onChange={(e) => setEmail(e.target.value)}
            />
            {token && (
              <>
                <label htmlFor="recovery-code">
                  {text("邮箱验证码", "Verification code")}
                </label>
                <input
                  id="recovery-code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]{8}"
                  maxLength={8}
                  required
                  value={code}
                  disabled={busy}
                  onChange={(e) => setCode(e.target.value)}
                />
                <label htmlFor="recovery-password">
                  {text("新密码", "New password")}
                </label>
                <input
                  id="recovery-password"
                  type="password"
                  autoComplete="new-password"
                  required
                  value={password}
                  disabled={busy}
                  onChange={(e) => setPassword(e.target.value)}
                />
                <label htmlFor="recovery-confirm">
                  {text("确认新密码", "Confirm new password")}
                </label>
                <input
                  id="recovery-confirm"
                  type="password"
                  autoComplete="new-password"
                  required
                  value={confirmation}
                  disabled={busy}
                  onChange={(e) => setConfirmation(e.target.value)}
                />
              </>
            )}
            {error && (
              <p role="alert" className="form-error">
                {error}
              </p>
            )}
            <button className="primary auth-submit" disabled={busy}>
              {busy
                ? text("正在处理…", "Processing…")
                : token
                  ? text("重置密码", "Reset password")
                  : text("发送验证码", "Send verification code")}
            </button>
            {token && (
              <button
                type="button"
                className="secondary auth-submit"
                disabled={busy}
                onClick={() => {
                  setToken("");
                  setCode("");
                  setPassword("");
                  setConfirmation("");
                  setError("");
                }}
              >
                {text(
                  "重新申请验证码 / 更换邮箱",
                  "Request another code / Change email",
                )}
              </button>
            )}
          </form>
        )}
        <button
          type="button"
          className="secondary auth-submit"
          disabled={busy}
          onClick={onBack}
        >
          {text("返回登录", "Back to sign in")}
        </button>
      </section>
    </div>
  );
}
