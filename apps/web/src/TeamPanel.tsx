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
  return (
    <section className="team-panel">
      <p className="eyebrow">ORGANIZATION</p>
      <h1>{organization.name}</h1>
      <p>
        当前身份：{roleNames[organization.role]}。服务器和团队权限属于当前组织。
      </p>
      {error && (
        <p role="alert" className="form-error">
          {error}
          <button className="secondary" onClick={() => void act(refresh)}>
            重试
          </button>
        </p>
      )}
      <h2>组织成员</h2>
      {!loaded && !error && <p role="status">正在加载成员…</p>}
      <div className="member-list">
        {members.map((m) => (
          <article key={m.id} className="member-row">
            <div>
              <strong>{m.username}</strong>
              <small>{roleNames[m.role]}</small>
            </div>
            {manage &&
            m.role !== "owner" &&
            (organization.role === "owner" || m.role !== "admin") ? (
              <div className="member-actions">
                <select
                  aria-label={`${m.username} 的角色`}
                  value={m.role}
                  disabled={busy}
                  onChange={(e) =>
                    void act(() => changeMember(m.id, e.target.value as Role))
                  }
                >
                  {roles.map((r) => (
                    <option key={r} value={r}>
                      {roleNames[r]}
                    </option>
                  ))}
                </select>
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => setRemoving(m)}
                >
                  移除
                </button>
              </div>
            ) : (
              <span>{m.role === "owner" ? "组织所有者" : ""}</span>
            )}
          </article>
        ))}
      </div>
      {removing && (
        <div className="team-banner" role="alert">
          <p>移除 {removing.username} 后，对方将立即失去此组织的访问权限。</p>
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
      )}
      {manage && (
        <>
          <h2>邀请新成员</h2>
          <p>
            邀请链接有效期 7
            天，仅能使用一次。任何持有链接的人都可以接受，请私下分享给目标成员。
          </p>
          <div className="member-actions">
            <select
              aria-label="邀请角色"
              value={role}
              disabled={busy}
              onChange={(e) => setRole(e.target.value as Role)}
            >
              {roles.map((r) => (
                <option key={r} value={r}>
                  {roleNames[r]}
                </option>
              ))}
            </select>
            <button
              className="primary"
              disabled={busy}
              onClick={() =>
                void act(async () => {
                  const value = await createInvitation(role);
                  setLink(value.url);
                })
              }
            >
              生成邀请链接
            </button>
          </div>
          {link && (
            <div className="invite-result">
              <label htmlFor="invite-link">邀请链接（仅本次显示）</label>
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
          <h2>最近的邀请</h2>
          {loaded && invitations.length === 0 && <p>暂无邀请。</p>}
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
                <article key={i.id} className="member-row">
                  <div>
                    <strong>
                      {roleNames[i.role]} · {state}
                    </strong>
                    <small>
                      到期时间 {new Date(i.expires_at).toLocaleString()}
                    </small>
                  </div>
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
                </article>
              );
            })}
          </div>
        </>
      )}
    </section>
  );
}
