package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
	"xingdu.app/xingdu/internal/billing"
	"xingdu.app/xingdu/internal/storage"
)

const stripeWebhookPath = "/api/v1/billing/stripe/webhook"

type BillingStore interface {
	Billing(context.Context) (billing.Record, error)
	EnsureBilling(context.Context) error
	MutateBilling(context.Context, string, string, func(*billing.Record) error) error
}

func (a *api) billingError(w http.ResponseWriter, err error) {
	if errors.Is(err, billing.ErrConflict) {
		failure(w, 409, "billing_conflict", "已有订阅或付款正在处理中，请刷新账单状态后重试")
		return
	}
	if errors.Is(err, billing.ErrUnavailable) {
		failure(w, 503, "billing_unavailable", "支付服务暂时不可用，请稍后重试")
		return
	}
	storeError(w, err)
}
func (a *api) billingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/billing", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		record, err := a.store.Billing(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		mode := "self_hosted"
		if a.billingCloud {
			mode = "cloud"
		}
		reply(w, 200, map[string]any{"data": map[string]any{"mode": mode, "test_mode": a.billingCloud && a.billingTest, "configured": a.billingCloud && a.billing != nil, "premium_available": a.billingCloud && a.billing != nil && a.billingPremium, "subscription": record, "active": record.Entitled(time.Now()), "has_customer": record.CustomerID != "", "server_limit": record.ServerLimit(time.Now())}})
	}))
	for _, action := range []string{"checkout", "portal", "sync"} {
		mux.HandleFunc("POST /api/v1/billing/"+action, a.tenant(func(w http.ResponseWriter, r *http.Request, user storage.User, _ string) {
			if !a.attempts.allowLimit("billing:"+user.ID, 30) {
				failure(w, 429, "rate_limited", "请稍后重试")
				return
			}
			if !a.billingCloud || a.billing == nil {
				failure(w, 503, "billing_disabled", "当前实例使用免费自部署模式，无需购买云端套餐")
				return
			}
			var in struct {
				Interval string `json:"interval"`
				Plan     string `json:"plan"`
			}
			if !decode(w, r, &in) {
				return
			}
			if action == "checkout" && in.Interval != "month" && in.Interval != "year" {
				failure(w, 422, "invalid_interval", "请选择月付或年付")
				return
			}
			if in.Plan == "" {
				in.Plan = "start"
			}
			if action == "checkout" && (in.Plan != "start" && in.Plan != "premium" || in.Plan == "premium" && !a.billingPremium) {
				failure(w, 422, "invalid_plan", "此套餐暂不可购买")
				return
			}
			if err := a.store.EnsureBilling(r.Context()); err != nil {
				storeError(w, err)
				return
			}
			if action == "checkout" {
				// Commit the customer binding before Checkout can emit a webhook.
				err := a.store.MutateBilling(r.Context(), "", "", func(record *billing.Record) error {
					if record.CustomerID != "" {
						return nil
					}
					var err error
					record.CustomerID, err = a.billing.Customer(r.Context(), record.OrganizationID)
					return err
				})
				if err != nil {
					a.billingError(w, err)
					return
				}
			}
			var redirect string
			err := a.store.MutateBilling(r.Context(), "", "", func(record *billing.Record) error {
				var err error
				switch action {
				case "checkout":
					if record.CustomerID == "" {
						record.CustomerID, err = a.billing.Customer(r.Context(), record.OrganizationID)
						if err != nil {
							return err
						}
					}
					redirect, err = a.billing.Checkout(r.Context(), record, in.Plan, in.Interval, a.origin)
				case "portal":
					if record.CustomerID == "" {
						return billing.ErrConflict
					}
					redirect, err = a.billing.Portal(r.Context(), record.CustomerID, a.origin+"/app/billing?organization="+record.OrganizationID)
				case "sync":
					err = a.billing.Sync(r.Context(), record)
				}
				return err
			})
			if err != nil {
				a.billingError(w, err)
				return
			}
			reply(w, 200, map[string]any{"data": map[string]string{"url": redirect}})
		}))
	}
	mux.HandleFunc("POST "+stripeWebhookPath, func(w http.ResponseWriter, r *http.Request) {
		if !a.billingCloud || a.billing == nil {
			failure(w, 503, "billing_disabled", "支付回调尚未配置")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			failure(w, 400, "invalid_webhook", "无效的支付回调")
			return
		}
		event, customer, err := a.billing.Event(body, r.Header.Get("Stripe-Signature"))
		if err != nil {
			failure(w, 400, "invalid_webhook", "无效的支付回调")
			return
		}
		if customer != "" {
			err = a.store.MutateBilling(r.Context(), customer, event, func(record *billing.Record) error { return a.billing.Sync(r.Context(), record) })
		}
		if err != nil {
			failure(w, 503, "billing_sync_failed", "订阅同步暂时失败")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
