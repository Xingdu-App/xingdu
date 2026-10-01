import { useCallback, useEffect, useRef, useState } from "react";
import {
  request,
  errorMessage,
  type Organization,
  type ManagedNode,
  type Host,
} from "./api";
import { t, useLocale } from "./i18n";
import { TableSkeleton } from "./LoadingSkeleton";
import Select from "./Select";
import "./CertificatesPage.css";

type Certificate = {
  id: string;
  domain: string;
  validation_target: string;
  state: string;
  expires_at: string | null;
  error_code: string;
  directory: string;
  platform: boolean;
  host_id?: string;
};
type Inventory = {
  certificates: Certificate[];
  limit: number;
  configured: boolean;
  test_mode: boolean;
  platform_domains_configured: boolean;
};
const states: Record<string, string> = {
  pending: "等待 DNS 授权",
  queued: "等待签发",
  issuing: "正在签发",
  issued: "已签发",
  failed: "签发失败",
  paused: "已暂停",
};
const errors: Record<string, string> = {
  certificate_quota_exceeded:
    "证书超出当前套餐配额，签发和续期已暂停。请移除多余证书或升级套餐。",
  issuance_interrupted:
    "签发任务多次中断，自动重试已停止。请联系运营者检查后重试。",
  dns_delegation_required: "未检测到正确的 CNAME 委托，请检查 DNS 记录后重试。",
  paid_subscription_required: "付费套餐已失效，自动续期暂停。",
  issuance_failed: "签发失败，系统会有限重试；请检查 DNS 授权与运营配置。",
  invalid_certificate: "返回的证书校验失败，未保存。",
};
export default function CertificatesPage({
  organization,
  nodes,
  hosts,
  onBilling,
}: {
  organization: Organization;
  nodes: ManagedNode[];
  hosts: Host[];
  onBilling: () => void;
}) {
  useLocale();
  const [now, setNow] = useState(() => Date.now());
  const [data, setData] = useState<Inventory | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [platform, setPlatform] = useState(false);
  const [hostID, setHostID] = useState("");
  const [domain, setDomain] = useState("");
  const [creating, setCreating] = useState(false);
  const createDialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (creating && !createDialog.current?.open)
      createDialog.current?.showModal();
    else if (!creating) createDialog.current?.close();
  }, [creating]);
  const [selected, setSelected] = useState<Certificate | null>(null);
  const manageDialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (selected && !manageDialog.current?.open)
      manageDialog.current?.showModal();
    else if (!selected) manageDialog.current?.close();
  }, [selected]);
  const [nodeID, setNodeID] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const canManage = ["owner", "admin"].includes(organization.role);
  const load = useCallback(async (signal?: AbortSignal) => {
    const out = await request<Inventory>(
      "/api/v1/certificates",
      "GET",
      undefined,
      signal,
    );
    if (!signal?.aborted) {
      setData(out);
      setNow(Date.now());
    }
  }, []);
  useEffect(() => {
    const c = new AbortController();
    void request<Inventory>("/api/v1/certificates", "GET", undefined, c.signal)
      .then((out) => {
        if (!c.signal.aborted) {
          setData(out);
          setNow(Date.now());
        }
      })
      .catch((e) => {
        if (!c.signal.aborted) setError(errorMessage(e));
      });
    const timer = window.setInterval(() => {
      void load(c.signal).catch(() => {});
    }, 15000);
    return () => {
      c.abort();
      window.clearInterval(timer);
    };
  }, [load]);
  const act = async (action: () => Promise<unknown>) => {
    setBusy(true);
    setError("");
    try {
      await action();
      await load();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  const current =
    data?.certificates.find((c) => c.id === selected?.id) ?? selected;
  const choose = (c: Certificate) => {
    setError("");
    setSelected(c);
    setNodeID("");
    setConfirmed(false);
    setDeleting(false);
    setNotice("");
  };
  const paid = (data?.limit ?? 0) > 0;
  const candidates = nodes.filter(
    (n) =>
      [
        "trojan",
        "vless",
        "vmess",
        "hysteria",
        "hysteria2",
        "tuic",
        "anytls",
        "http",
      ].includes(n.protocol) &&
      (!current?.platform || n.host_id === current.host_id) &&
      n.state !== "removed" &&
      !n.relay_exit_id,
  );
  return (
    <section className="panel certificates-page">
      <div className="section-heading">
        <div>
          <h2>{t("证书管理")}</h2>
          <p className="subtitle">
            {t(
              "通过 DNS 委托自动签发和续期；签发完成后，将证书应用到对应节点。",
            )}
          </p>
        </div>
        {canManage && (
          <button
            className="primary"
            disabled={
              !paid ||
              !data?.configured ||
              busy ||
              (data?.certificates.length ?? 0) >= (data?.limit ?? 0)
            }
            onClick={() => {
              setError("");
              setPlatform(false);
              setDomain("");
              setHostID("");
              setCreating(true);
              setSelected(null);
            }}
          >
            {t("添加域名")}
          </button>
        )}
      </div>
      {error && !creating && !current && (
        <div role="alert" className="alert">
          {error}
          <button
            className="secondary"
            disabled={busy}
            onClick={() => void act(() => load())}
          >
            {t("重试")}
          </button>
        </div>
      )}
      {notice && !current && (
        <p className="notice" role="status">
          {t(notice)}
        </p>
      )}
      {data && (
        <p className="subtitle">
          {t("已使用 {0} / {1} 张证书", {
            0: data.certificates.length,
            1: data.limit,
          })}
        </p>
      )}
      {data && !paid && (
        <div className="certificate-callout">
          <p>
            {t(
              "自动证书仅供有效付费套餐使用：Starter 10 张，Premium 50 张。手动上传自己的证书仍可使用。",
            )}
          </p>
          {organization.role === "owner" && (
            <button className="secondary" onClick={onBilling}>
              {t("查看套餐")}
            </button>
          )}
        </div>
      )}
      {data && !data.configured && (
        <p className="certificate-callout">
          {t(
            "运营者尚未配置自动证书服务，暂时无法添加域名或签发。已有证书仍可查看。",
          )}
        </p>
      )}
      {data?.configured && data.test_mode && (
        <p className="certificate-callout">
          {t("当前连接测试 CA，签发的证书不受客户端信任，不能应用到节点。")}
        </p>
      )}
      <dialog
        ref={createDialog}
        className="host-dialog machine-dialog certificate-create-dialog"
        aria-labelledby="certificate-create-title"
        onCancel={(e) => {
          e.preventDefault();
          if (!busy) setCreating(false);
        }}
        onClose={() => setCreating(false)}
      >
        <div className="dialog-heading">
          <h2 id="certificate-create-title">{t("添加域名")}</h2>
          <button
            type="button"
            className="icon-button"
            aria-label={t("关闭")}
            disabled={busy}
            onClick={() => setCreating(false)}
          >
            ×
          </button>
        </div>
        <form
          id="certificate-create-form"
          className="machine-content certificate-create"
          onSubmit={(e) => {
            e.preventDefault();
            if (platform && !hostID) return;
            void act(async () => {
              const c = await request<Certificate>(
                "/api/v1/certificates",
                "POST",
                platform
                  ? { platform: true, host_id: hostID }
                  : { domain: domain.trim().toLowerCase() },
              );
              setCreating(false);
              setDomain("");
              choose(c);
            });
          }}
        >
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <label>
            {t("域名来源")}
            <Select
              label={t("域名来源")}
              value={platform ? "platform" : "custom"}
              disabled={busy}
              onChange={(value) => setPlatform(value === "platform")}
              options={[
                { value: "custom", label: t("自有域名") },
                ...(data?.platform_domains_configured
                  ? [{ value: "platform", label: t("平台随机域名") }]
                  : []),
              ]}
            />
          </label>
          {platform ? (
            <label>
              {t("绑定服务器")}
              <Select
                label={t("绑定服务器")}
                value={hostID}
                disabled={busy}
                onChange={setHostID}
                options={[
                  { value: "", label: t("选择服务器") },
                  ...hosts.map((h) => ({ value: h.id, label: h.name })),
                ]}
              />
            </label>
          ) : (
            <label>
              {t("自有域名")}
              <input
                required
                type="text"
                maxLength={253}
                value={domain}
                placeholder="node.example.com"
                disabled={busy}
                onChange={(e) => setDomain(e.target.value)}
              />
            </label>
          )}
          <p className="subtitle">
            {t(
              platform
                ? "平台为此服务器分配独立随机域名，服务器地址须为公网 IP。"
                : "使用单个完整域名，暂不接受泛域名。",
            )}
          </p>
        </form>
        <div className="dialog-footer">
          <button
            type="button"
            className="secondary"
            disabled={busy}
            onClick={() => setCreating(false)}
          >
            {t("取消")}
          </button>
          <button
            type="submit"
            form="certificate-create-form"
            className="primary"
            disabled={busy || (platform && !hostID)}
          >
            {busy ? t("正在保存…") : t("添加")}
          </button>
        </div>
      </dialog>
      {!data ? (
        <TableSkeleton
          headers={[t("域名"), t("状态"), t("到期时间"), t("操作")]}
        />
      ) : data.certificates.length === 0 ? (
        <p className="empty-state">
          {t("暂无托管证书。节点中手动上传的证书仍在节点配置中管理。")}
        </p>
      ) : (
        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>{t("域名")}</th>
                <th>{t("状态")}</th>
                <th>{t("到期时间")}</th>
                <th>{t("操作")}</th>
              </tr>
            </thead>
            <tbody>
              {data.certificates.map((c) => (
                <tr key={c.id}>
                  <td>
                    <strong>{c.domain}</strong>
                    {c.directory && c.directory.includes("staging") && (
                      <small>{t("测试证书")}</small>
                    )}
                  </td>
                  <td>
                    {t(states[c.state] ?? c.state)}
                    {c.expires_at && Date.parse(c.expires_at) <= now && (
                      <small>{t("已过期")}</small>
                    )}
                  </td>
                  <td>
                    {c.expires_at
                      ? new Date(c.expires_at).toLocaleDateString()
                      : "—"}
                  </td>
                  <td>
                    <button className="secondary" onClick={() => choose(c)}>
                      {t("管理")}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {current && (
        <dialog
          ref={manageDialog}
          className="host-dialog machine-dialog certificate-manage-dialog"
          aria-labelledby="certificate-manage-title"
          onCancel={(e) => {
            e.preventDefault();
            if (!busy) setSelected(null);
          }}
          onClose={() => setSelected(null)}
        >
          <div className="dialog-heading">
            <h2 id="certificate-manage-title">{t("管理证书")}</h2>
            <button
              type="button"
              className="icon-button"
              aria-label={t("关闭")}
              disabled={busy}
              onClick={() => setSelected(null)}
            >
              ×
            </button>
          </div>
          <div className="machine-content certificate-detail">
            <h3 className="certificate-domain">{current.domain}</h3>
            {error && (
              <div role="alert" className="alert">
                {error}
              </div>
            )}
            {notice && (
              <p role="status" className="notice">
                {t(notice)}
              </p>
            )}
            {current.platform ? (
              <p className="subtitle">
                {t(
                  "平台负责此域名的 DNS，无需添加 CNAME。域名绑定原服务器，不会分配给其他用户。",
                )}
              </p>
            ) : (
              <>
                {" "}
                <p className="subtitle">
                  {t(
                    "在域名的 DNS 中添加以下 CNAME 记录，关闭代理，保留此记录供自动续期使用。无需提交 DNS API 密钥。",
                  )}
                </p>
                <dl>
                  <dt>{t("记录名称")}</dt>
                  <dd>
                    <code>_acme-challenge.{current.domain}</code>
                    <button
                      className="secondary"
                      onClick={() =>
                        void act(() =>
                          navigator.clipboard.writeText(
                            `_acme-challenge.${current.domain}`,
                          ),
                        )
                      }
                    >
                      {t("复制")}
                    </button>
                  </dd>
                  <dt>{t("目标")}</dt>
                  <dd>
                    <code>{current.validation_target}</code>
                    <button
                      className="secondary"
                      onClick={() =>
                        void act(() =>
                          navigator.clipboard.writeText(
                            current.validation_target,
                          ),
                        )
                      }
                    >
                      {t("复制")}
                    </button>
                  </dd>
                </dl>
              </>
            )}
            {current.error_code && (
              <p role="status" className="certificate-callout">
                {t(
                  errors[current.error_code] ??
                    "证书服务暂时不可用，请联系运营者。",
                )}
              </p>
            )}
            {canManage && (
              <>
                <div className="certificate-actions">
                  <button
                    className="primary"
                    disabled={
                      busy ||
                      !paid ||
                      !data?.configured ||
                      ["issuing", "queued"].includes(current.state) ||
                      (current.state === "issued" &&
                        !!current.expires_at &&
                        Date.parse(current.expires_at) >
                          now + 30 * 24 * 3600 * 1000 &&
                        current.directory.includes("staging") ===
                          data?.test_mode)
                    }
                    onClick={() =>
                      void act(async () => {
                        await request(
                          `/api/v1/certificates/${current.id}/issue`,
                          "POST",
                          {},
                        );
                        setNotice(
                          "签发任务已排队，请等待 DNS 验证与 CA 签发。",
                        );
                      })
                    }
                  >
                    {t(current.expires_at ? "申请续期" : "验证并签发")}
                  </button>
                  <button
                    className="secondary"
                    disabled={busy}
                    onClick={() => {
                      setDeleting((v) => !v);
                      setConfirmed(false);
                    }}
                  >
                    {t("移除管理")}
                  </button>
                </div>
                {current.expires_at &&
                  !current.directory.includes("staging") &&
                  Date.parse(current.expires_at) > now && (
                    <div className="certificate-apply">
                      <label>
                        {t("应用到节点")}
                        <Select
                          label={t("应用到节点")}
                          value={nodeID}
                          disabled={busy || !paid}
                          onChange={(value) => {
                            setNodeID(value);
                            setConfirmed(false);
                          }}
                          options={[
                            { value: "", label: t("选择 TLS 节点") },
                            ...candidates.map((n) => ({
                              value: n.id,
                              label: n.name,
                            })),
                          ]}
                        />
                      </label>
                      <p className="subtitle">
                        {t(
                          "应用时会将节点 TLS 域名更新为此证书域名。中转依赖需协调维护。续期不会自动重启节点，请在签发后应用并刷新客户端订阅。",
                        )}
                      </p>
                      <label className="certificate-checkbox">
                        <input
                          type="checkbox"
                          checked={confirmed}
                          disabled={busy || !paid}
                          onChange={(e) => setConfirmed(e.target.checked)}
                        />
                        <span>{t("确认更新，连接会短暂中断")}</span>
                      </label>
                      <button
                        className="primary"
                        disabled={busy || !paid || !nodeID || !confirmed}
                        onClick={() =>
                          void act(async () => {
                            const n = nodes.find((n) => n.id === nodeID);
                            if (!n) return;
                            await request(
                              `/api/v1/certificates/${current.id}/apply`,
                              "POST",
                              {
                                node_id: n.id,
                                host_id: n.host_id,
                                confirm: true,
                              },
                            );
                            setConfirmed(false);
                            setNotice(
                              "节点更新已排队，请在部署记录中确认结果；签发成功不代表节点已应用。",
                            );
                          })
                        }
                      >
                        {t("应用证书")}
                      </button>
                    </div>
                  )}
                {deleting && (
                  <div className="certificate-callout">
                    <p>
                      {t(
                        "移除后停止自动续期并删除托管私钥，不撤销 CA 证书，也不会停止节点上的服务。",
                      )}
                    </p>
                    <button
                      className="danger"
                      disabled={busy}
                      onClick={() =>
                        void act(async () => {
                          await request(
                            `/api/v1/certificates/${current.id}`,
                            "DELETE",
                            {},
                          );
                          setSelected(null);
                        })
                      }
                    >
                      {t("确认移除")}
                    </button>
                    <button
                      className="secondary"
                      disabled={busy}
                      onClick={() => setDeleting(false)}
                    >
                      {t("取消")}
                    </button>
                  </div>
                )}
              </>
            )}
          </div>
          <div className="dialog-footer">
            <button
              type="button"
              className="secondary"
              disabled={busy}
              onClick={() => setSelected(null)}
            >
              {t("关闭")}
            </button>
          </div>
        </dialog>
      )}
    </section>
  );
}
