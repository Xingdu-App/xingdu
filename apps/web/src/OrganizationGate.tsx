import { useEffect, useState } from "react";
import type { FormEvent } from "react";
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
      <select
        id="organization-switch"
        value={selected}
        disabled={busy}
        onChange={(e) => {
          setOrganization(e.target.value);
          setSelected(e.target.value);
          setError("");
        }}
      >
        {orgs.map((o) => (
          <option key={o.id} value={o.id}>
            {o.name} · {roleNames[o.role]}
          </option>
        ))}
      </select>
      <button className="secondary" onClick={() => setCreating(!creating)}>
        ＋ 创建组织
      </button>
    </div>
  );
  const overlay = (creating || (!org && !loading)) && (
    <div className="org-create">
      <form onSubmit={create}>
        <h2>创建组织</h2>
        <label htmlFor="org-name">组织名称</label>
        <input
          id="org-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={64}
          required
          disabled={busy}
        />
        <button className="primary" disabled={busy}>
          创建组织
        </button>
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
      </form>
    </div>
  );
  const banner = (
    <>
      {error && (
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
