import { useEffect, useState } from "react";
import { APIError, authConfig, errorMessage, startOAuth } from "./api";
import { getLoginIdentities, unlinkLoginIdentity } from "./account-api";
import type { LoginIdentities } from "./account-api";
import {
  canUnlinkIdentity,
  oauthAuthorizationURL,
  oauthFeedback,
  oauthProviderNames,
} from "./oauth";
import type { OAuthProvider, OAuthProviders } from "./oauth";
import { t, useLocale } from "./i18n";

export default function LoginMethods({
  onHasPassword,
}: {
  onHasPassword: (value: boolean | null) => void;
}) {
  useLocale();
  const [methods, setMethods] = useState<LoginIdentities | null>(null);
  const [providers, setProviders] = useState<OAuthProviders>({
    google: false,
    github: false,
  });
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<OAuthProvider | "">("");
  const [confirmation, setConfirmation] = useState<OAuthProvider | null>(null);
  const [error, setError] = useState(() => {
    const feedback = oauthFeedback(new URL(window.location.href));
    return feedback.error ? t(feedback.message) : "";
  });
  const [notice, setNotice] = useState(() => {
    const feedback = oauthFeedback(new URL(window.location.href));
    return feedback.error ? "" : t(feedback.message);
  });
  const [reload, setReload] = useState(0);
  useEffect(() => {
    const feedback = oauthFeedback(new URL(window.location.href));
    if (feedback.consumed)
      window.history.replaceState(window.history.state, "", feedback.cleanPath);
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    Promise.allSettled([
      getLoginIdentities(controller.signal),
      authConfig(controller.signal),
    ]).then(([identities, config]) => {
      if (controller.signal.aborted) return;
      if (identities.status === "fulfilled") {
        setMethods(identities.value);
        onHasPassword(identities.value.has_password);
      } else {
        setMethods(null);
        onHasPassword(null);
        setError(errorMessage(identities.reason));
      }
      if (config.status === "fulfilled")
        setProviders(
          config.value.oauth_providers ?? { google: false, github: false },
        );
      else {
        setProviders({ google: false, github: false });
        setError(t("暂时无法读取第三方登录配置，请刷新重试。"));
      }
      setLoading(false);
    });
    return () => controller.abort();
  }, [reload, onHasPassword]);
  async function link(provider: OAuthProvider) {
    if (busy || !providers[provider]) return;
    setBusy(provider);
    setError("");
    setNotice("");
    try {
      const result = await startOAuth(provider, "link");
      window.location.assign(
        oauthAuthorizationURL(provider, result.authorization_url),
      );
    } catch (reason) {
      setError(errorMessage(reason));
      setBusy("");
    }
  }
  async function unlink(provider: OAuthProvider) {
    if (busy) return;
    setBusy(provider);
    setError("");
    setNotice("");
    try {
      await unlinkLoginIdentity(provider);
      setConfirmation(null);
      setNotice(t("登录方式已解绑。"));
      setLoading(true);
      setReload((value) => value + 1);
    } catch (reason) {
      setError(
        reason instanceof APIError && reason.code === "last_login_method"
          ? t("至少保留一种登录方式，不能解绑最后一个账号。")
          : errorMessage(reason),
      );
    } finally {
      setBusy("");
    }
  }
  return (
    <section aria-labelledby="login-methods-heading">
      <div className="section-heading">
        <h3 id="login-methods-heading">{t("登录方式")}</h3>
        <button
          className="secondary"
          type="button"
          disabled={loading || !!busy}
          onClick={() => {
            setLoading(true);
            setError("");
            setReload((value) => value + 1);
          }}
        >
          {t("刷新")}
        </button>
      </div>
      <p className="form-hint">
        {t(
          "使用相同已验证邮箱登录时会自动关联账号。不同邮箱也可在这里手动关联；解绑不会删除 Google 或 GitHub 账号。",
        )}
      </p>
      {error && (
        <p className="form-error" role="alert">
          {t(error)}
        </p>
      )}
      {notice && <p role="status">{notice}</p>}
      {loading && <p role="status">{t("正在加载登录方式…")}</p>}
      {methods && (
        <>
          <p className="form-hint">
            {methods.has_password
              ? t("已设置登录密码，可使用邮箱或原用户名登录。")
              : t(
                  "此账号仅使用第三方身份登录，尚未设置密码。目前不提供新增密码，请保留至少一种已绑定登录方式。",
                )}
          </p>
          <ul className="identity-methods">
            {(["google", "github"] as const).map((provider) => {
              const identity = methods.identities.find(
                (item) => item.provider === provider,
              );
              const lastSocialMethod = !canUnlinkIdentity(
                provider,
                methods.has_password,
                methods.identities,
                providers,
              );
              return (
                <li key={provider}>
                  <div>
                    <strong>{oauthProviderNames[provider]}</strong>
                    <small>
                      {identity ? t("已绑定") : t("未绑定")}
                      {identity?.email ? ` · ${identity.email}` : ""}
                    </small>
                    {!providers[provider] && (
                      <small>
                        {t("该服务当前未配置，无法进行新的授权登录。")}
                      </small>
                    )}
                    {identity && lastSocialMethod && (
                      <small>{t("请先保留另一种可用登录方式，再解绑。")}</small>
                    )}
                  </div>
                  {identity ? (
                    <button
                      className="secondary"
                      type="button"
                      disabled={loading || !!busy || lastSocialMethod}
                      onClick={() => {
                        setConfirmation(provider);
                        setError("");
                        setNotice("");
                      }}
                    >
                      {t("解绑")}
                    </button>
                  ) : (
                    <button
                      className="secondary"
                      type="button"
                      disabled={loading || !!busy || !providers[provider]}
                      onClick={() => void link(provider)}
                    >
                      {busy === provider ? t("正在处理…") : t("绑定")}
                    </button>
                  )}
                  {confirmation === provider && identity && (
                    <div className="identity-confirmation">
                      <p>
                        {t(
                          "解绑 {0} 后将移除当前关联；再次使用相同已验证邮箱登录时会重新关联。请确认其他登录方式可用。",
                          { 0: oauthProviderNames[provider] },
                        )}
                      </p>
                      <button
                        className="secondary"
                        type="button"
                        disabled={!!busy || loading || lastSocialMethod}
                        onClick={() => void unlink(provider)}
                      >
                        {t("确认解绑")}
                      </button>
                      <button
                        className="secondary"
                        type="button"
                        disabled={!!busy}
                        onClick={() => setConfirmation(null)}
                      >
                        {t("取消")}
                      </button>
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        </>
      )}
    </section>
  );
}
