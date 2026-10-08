import { useEffect, useRef, useState } from "react";
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
  "certificates:read": "查看证书状态",
  "certificates:write": "管理域名和证书签发",
  "exits:read": "查看外部代理出口",
  "exits:write": "管理外部代理出口及线路选择",
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
  const [editing, setEditing] = useState<Key | null>(null);
  const [creating, setCreating] = useState(false);
  const createDialog = useRef<HTMLDialogElement>(null);
  const closeCreate = () => {
    if (busy) return;
    setCreating(false);
    setSecret("");
    setCopied(false);
  };
  useEffect(() => {
    if (creating && !createDialog.current?.open)
      createDialog.current?.showModal();
    else if (!creating) createDialog.current?.close();
  }, [creating]);
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
          {error && !creating && (
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
          <dialog
            ref={createDialog}
            className="host-dialog machine-dialog api-key-create-dialog"
            aria-labelledby="api-key-create-title"
            onCancel={(e) => {
              e.preventDefault();
              closeCreate();
            }}
            onClose={() => {
              setCreating(false);
              setSecret("");
              setCopied(false);
            }}
          >
            <div className="dialog-heading">
              <h2 id="api-key-create-title">
                {t(
                  secret
                    ? "请立即保存密钥"
                    : editing
                      ? "修改权限"
                      : "创建 API 密钥",
                )}
              </h2>
              <button
                type="button"
                className="icon-button"
                aria-label={t("关闭")}
                disabled={busy}
                onClick={closeCreate}
              >
                ×
              </button>
            </div>
            {error && (
              <p className="form-error api-key-dialog-error" role="alert">
                {t(error)}
              </p>
            )}
            {secret && (
              <section className="machine-content api-key-secret" role="status">
                <p>
                  {t(
                    "完整密钥仅显示这一次，关闭弹窗后无法再次查看。不要将它放入前端代码或 Git 仓库。",
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
                    disabled={busy}
                    onClick={closeCreate}
                  >
                    {t("已保存，隐藏密钥")}
                  </button>
                </div>
              </section>
            )}

            {!secret && (
              <form
                id="api-key-create-form"
                className="machine-content api-key-form"
                onSubmit={(e) => {
                  e.preventDefault();
                  void act(async () => {
                    if (editing) {
                      await request<Key>(
                        "/api/v1/api-keys/" + editing.id,
                        "PATCH",
                        { scopes },
                      );
                      setKeys((items) =>
                        items.map((key) =>
                          key.id === editing.id
                            ? { ...key, scopes: [...scopes] }
                            : key,
                        ),
                      );
                      setCreating(false);
                      return;
                    }
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
                {!editing && (
                  <>
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
                    <label>
                      {t("有效期")}
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
                    </label>
                  </>
                )}
                {editing && (
                  <p className="subtitle">
                    {editing.name} · {t("权限修改立即生效，现有密钥无需更换。")}
                  </p>
                )}
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
              </form>
            )}
            {!secret && (
              <div className="dialog-footer">
                <button
                  type="button"
                  className="secondary"
                  disabled={busy}
                  onClick={closeCreate}
                >
                  {t("取消")}
                </button>
                <button
                  type="submit"
                  form="api-key-create-form"
                  className="primary"
                  disabled={
                    busy || (!editing && !name.trim()) || scopes.length === 0
                  }
                >
                  {busy ? t("处理中…") : t(editing ? "保存权限" : "创建密钥")}
                </button>
              </div>
            )}
          </dialog>
          <section className="panel api-key-list">
            <div className="api-key-heading">
              <h2>{t("组织密钥")}</h2>
              <div className="api-key-actions">
                <button className="secondary" onClick={onDocs}>
                  {t("API 文档")}
                </button>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => void act(refresh)}
                >
                  {t("刷新")}
                </button>
                <button
                  className="primary"
                  disabled={busy}
                  onClick={() => {
                    setError("");
                    setEditing(null);
                    setName("");
                    setDays("90");
                    setScopes(["hosts:read", "nodes:read"]);
                    setCreating(true);
                  }}
                >
                  {t("创建密钥")}
                </button>
              </div>
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
                        ? new Date(key.last_used_at).toLocaleString(localeTag())
                        : t("尚未使用")}
                    </p>
                    <div className="api-key-actions">
                      {!key.revoked_at && !expired && (
                        <button
                          className="secondary"
                          disabled={busy}
                          onClick={() => {
                            setError("");
                            setEditing(key);
                            setScopes([...key.scopes]);
                            setCreating(true);
                          }}
                        >
                          {t("修改权限")}
                        </button>
                      )}
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
                    </div>
                  </article>
                );
              })
            )}
          </section>
        </>
      )}
    </section>
  );
}
