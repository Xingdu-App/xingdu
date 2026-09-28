import Select from "./Select";
import { useEffect, useState } from "react";
import {
  changeMember,
  createInvitation,
  errorMessage,
  listInvitations,
  listMembers,
  removeMember,
  revokeInvitation,
  roleNames,
} from "./api";
import type { Invitation, Member, Organization, Role } from "./api";
export default function TeamPanel({
  organization,
}: {
  organization: Organization;
}) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 60000);
    return () => window.clearInterval(timer);
  }, []);
  const [members, setMembers] = useState<Member[]>([]),
    [invitations, setInvitations] = useState<Invitation[]>([]);
  const [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [link, setLink] = useState(""),
    [role, setRole] = useState<Role>("member"),
    [removing, setRemoving] = useState<Member | null>(null),
    [loaded, setLoaded] = useState(false);
  const manage = organization.role === "owner" || organization.role === "admin";
  async function refresh() {
    const [m, i] = await Promise.all([
      listMembers(),
      manage ? listInvitations() : Promise.resolve([]),
    ]);
    setMembers(m);
    setInvitations(i);
    setLoaded(true);
  }
  useEffect(() => {
    let active = true;
    Promise.all([
      listMembers(),
      manage ? listInvitations() : Promise.resolve([]),
    ])
      .then(([m, i]) => {
        if (active) {
          setMembers(m);
          setInvitations(i);
          setLoaded(true);
        }
      })
      .catch((e) => {
        if (active) setError(errorMessage(e));
      });
    return () => {
      active = false;
    };
  }, [manage]);
  async function act(action: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await action();
      await refresh();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  const roles: Role[] =
    organization.role === "owner"
      ? ["admin", "member", "viewer"]
      : ["member", "viewer"];
  const descriptions: Record<Role, string> = {
    owner: "管理组织、成员与全部资源",
    admin: "管理成员、服务器与机器接入",
    member: "新增服务器资料，查看组织资源",
    viewer: "仅查看组织资源，无法进行修改",
  };
  const roleOptions = roles.map((r) => ({
    value: r,
    label: roleNames[r],
    description: descriptions[r],
  }));
  const pendingCount = invitations.filter(
    (i) =>
      !i.accepted_at && !i.revoked_at && new Date(i.expires_at).getTime() > now,
  ).length;
  return (
    <section className="team-page">
      <div className="page-heading">
        <div>
          <p className="eyebrow">WORKSPACE & PEOPLE</p>
          <h1>组织与成员</h1>
          <p className="subtitle">在同一个空间里，管理资源与协作权限。</p>
        </div>
        <span className="role-badge">{roleNames[organization.role]}</span>
      </div>
      <div className="organization-summary">
        <span className="organization-mark" aria-hidden="true">
          {organization.name.slice(0, 1)}
        </span>
        <div className="organization-summary-name">
          <h2>{organization.name}</h2>
          <p>当前组织 · {descriptions[organization.role]}</p>
        </div>
        <div className="organization-count">
          <strong>{loaded ? members.length : "—"}</strong>
          <span>组织成员</span>
        </div>
        {manage && (
          <div className="organization-count">
            <strong>{loaded ? pendingCount : "—"}</strong>
            <span>待接受邀请</span>
          </div>
        )}
      </div>
      {error && (
        <div role="alert" className="form-error team-error">
          {error}
          <button
            className="secondary"
            disabled={busy}
            onClick={() => void act(refresh)}
          >
            重试
          </button>
        </div>
      )}
      <div className={`team-grid${manage ? "" : " team-grid-readonly"}`}>
        <section className="panel team-members">
          <div className="section-heading">
            <div>
              <h2>组织成员</h2>
              <p>查看成员身份，分配合适的访问权限。</p>
            </div>
            <span className="badge">
              {loaded ? `${members.length} 位成员` : "加载中"}
            </span>
          </div>
          {!loaded && !error && (
            <p className="team-empty" role="status">
              正在加载成员…
            </p>
          )}
          <div className="member-list">
            {members.map((m) => (
              <article key={m.id} className="member-row">
                <div className="member-identity">
                  <span className="member-avatar" aria-hidden="true">
                    {m.username.slice(0, 1).toUpperCase()}
                  </span>
                  <div>
                    <strong>{m.username}</strong>
                    <small>{descriptions[m.role]}</small>
                  </div>
                </div>
                {manage &&
                m.role !== "owner" &&
                (organization.role === "owner" || m.role !== "admin") ? (
                  <div className="member-actions">
                    <Select
                      label={`${m.username} 的角色`}
                      value={m.role}
                      disabled={busy}
                      options={roleOptions}
                      onChange={(value) =>
                        void act(() => changeMember(m.id, value as Role))
                      }
                    />
                    <button
                      className="member-remove"
                      disabled={busy}
                      onClick={() => setRemoving(m)}
                    >
                      移除
                    </button>
                  </div>
                ) : (
                  <span className="role-badge">{roleNames[m.role]}</span>
                )}
              </article>
            ))}
          </div>
          {removing && (
            <div className="team-banner remove-banner" role="alert">
              <p>
                移除 {removing.username} 后，对方将立即失去此组织的访问权限。
              </p>
              <div className="member-actions">
                <button
                  className="danger"
                  disabled={busy}
                  onClick={() =>
                    void act(async () => {
                      await removeMember(removing.id);
                      setRemoving(null);
                    })
                  }
                >
                  确认移除
                </button>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => setRemoving(null)}
                >
                  取消
                </button>
              </div>
            </div>
          )}
          <div className="panel-note">
            {manage
              ? "组织所有者的身份固定，其他成员按角色获得相应权限。"
              : "成员与邀请由组织所有者或管理员管理。"}
          </div>
        </section>
        {manage && (
          <section className="panel team-invite">
            <div className="section-heading">
              <div>
                <h2>邀请新成员</h2>
                <p>让协作者加入当前组织。</p>
              </div>
              <span className="invite-symbol" aria-hidden="true">
                ＋
              </span>
            </div>
            <div className="panel-content">
              <label className="field-label" htmlFor="invite-role">
                成员角色
              </label>
              <Select
                id="invite-role"
                label="邀请角色"
                value={role}
                disabled={busy}
                options={roleOptions}
                onChange={(value) => setRole(value as Role)}
              />
              <button
                className="primary invite-submit"
                disabled={busy}
                onClick={() =>
                  void act(async () => {
                    const value = await createInvitation(role);
                    setLink(value.url);
                  })
                }
              >
                {busy ? "正在处理…" : "生成邀请链接"}
                <span aria-hidden="true">↗</span>
              </button>
              <p className="invite-help">
                链接 7 天内有效，仅可使用一次。请私下分享给目标成员。
              </p>
              {link && (
                <div className="invite-result">
                  <label htmlFor="invite-link">邀请链接 · 仅本次显示</label>
                  <input
                    id="invite-link"
                    readOnly
                    value={link}
                    onFocus={(e) => e.target.select()}
                  />
                  <button
                    className="secondary"
                    onClick={() =>
                      void navigator.clipboard
                        .writeText(link)
                        .catch(() => setError("复制失败，请选中链接手动复制。"))
                    }
                  >
                    复制链接
                  </button>
                </div>
              )}
            </div>
          </section>
        )}
      </div>
      {manage && (
        <section className="panel team-invitations">
          <div className="section-heading">
            <div>
              <h2>邀请记录</h2>
              <p>跟踪邀请状态，撤销尚未使用的链接。</p>
            </div>
            <span className="badge">{pendingCount} 条待接受</span>
          </div>
          {loaded && invitations.length === 0 && (
            <div className="team-empty">
              <span className="empty-invite-mark" aria-hidden="true">
                ↗
              </span>
              <strong>还没有发出邀请</strong>
              <p>生成一份邀请链接，开始团队协作。</p>
            </div>
          )}
          <div className="member-list">
            {invitations.map((i) => {
              const state = i.accepted_at
                ? "已接受"
                : i.revoked_at
                  ? "已撤销"
                  : new Date(i.expires_at).getTime() <= now
                    ? "已过期"
                    : "待接受";
              return (
                <article key={i.id} className="member-row invitation-row">
                  <div className="invitation-identity">
                    <span className="invitation-icon" aria-hidden="true">
                      ↗
                    </span>
                    <div>
                      <strong>{roleNames[i.role]}邀请</strong>
                      <small>
                        到期时间 {new Date(i.expires_at).toLocaleString()}
                      </small>
                    </div>
                  </div>
                  <div className="member-actions">
                    <span
                      className={`invitation-status${state === "待接受" ? " is-pending" : ""}`}
                    >
                      {state}
                    </span>
                    {state === "待接受" &&
                      (organization.role === "owner" || i.role !== "admin") && (
                        <button
                          className="secondary"
                          disabled={busy}
                          onClick={() => void act(() => revokeInvitation(i.id))}
                        >
                          撤销邀请
                        </button>
                      )}
                  </div>
                </article>
              );
            })}
          </div>
        </section>
      )}
    </section>
  );
}
