import { t, useLocale } from "./i18n";
import Select from "./Select";
import { useEffect, useRef, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  authConfig,
  acceptInvitation,
  createOrganization,
  errorMessage,
  listOrganizations,
  setOrganization,
  roleNames,
} from "./api";
import type { Organization, Session } from "./api";
import App from "./App";
import { organizationURL, writeURL } from "./navigation";
export default function OrganizationGate({
  session,
  onLogout,
}: {
  session: Session;
  onLogout: () => Promise<void>;
}) {
  useLocale();
  const [cloud, setCloud] = useState(false);
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [selected, setSelected] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const [invite, setInvite] = useState(
    () =>
      new URLSearchParams(window.location.hash.slice(1)).get("invite") ?? "",
  );
  useEffect(() => {
    const syncInvite = () =>
      setInvite(
        new URLSearchParams(window.location.hash.slice(1)).get("invite") ?? "",
      );
    window.addEventListener("hashchange", syncInvite);
    return () => window.removeEventListener("hashchange", syncInvite);
  }, []);
  function selectOrganization(
    id: string,
    replace = false,
    preserveDetail = false,
  ) {
    writeURL(
      organizationURL(new URL(window.location.href), id, !preserveDetail),
      replace,
    );
    setOrganization(id);
    setSelected(id);
    setError("");
  }
  function resolveOrganization(values: Organization[], preferred?: string) {
    const requested =
      preferred ??
      new URL(window.location.href).searchParams.get("organization");
    if (requested && !values.some((o) => o.id === requested)) {
      setOrganization("");
      setSelected("");
      setError(t("此组织不存在或你没有访问权限，请选择其他组织。"));
      return;
    }
    const next = requested || values[0]?.id || "";
    selectOrganization(next, !preferred, !preferred);
  }
  async function refresh(preferred?: string) {
    const values = await listOrganizations();
    setOrgs(values);
    resolveOrganization(values, preferred);
  }
  useEffect(() => {
    let active = true;
    Promise.all([listOrganizations(), authConfig()])
      .then(([values, config]) => {
        if (!active) return;
        setCloud(config.mode === "cloud");
        setOrgs(values);
        resolveOrganization(values);
      })
      .catch((e) => {
        if (active) setError(errorMessage(e));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      setOrganization("");
    };
  }, []);
  useEffect(() => {
    if (loading) return;
    const sync = () => resolveOrganization(orgs);
    window.addEventListener("popstate", sync);
    return () => window.removeEventListener("popstate", sync);
  }, [orgs, loading]);
  async function create(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const org = await createOrganization(name);
      await refresh(org.id);
      setCreating(false);
      setName("");
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  const org = orgs.find((o) => o.id === selected);
  const controls = (
    <div className="org-switch">
      <label htmlFor="organization-switch">{t("当前组织")}</label>
      {cloud ? <Select
        id="organization-switch"
        label={t("切换组织")}
        variant="organization"
        action={{ label: t("＋ 创建组织"), onClick: () => setCreating(true) }}
        value={selected}
        disabled={busy}
        options={orgs.map((o) => ({
          value: o.id,
          label: o.name,
          description: roleNames[o.role],
        }))}
        onChange={(value) => {
          selectOrganization(value);
        }}
      /> : <strong>{org?.name}</strong>}
    </div>
  );
  const createForm = (
    <form onSubmit={create}>
      <div className="dialog-heading">
        <div>
          <p className="eyebrow">NEW WORKSPACE</p>
          <h2 id="org-create-title">{t("创建组织")}</h2>
        </div>
        {org && (
          <button
            type="button"
            className="icon-button"
            aria-label={t("关闭创建组织")}
            disabled={busy}
            onClick={() => setCreating(false)}
          >
            ×
          </button>
        )}
      </div>
      <p className="form-hint">
        {t("为团队建立独立的协作空间，分别管理成员和服务器。")}
      </p>
      <label htmlFor="org-name">{t("组织名称")}</label>
      <input
        id="org-name"
        value={name}
        onChange={(e) => setName(e.target.value)}
        maxLength={64}
        required
        disabled={busy}
        placeholder={t("例如：我的团队")}
      />
      {error && (
        <p role="alert" className="form-error">
          {t(error)}
        </p>
      )}
      <div className="dialog-actions">
        {org && (
          <button
            type="button"
            className="secondary"
            onClick={() => setCreating(false)}
            disabled={busy}
          >
            {t("取消")}
          </button>
        )}
        <button className="primary" disabled={busy}>
          {busy ? t("正在创建…") : t("创建组织")}
        </button>
      </div>
    </form>
  );
  const overlay =
    creating && org ? (
      <OrganizationDialog busy={busy} onClose={() => setCreating(false)}>
        {createForm}
      </OrganizationDialog>
    ) : cloud && !org && !loading && !orgs.length && !error ? (
      <div className="org-create">{createForm}</div>
    ) : null;
  const banner = (
    <>
      {error && !creating && org && (
        <div className="team-banner" role="alert">
          {t(error)}
          <button
            className="secondary"
            onClick={() =>
              refresh()
                .then(() => setError(""))
                .catch((e) => setError(errorMessage(e)))
            }
          >
            {t("重试")}
          </button>
        </div>
      )}
      {invite && (
        <section className="team-banner">
          <div>
            <strong>{t("你收到了一份组织邀请")}</strong>
            <p>
              {t("接受后，当前账号")}
              {session.username}{" "}
              {t("将加入邀请者的组织。请确认链接来自可信的组织管理员。")}
            </p>
          </div>
          <button
            className="primary"
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              setError("");
              try {
                const result = await acceptInvitation(invite);
                const url = new URL(window.location.href);
                url.hash = "";
                writeURL(url, true);
                setInvite("");
                await refresh(result.organization_id);
              } catch (e) {
                setError(errorMessage(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            {t("接受邀请")}
          </button>
          <button
            className="secondary"
            disabled={busy}
            onClick={() => {
              setInvite("");
              const url = new URL(window.location.href);
              url.hash = "";
              writeURL(url, true);
            }}
          >
            {t("忽略")}
          </button>
        </section>
      )}
      {overlay}
    </>
  );
  if (loading)
    return (
      <div className="auth-page">
        <p role="status">{t("正在加载组织…")}</p>
      </div>
    );
  if (!org)
    return (
      <div className="auth-page">
        <section className="auth-card">
          {orgs.length > 0 && controls}
          {error && <p role="alert">{error}</p>}
          {error && (
            <button
              className="secondary"
              onClick={() =>
                void refresh().catch((e) => setError(errorMessage(e)))
              }
            >
              {t("重试")}
            </button>
          )}
          {banner}
          {!cloud && !orgs.length && <p>{t("你尚未加入此实例的组织，请联系管理员获取邀请。")}</p>}
          <button className="secondary" onClick={() => void onLogout()}>
            {t("退出登录")}
          </button>
        </section>
      </div>
    );
  return (
    <App
      cloud={cloud}
      key={org.id}
      username={session.username}
      onLogout={onLogout}
      organization={org}
      organizationControls={controls}
      organizationBanner={banner}
    />
  );
}

function OrganizationDialog({
  children,
  busy,
  onClose,
}: {
  children: ReactNode;
  busy: boolean;
  onClose: () => void;
}) {
  useLocale();
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const element = dialog.current;
    element?.showModal();
    element?.querySelector<HTMLInputElement>("#org-name")?.focus();
    return () => element?.close();
  }, []);
  return (
    <dialog
      ref={dialog}
      className="host-dialog organization-dialog"
      aria-labelledby="org-create-title"
      onCancel={(event) => {
        event.preventDefault();
        if (!busy) onClose();
      }}
    >
      {children}
    </dialog>
  );
}
