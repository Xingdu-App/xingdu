import { t, useLocale, localeTag } from "./i18n";
import { useEffect, useState } from "react";
import { errorMessage } from "./api";
import {
  changePassword,
  getProfile,
  getSessions,
  revokeSession,
  saveProfile,
} from "./account-api";
import type { AccountSession } from "./account-api";
import "./AccountForms.css";
import LoginMethods from "./LoginMethods";
export function ProfileForm() {
  useLocale();
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    const c = new AbortController();
    getProfile(c.signal)
      .then((v) => {
        setName(v.display_name);
        setBusy(false);
      })
      .catch((e) => {
        if (!c.signal.aborted) {
          setError(errorMessage(e));
          setBusy(false);
        }
      });
    return () => c.abort();
  }, []);
  return (
    <form
      className="account-form"
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        setError("");
        setNotice("");
        try {
          await saveProfile(name);
          setNotice(t("账户资料已保存"));
        } catch (e) {
          setError(errorMessage(e));
        } finally {
          setBusy(false);
        }
      }}
    >
      <label>
        {t("显示名称")}
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={64}
          required
          disabled={busy}
          autoComplete="nickname"
        />
      </label>
      <p className="form-hint">
        {t("显示名称用于账户资料，登录用户名保持不变。")}
      </p>
      {error && <p role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}
      <button className="secondary" disabled={busy}>
        {busy ? t("处理中…") : t("保存资料")}
      </button>
    </form>
  );
}
export function SecurityForms() {
  useLocale();
  const [hasPassword, setHasPassword] = useState<boolean | null>(null);
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [sessions, setSessions] = useState<AccountSession[]>([]);
  const [sessionError, setSessionError] = useState("");
  const [loading, setLoading] = useState(true);
  const [revoking, setRevoking] = useState("");
  const [reload, setReload] = useState(0);
  useEffect(() => {
    const c = new AbortController();
    getSessions(c.signal)
      .then((v) => {
        setSessions(v);
        setSessionError("");
      })
      .catch((e) => {
        if (!c.signal.aborted) setSessionError(errorMessage(e));
      })
      .finally(() => {
        if (!c.signal.aborted) setLoading(false);
      });
    return () => c.abort();
  }, [reload]);
  return (
    <div className="account-security-sections">
      <LoginMethods onHasPassword={setHasPassword} />
      {hasPassword === true && (
        <form
          className="account-form"
          onSubmit={async (e) => {
            e.preventDefault();
            setError("");
            if (next !== confirm) {
              setError(t("两次输入的新密码不一致"));
              return;
            }
            setBusy(true);
            try {
              await changePassword(current, next);
              setCurrent("");
              setNext("");
              setConfirm("");
              window.dispatchEvent(new Event("xingdu:unauthorized"));
            } catch (e) {
              setError(errorMessage(e));
            } finally {
              setBusy(false);
            }
          }}
        >
          <h3>{t("修改密码")}</h3>
          <p className="form-hint">
            {t(
              "修改后所有登录会话（包括当前会话）立即失效，请使用新密码重新登录。",
            )}
          </p>
          <label>
            {t("当前密码")}
            <input
              type="password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
              autoComplete="current-password"
              required
              disabled={busy}
            />
          </label>
          <label>
            {t("新密码")}
            <input
              type="password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
              autoComplete="new-password"
              minLength={12}
              maxLength={72}
              required
              disabled={busy}
            />
          </label>
          <label>
            {t("确认新密码")}
            <input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              autoComplete="new-password"
              minLength={12}
              maxLength={72}
              required
              disabled={busy}
            />
          </label>
          <p className="form-hint">
            {t("使用至少 12 个字符的独立密码；服务器限制为 72 字节。")}
          </p>
          {error && <p role="alert">{error}</p>}
          <button className="secondary" disabled={busy}>
            {busy ? t("修改中…") : t("修改密码并重新登录")}
          </button>
        </form>
      )}
      <section className="account-session-section">
        <div className="section-heading">
          <h3>{t("登录会话")}</h3>
          <button
            className="secondary"
            disabled={loading}
            onClick={() => {
              setLoading(true);
              setReload((v) => v + 1);
            }}
          >
            {t("刷新")}
          </button>
        </div>
        <p className="form-hint">
          {t(
            "按登录时间识别会话。每次登录最多保持 24 小时；不采集设备指纹或精确位置。",
          )}
        </p>
        {sessionError && <p role="alert">{sessionError}</p>}
        {loading && <p role="status">{t("加载中…")}</p>}
        <ul className="account-sessions">
          {sessions.map((s) => (
            <li key={s.id}>
              <div>
                <strong>{s.current ? t("当前会话") : t("其他登录会话")}</strong>
                <span>
                  {t("登录于 {0}", {
                    0: new Date(s.created_at).toLocaleString(localeTag()),
                  })}
                </span>
                <span>
                  {t("到期于 {0}", {
                    0: new Date(s.expires_at).toLocaleString(localeTag()),
                  })}
                </span>
              </div>
              <button
                className="secondary"
                disabled={!!revoking}
                onClick={async () => {
                  setRevoking(s.id);
                  setSessionError("");
                  try {
                    await revokeSession(s.id);
                    if (s.current)
                      window.dispatchEvent(new Event("xingdu:unauthorized"));
                    else setReload((v) => v + 1);
                  } catch (e) {
                    setSessionError(errorMessage(e));
                  } finally {
                    setRevoking("");
                  }
                }}
              >
                {revoking === s.id
                  ? t("撤销中…")
                  : s.current
                    ? t("退出当前会话")
                    : t("撤销会话")}
              </button>
            </li>
          ))}
        </ul>
      </section>
      <p className="form-hint">
        {t("Passkey（Face ID / Touch ID）、双重验证和忘记密码恢复尚未开放。")}
      </p>
    </div>
  );
}
