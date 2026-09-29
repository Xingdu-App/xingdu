import { TableSkeleton } from "./LoadingSkeleton";
import { useEffect, useState } from "react";
import { request, errorMessage } from "./api";
import { t, useLocale, localeTag } from "./i18n";
import "./OperationsPanel.css";
type Operations = {
  usage: Record<string, { used: number; limit: number }>;
  audit: {
    kind: string;
    resource_id: string;
    actor_id: string;
    event: string;
    created_at: string;
  }[];
};
const events: Record<string, string> = {
  host_deleted: "删除服务器",
  enrollment_issued: "签发接入令牌",
  agent_revoked: "撤销 Agent",
  credential_deleted: "删除 SSH 凭据",
  credential_saved: "保存 SSH 凭据",
  protocol_connection_revealed: "查看节点连接信息",
  subscription_created: "创建订阅",
  subscription_updated: "更新订阅",
  subscription_token_rotated: "轮换订阅令牌",
  subscription_deleted: "删除订阅",
  ownership_transferred: "转移组织所有权",
};
const actions: Record<string, string> = {
  ssh_install: "安装 Agent",
  protocol_deploy: "安装节点",
  protocol_remove: "卸载节点",
  protocol_restart: "重启节点",
};
const states: Record<string, string> = {
  queued: "已排队",
  running: "执行中",
  succeeded: "已完成",
  failed: "失败",
  interrupted: "中断",
  cancelled: "已取消",
  removed: "已卸载",
};
function auditLabel(event: string) {
  if (events[event]) return t(events[event]);
  for (const [prefix, label] of Object.entries(actions)) {
    if (event.startsWith(prefix + "_")) {
      const state = states[event.slice(prefix.length + 1)];
      if (state) return `${t(label)} · ${t(state)}`;
    }
  }
  return event;
}
export default function OperationsPanel({
  organizationID,
}: {
  organizationID: string;
}) {
  useLocale();
  const [result, setResult] = useState<{
    organizationID: string;
    refresh: number;
    data: Operations;
  } | null>(null);
  const [failure, setFailure] = useState<{
    organizationID: string;
    refresh: number;
    message: string;
  } | null>(null);
  const [page, setPage] = useState(0);
  const [refresh, setRefresh] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    request<Operations>(
      "/api/v1/organization/operations",
      "GET",
      undefined,
      controller.signal,
    )
      .then((data) => {
        if (!controller.signal.aborted)
          setResult({ organizationID, refresh, data });
      })
      .catch((error) => {
        if (!controller.signal.aborted)
          setFailure({ organizationID, refresh, message: errorMessage(error) });
      });
    return () => controller.abort();
  }, [organizationID, refresh]);
  const data = result?.organizationID === organizationID ? result.data : null;
  const error =
    failure?.organizationID === organizationID && failure.refresh === refresh
      ? failure.message
      : "";
  const loading =
    !(
      result?.organizationID === organizationID && result.refresh === refresh
    ) &&
    !(
      failure?.organizationID === organizationID && failure.refresh === refresh
    );
  const labels: Record<string, string> = {
    hosts: t("服务器"),
    deployments: t("节点部署"),
    subscriptions: t("订阅"),
    members: t("组织成员"),
  };
  const kinds: Record<string, string> = {
    machine: t("服务器"),
    subscription: t("订阅"),
    organization: t("组织"),
  };
  const count = data?.audit.length ?? 0;
  const pages = Math.max(1, Math.ceil(count / 10));
  const current = Math.min(page, pages - 1);
  return (
    <section className="panel operations-panel">
      <div className="operations-heading">
        <h2>{t("资源配额与审计")}</h2>
        <button
          className="secondary compact"
          disabled={loading}
          onClick={() => setRefresh((v) => v + 1)}
        >
          {t("刷新")}
        </button>
      </div>
      {error && (
        <p role="alert" className="form-error">
          {error}
        </p>
      )}
      {!data && !error && (
        <TableSkeleton
          headers={[t("时间"), t("操作"), t("资源"), t("操作者")]}
        />
      )}
      {data && (
        <>
          <dl className="operations-quotas">
            {Object.entries(data.usage).map(([name, value]) => (
              <div key={name}>
                <dt>{labels[name] ?? name}</dt>
                <dd>
                  <strong>{value.used}</strong>
                  <span>/ {value.limit}</span>
                </dd>
                <progress
                  aria-label={labels[name] ?? name}
                  value={value.used}
                  max={Math.max(1, value.limit)}
                />
              </div>
            ))}
          </dl>
          <div className="operations-audit-heading">
            <h3>{t("操作记录")}</h3>
            <span>{t("最近 {0} 条", { 0: count })}</span>
          </div>
          <p className="form-hint">
            {t(
              "配额由服务管理员设置。审计展示最近 100 条机器、订阅与组织操作，不包含凭据。",
            )}
          </p>
          <div className="operations-table-scroll">
            <table className="operations-table">
              <thead>
                <tr>
                  <th scope="col">{t("时间")}</th>
                  <th scope="col">{t("操作")}</th>
                  <th scope="col">{t("资源")}</th>
                  <th scope="col">{t("操作者")}</th>
                </tr>
              </thead>
              <tbody>
                {data.audit
                  .slice(current * 10, current * 10 + 10)
                  .map((event, index) => (
                    <tr key={`${event.created_at}:${event.event}:${index}`}>
                      <td>
                        <time dateTime={event.created_at}>
                          {new Date(event.created_at).toLocaleString(
                            localeTag(),
                          )}
                        </time>
                      </td>
                      <td>{auditLabel(event.event)}</td>
                      <td>
                        <span>{kinds[event.kind] ?? event.kind}</span>
                        <code title={event.resource_id}>
                          {event.resource_id.slice(0, 8)}
                        </code>
                      </td>
                      <td>
                        <code title={event.actor_id}>
                          {event.actor_id.slice(0, 8) || "—"}
                        </code>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
          {count === 0 ? (
            <p className="operations-empty">{t("暂无审计记录")}</p>
          ) : (
            <nav
              className="operations-pagination"
              aria-label={t("审计记录分页")}
            >
              <span>{t("第 {0} / {1} 页", { 0: current + 1, 1: pages })}</span>
              <div>
                <button
                  className="secondary compact"
                  disabled={current === 0}
                  onClick={() => setPage(current - 1)}
                >
                  {t("上一页")}
                </button>
                <button
                  className="secondary compact"
                  disabled={current + 1 >= pages}
                  onClick={() => setPage(current + 1)}
                >
                  {t("下一页")}
                </button>
              </div>
            </nav>
          )}
        </>
      )}
    </section>
  );
}
