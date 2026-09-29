import { useCallback, useEffect, useRef, useState } from "react";
import { request, errorMessage } from "./api";
import type { Organization } from "./api";
import { t, useLocale, localeTag } from "./i18n";
import "./BillingPage.css";

type BillingState = {
  mode: "self_hosted" | "cloud";
  test_mode: boolean;
  configured: boolean;
  active: boolean;
  has_customer: boolean;
  server_limit: number;
  subscription: {
    status: string;
    interval: string;
    period_end: number;
    cancel_at_period_end: boolean;
  };
};
const statusLabels: Record<string, string> = {
  none: "未开通",
  active: "已开通",
  past_due: "付款逾期",
  unpaid: "未付款",
  canceled: "已取消",
  incomplete: "等待付款",
  incomplete_expired: "付款已过期",
  payment_pending: "等待付款确认",
  trialing: "试用中",
  paused: "已暂停",
  unsupported: "套餐需核查",
};
export default function BillingPage({
  organization,
}: {
  organization: Organization;
}) {
  useLocale();
  const [data, setData] = useState<BillingState | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [interval, setInterval] = useState<"month" | "year">("year");
  const actionController = useRef<AbortController | null>(null);
  useEffect(() => () => actionController.current?.abort(), []);
  const owner = organization.role === "owner";
  const returned = new URLSearchParams(window.location.search).get("checkout");
  const load = useCallback(async (signal?: AbortSignal) => {
    const result = await request<BillingState>(
      "/api/v1/billing",
      "GET",
      undefined,
      signal,
    );
    if (!signal?.aborted) setData(result);
    return result;
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    void request<BillingState>(
      "/api/v1/billing",
      "GET",
      undefined,
      controller.signal,
    )
      .then(async (result) => {
        if (controller.signal.aborted) return;
        setData(result);
        if (returned === "success" && owner && result.configured) {
          await request("/api/v1/billing/sync", "POST", {}, controller.signal);
          await load(controller.signal);
        }
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(errorMessage(e));
      });
    return () => controller.abort();
  }, [load, returned, owner]);
  async function act(action: "checkout" | "portal" | "sync") {
    if (actionController.current && !actionController.current.signal.aborted)
      return;
    const controller = new AbortController();
    actionController.current = controller;
    setBusy(true);
    setError("");
    try {
      const result = await request<{ url: string }>(
        `/api/v1/billing/${action}`,
        "POST",
        action === "checkout" ? { interval } : {},
        controller.signal,
      );
      if (controller.signal.aborted) return;
      if (action !== "sync") {
        const target = new URL(result.url);
        const host =
          action === "checkout" ? "checkout.stripe.com" : "billing.stripe.com";
        if (target.protocol !== "https:" || target.host !== host)
          throw new Error(t("支付服务暂时不可用，请稍后重试"));
        window.location.assign(target.href);
      } else await load(controller.signal);
    } catch (e) {
      if (!controller.signal.aborted) setError(errorMessage(e));
    } finally {
      const aborted = controller.signal.aborted;
      controller.abort();
      if (actionController.current === controller) {
        actionController.current = null;
        if (!aborted) setBusy(false);
      }
    }
  }
  const canPurchase =
    data?.configured &&
    owner &&
    ["none", "canceled", "incomplete_expired"].includes(
      data.subscription.status,
    );
  return (
    <div className="billing-page">
      <div className="page-heading">
        <div>
          <p className="eyebrow">XINGDU CLOUD</p>
          <h1>{t("套餐与账单")}</h1>
          <p className="subtitle">{t("按组织订阅，与你的团队共享。")}</p>
        </div>
      </div>
      {error && (
        <div className="alert" role="alert">
          {error}
          <button
            onClick={() =>
              void load()
                .then(() => setError(""))
                .catch((e) => setError(errorMessage(e)))
            }
          >
            {t("重试")}
          </button>
        </div>
      )}
      {!data && !error && <p role="status">{t("正在加载…")}</p>}
      {data && (
        <>
          {data.test_mode && (
            <p className="alert" role="status">
              {t("当前为 Stripe 测试模式，不会产生真实扣款。")}
            </p>
          )}
          <section className="panel billing-summary">
            <div>
              <span className="eyebrow">{organization.name}</span>
              <h2>
                {data.mode === "self_hosted"
                  ? t("免费自部署版")
                  : t(statusLabels[data.subscription.status] ?? "套餐需核查")}
              </h2>
              {data.mode === "self_hosted" ? (
                <p>
                  {t(
                    "当前实例为自部署模式，无需支付订阅费。云端套餐仅适用于星渡托管服务。",
                  )}
                </p>
              ) : (
                <>
                  <p>{t("免费版可管理 1 台服务器，付费套餐可管理 10 台。")}</p>
                  <p>{t("当前服务器额度：{0} 台", { 0: data.server_limit })}</p>
                  {data.subscription.period_end > 0 && (
                    <p>
                      {t(
                        data.subscription.cancel_at_period_end
                          ? "套餐结束日期：{0}"
                          : "当前账期结束：{0}",
                        {
                          0: new Date(
                            data.subscription.period_end * 1000,
                          ).toLocaleDateString(localeTag()),
                        },
                      )}
                    </p>
                  )}
                  {!owner && <p>{t("仅组织所有者可以购买套餐和管理账单。")}</p>}
                  {returned === "success" && (
                    <p role="status">
                      {t(
                        data.active
                          ? "套餐已开通。"
                          : "付款结果正在确认，请刷新账单状态。",
                      )}
                    </p>
                  )}
                  {returned === "cancelled" && (
                    <p>{t("已返回账单页，你可以重新选择付款方式。")}</p>
                  )}
                </>
              )}
            </div>
            {data.configured && owner && (
              <div className="billing-actions">
                <button
                  className="secondary"
                  disabled={busy}
                  onClick={() => void act("sync")}
                >
                  {t("刷新账单状态")}
                </button>
                {data.has_customer && (
                  <button
                    className="primary"
                    disabled={busy}
                    onClick={() => void act("portal")}
                  >
                    {t("管理账单")}
                  </button>
                )}
              </div>
            )}
          </section>
          <section className="panel billing-plan">
            <div className="billing-plan-info">
              <span className="eyebrow">XINGDU CLOUD</span>
              <h2>{t("一个套餐，管理你的服务器。")}</h2>
              <p>
                {t("包含 10 台服务器，按组织计费。VPS 和网络流量由你自行提供。")}
              </p>
              <ul>
                <li>{t("机器接入与状态监控")}</li>
                <li>{t("协议部署与节点订阅")}</li>
                <li>{t("组织邀请与成员协作")}</li>
              </ul>
            </div>
            <div className="billing-plan-price">
              <div
                className="billing-cadence"
                role="group"
                aria-label={t("付款周期")}
              >
                <button
                  type="button"
                  aria-pressed={interval === "month"}
                  onClick={() => setInterval("month")}
                  disabled={busy}
                >
                  {t("月付")}
                </button>
                <button
                  type="button"
                  aria-pressed={interval === "year"}
                  onClick={() => setInterval("year")}
                  disabled={busy}
                >
                  {t("年付")} <span>{t("省 33%")}</span>
                </button>
              </div>
              <p className="billing-amount">
                ${interval === "year" ? "40" : "5"}
                <span> / {t(interval === "year" ? "年" : "月")}</span>
              </p>
              <p>
                {t(
                  interval === "year"
                    ? "约 $3.33/月，每年支付 $40。"
                    : "每月支付 $5。",
                )}
              </p>
              <button
                className="primary"
                disabled={!canPurchase || busy}
                onClick={() => void act("checkout")}
              >
                {t(
                  data.active
                    ? "当前套餐"
                    : busy
                      ? "正在处理…"
                      : "前往安全支付",
                )}
              </button>
              <small>
                {t("通过 Stripe 支付并自动续费，可在账单管理中取消下次续费。")}
              </small>
            </div>
          </section>
          <p className="billing-note">
            {t(
              "取消续费后可使用至当前账期结束。到期后限制新增资源，已有机器和配置不会自动删除。更换月付或年付周期需等当前订阅结束。",
            )}
          </p>
        </>
      )}
    </div>
  );
}
