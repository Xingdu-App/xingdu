import Select from "./Select";
import { useEffect, useRef, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  acceptInvitation,
  createOrganization,
  errorMessage,
  listOrganizations,
  setOrganization,
  roleNames,
} from "./api";
import type { Organization, Session } from "./api";
import App from "./App";
export default function OrganizationGate({
  session,
  onLogout,
}: {
  session: Session;
  onLogout: () => Promise<void>;
}) {
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
  async function refresh(preferred?: string) {
    const values = await listOrganizations();
    setOrgs(values);
    const next =
      values.find((o) => o.id === preferred)?.id ?? values[0]?.id ?? "";
    setOrganization(next);
    setSelected(next);
  }
  useEffect(() => {
    let active = true;
    listOrganizations()
      .then((values) => {
        if (!active) return;
        setOrgs(values);
        const id = values[0]?.id ?? "";
        setOrganization(id);
        setSelected(id);
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
      <label htmlFor="organization-switch">当前组织</label>
      <Select
        id="organization-switch"
        label="切换组织"
        variant="organization"
        value={selected}
        disabled={busy}
        options={orgs.map((o) => ({
          value: o.id,
          label: o.name,
          description: roleNames[o.role],
        }))}
        onChange={(value) => {
          setOrganization(value);
          setSelected(value);
          setError("");
        }}
      />
      <button
        className="org-create-button"
        onClick={() => setCreating(!creating)}
      >
        ＋ 创建组织
      </button>
    </div>
  );
  const createForm = (
    <form onSubmit={create}>
      <div className="dialog-heading">
        <div>
          <p className="eyebrow">NEW WORKSPACE</p>
          <h2 id="org-create-title">创建组织</h2>
        </div>
        {org && (
          <button
            type="button"
            className="icon-button"
            aria-label="关闭创建组织"
            disabled={busy}
            onClick={() => setCreating(false)}
          >
            ×
          </button>
        )}
      </div>
      <p className="form-hint">
        为团队建立独立的协作空间，分别管理成员和服务器。
      </p>
      <label htmlFor="org-name">组织名称</label>
      <input
        id="org-name"
        value={name}
        onChange={(e) => setName(e.target.value)}
        maxLength={64}
        required
        disabled={busy}
        placeholder="例如：我的团队"
      />
      {error && (
        <p role="alert" className="form-error">
          {error}
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
            取消
          </button>
        )}
        <button className="primary" disabled={busy}>
          {busy ? "正在创建…" : "创建组织"}
        </button>
      </div>
    </form>
  );
  const overlay =
    creating && org ? (
      <OrganizationDialog busy={busy} onClose={() => setCreating(false)}>
        {createForm}
      </OrganizationDialog>
    ) : !org && !loading ? (
      <div className="org-create">{createForm}</div>
    ) : null;
  const banner = (
    <>
      {error && !creating && org && (
        <div className="team-banner" role="alert">
          {error}
          <button
            className="secondary"
            onClick={() =>
              refresh(selected)
                .then(() => setError(""))
                .catch((e) => setError(errorMessage(e)))
            }
          >
            重试
          </button>
        </div>
      )}
      {invite && (
        <section className="team-banner">
          <div>
            <strong>你收到了一份组织邀请</strong>
            <p>
              接受后，当前账号 {session.username}{" "}
              将加入邀请者的组织。请确认链接来自可信的组织管理员。
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
                window.history.replaceState(null, "", window.location.pathname);
                setInvite("");
                await refresh(result.organization_id);
              } catch (e) {
                setError(errorMessage(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            接受邀请
          </button>
          <button
            className="secondary"
            disabled={busy}
            onClick={() => {
              setInvite("");
              window.history.replaceState(null, "", window.location.pathname);
            }}
          >
            忽略
          </button>
        </section>
      )}
      {overlay}
    </>
  );
  if (loading)
    return (
      <div className="auth-page">
        <p role="status">正在加载组织…</p>
      </div>
    );
  if (!org)
    return (
      <div className="auth-page">
        <section className="auth-card">
          {banner}
          <button className="secondary" onClick={() => void onLogout()}>
            退出登录
          </button>
        </section>
      </div>
    );
  return (
    <App
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
