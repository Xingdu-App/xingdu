import { useEffect, useState } from "react";
import { request, errorMessage, type Organization } from "./api";
import { t, useLocale, localeTag } from "./i18n";
import Select from "./Select";
import "./api-keys.css";

type Key = {
  id: string;
  name: string;
  prefix: string;
  scopes: string[];
  created_by: string;
  expires_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
};
const scopeNames: Record<string, string> = {
  "subscriptions:read": "查看订阅设置",
  "subscriptions:write": "管理订阅设置",
  "subscriptions:export": "导出订阅配置及连接凭据",
  "hosts:read": "查看服务器",
  "hosts:write": "管理服务器",
  "nodes:read": "查看节点",
  "nodes:write": "安装、重启和卸载节点",
  "nodes:probe": "上报节点探测结果",
  "nodes:credentials": "读取节点连接凭据",
};
export default function APIKeysPanel({
  organization,
  onDocs,
}: {
  organization: Organization;
  onDocs: () => void;
}) {
  useLocale();
  const manage = ["owner", "admin"].includes(organization.role);
  const [keys, setKeys] = useState<Key[]>([]);
  const [name, setName] = useState("");
  const [days, setDays] = useState("90");
  const [scopes, setScopes] = useState<string[]>(["hosts:read", "nodes:read"]);
  const [secret, setSecret] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [revoke, setRevoke] = useState<Key | null>(null);
  const [copied, setCopied] = useState(false);
  const [now, setNow] = useState(() => Date.now());
  const refresh = async () => {
    setKeys(await request<Key[]>("/api/v1/api-keys"));
    setLoaded(true);
  };
  useEffect(() => {
    if (!manage) return;
    const abort = new AbortController();
    request<Key[]>("/api/v1/api-keys", "GET", undefined, abort.signal)
      .then((data) => {
        if (!abort.signal.aborted) {
          setKeys(data);
          setLoaded(true);
        }
      })
      .catch((e) => {
        if (!abort.signal.aborted) setError(errorMessage(e));
      });
    return () => abort.abort();
  }, [manage]);
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 60000);
    return () => window.clearInterval(timer);
  }, []);
  async function act(fn: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="api-keys-page">
      {!manage ? (
        <div className="notice">
          {t("只有组织所有者和管理员可以管理 API 密钥。")}
        </div>
      ) : (
        <>
          {error && (
            <div className="form-error" role="alert">
              {t(error)}{" "}
              <button
                className="secondary"
                disabled={busy}
                onClick={() => void act(refresh)}
              >
                {t("重试")}
              </button>
            </div>
          )}
          {secret && (
            <section className="panel api-key-secret" role="status">
              <h2>{t("请立即保存密钥")}</h2>
              <p>
                {t(
                  "完整密钥仅显示这一次，离开页面后无法再次查看。不要将它放入前端代码或 Git 仓库。",
                )}
              </p>
              <code>{secret}</code>
              <div className="api-key-actions">
                <button
                  className="secondary"
                  onClick={() =>
                    void act(async () => {
                      await navigator.clipboard.writeText(secret);
                      setCopied(true);
                    })
                  }
                >
                  {copied ? t("已复制") : t("复制密钥")}
                </button>
                <button
                  className="secondary"
                  onClick={() => {
                    setSecret("");
                    setCopied(false);
                  }}
                >
                  {t("已保存，隐藏密钥")}
                </button>
              </div>
            </section>
          )}
          <div className="api-key-layout">
            <form
              className="panel api-key-form"
              onSubmit={(e) => {
                e.preventDefault();
                void act(async () => {
                  const result = await request<{ key: Key; secret: string }>(
                    "/api/v1/api-keys",
                    "POST",
                    { name, scopes, expires_in_days: Number(days) },
                  );
                  setSecret(result.secret);
                  setCopied(false);
                  setName("");
                  await refresh();
                });
              }}
            >
              <h2>{t("创建 API 密钥")}</h2>
              <label>
                {t("密钥名称")}
                <input
                  required
                  maxLength={64}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder={t("例如：自动部署脚本")}
                  disabled={busy}
                />
              </label>
              <Select
                label={t("有效期")}
                value={days}
                onChange={setDays}
                disabled={busy}
                options={["30", "90", "365"].map((value) => ({
                  value,
                  label: t("{0} 天", { 0: value }),
                }))}
              />
              <fieldset disabled={busy}>
                <legend>{t("授权范围")}</legend>
                {Object.entries(scopeNames).map(([scope, label]) => (
                  <label className="api-key-scope" key={scope}>
                    <input
                      type="checkbox"
                      checked={scopes.includes(scope)}
                      onChange={(e) =>
                        setScopes(
                          e.target.checked
                            ? [...scopes, scope]
                            : scopes.filter((s) => s !== scope),
                        )
                      }
                    />
                    <span>
                      {t(label)}
                      <small>{scope}</small>
                    </span>
                  </label>
                ))}
              </fieldset>
              <p className="subtitle">
                {t(
                  "节点凭据读取会返回连接密码等敏感信息，请仅向需要的脚本开放。",
                )}
              </p>
              <button
                className="primary"
                disabled={
                  busy || !name.trim() || scopes.length === 0 || !!secret
                }
              >
                {busy ? t("处理中…") : t("创建密钥")}
              </button>
            </form>
            <section className="panel api-key-list">
              <div className="section-heading">
                <h2>{t("组织密钥")}</h2>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => void act(refresh)}
                >
                  {t("刷新")}
                </button>
              </div>
              <p className="subtitle">
                {t(
                  "密钥绑定创建者身份；创建者退出组织或失去管理员权限后，密钥将无法使用。",
                )}
              </p>
              {!loaded ? (
                <p>{t("加载中")}</p>
              ) : keys.length === 0 ? (
                <p>{t("暂无 API 密钥")}</p>
              ) : (
                keys.map((key) => {
                  const expired = Date.parse(key.expires_at) <= now;
                  return (
                    <article className="api-key-item" key={key.id}>
                      <div className="api-key-row">
                        <strong>{key.name}</strong>
                        <span className="badge">
                          {key.revoked_at
                            ? t("已撤销")
                            : expired
                              ? t("已过期")
                              : t("有效")}
                        </span>
                      </div>
                      <code>{key.prefix}…</code>
                      <small>{key.id}</small>
                      <div className="api-key-scopes">
                        {key.scopes.map((scope) => (
                          <span className="badge" key={scope}>
                            {t(scopeNames[scope] ?? scope)}
                          </span>
                        ))}
                      </div>
                      <p>
                        {t("到期时间")} ·{" "}
                        {new Date(key.expires_at).toLocaleString(localeTag())}
                      </p>
                      <p>
                        {t("最近使用")} ·{" "}
                        {key.last_used_at
                          ? new Date(key.last_used_at).toLocaleString(
                              localeTag(),
                            )
                          : t("尚未使用")}
                      </p>
                      {!key.revoked_at &&
                        (revoke?.id === key.id ? (
                          <div className="api-key-actions">
                            <span>{t("撤销后不可恢复。")}</span>
                            <button
                              className="danger"
                              disabled={busy}
                              onClick={() =>
                                void act(async () => {
                                  await request(
                                    "/api/v1/api-keys/" + key.id,
                                    "DELETE",
                                  );
                                  setRevoke(null);
                                  setSecret("");
                                  await refresh();
                                })
                              }
                            >
                              {t("确认撤销")}
                            </button>
                            <button
                              className="secondary"
                              disabled={busy}
                              onClick={() => setRevoke(null)}
                            >
                              {t("取消")}
                            </button>
                          </div>
                        ) : (
                          <button
                            className="secondary"
                            disabled={busy}
                            onClick={() => setRevoke(key)}
                          >
                            {t("撤销")}
                          </button>
                        ))}
                    </article>
                  );
                })
              )}
            </section>
          </div>
          <section className="panel api-key-help">
            <h2>{t("API 文档")}</h2>
            <p>{t("查看认证方式、接口权限和可运行的请求示例。")}</p>
            <button className="secondary" onClick={onDocs}>
              {t("查看 API 文档 →")}
            </button>
          </section>
        </>
      )}
    </section>
  );
}
