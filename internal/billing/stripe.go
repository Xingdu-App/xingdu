// Package billing implements Xingdu's organization-scoped Cloud plans.
package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const APIVersion = "2025-03-31.basil"

var ErrUnavailable = errors.New("billing unavailable")
var ErrConflict = errors.New("billing operation conflicts with current subscription")
var ErrSignature = errors.New("invalid Stripe signature")

type Config struct{ Mode, SecretKey, WebhookSecret, MonthlyPrice, YearlyPrice, PremiumMonthlyPrice, PremiumYearlyPrice, PortalConfiguration string }

func (c Config) Validate() error {
	if c.Mode == "" || c.Mode == "self_hosted" {
		return nil
	}
	if c.Mode != "cloud" {
		return errors.New("XINGDU_BILLING_MODE must be self_hosted or cloud")
	}
	if !(strings.HasPrefix(c.SecretKey, "sk_test_") || strings.HasPrefix(c.SecretKey, "sk_live_")) || !strings.HasPrefix(c.WebhookSecret, "whsec_") || !strings.HasPrefix(c.MonthlyPrice, "price_") || !strings.HasPrefix(c.YearlyPrice, "price_") || c.MonthlyPrice == c.YearlyPrice || !strings.HasPrefix(c.PortalConfiguration, "bpc_") {
		return errors.New("cloud billing requires Stripe secret, webhook secret, two distinct price IDs and a portal configuration")
	}
	if c.PremiumMonthlyPrice != "" || c.PremiumYearlyPrice != "" {
		seen := map[string]bool{c.MonthlyPrice: true, c.YearlyPrice: true}
		for _, price := range []string{c.PremiumMonthlyPrice, c.PremiumYearlyPrice} {
			if !strings.HasPrefix(price, "price_") || seen[price] {
				return errors.New("Premium requires two distinct Stripe price IDs")
			}
			seen[price] = true
		}
	}
	return nil
}
func (c Config) Cloud() bool { return c.Mode == "cloud" }
func (c Config) Live() bool  { return strings.HasPrefix(c.SecretKey, "sk_live_") }

type Record struct {
	Plan              string `json:"plan"`
	CheckoutPlan      string `json:"-"`
	OrganizationID    string `json:"-"`
	CustomerID        string `json:"-"`
	Attempt           string `json:"-"`
	CheckoutID        string `json:"-"`
	CheckoutInterval  string `json:"-"`
	SubscriptionID    string `json:"-"`
	Status            string `json:"status"`
	Interval          string `json:"interval"`
	PeriodEnd         int64  `json:"period_end"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
}

func (r Record) Entitled(now time.Time) bool { return r.Status == "active" && r.PeriodEnd > now.Unix() }

// CertificateLimit excludes trials and expired subscriptions, like host quotas.
func (r Record) CertificateLimit(now time.Time) int {
	if !r.Entitled(now) {
		return 0
	}
	if r.Plan == "premium" {
		return 50
	}
	return 10
}

func (r Record) ServerLimit(now time.Time) int {
	if r.Entitled(now) {
		if r.Plan == "premium" {
			return 50
		}
		return 10
	}
	return 1
}

type Gateway interface {
	Customer(context.Context, string) (string, error)
	Checkout(context.Context, *Record, string, string, string) (string, error)
	Portal(context.Context, string, string) (string, error)
	Sync(context.Context, *Record) error
	Event([]byte, string) (string, string, error)
}
type Stripe struct {
	Config Config
	HTTP   *http.Client
	base   string
}

func New(c Config) *Stripe {
	return &Stripe{Config: c, HTTP: &http.Client{Timeout: 8 * time.Second}, base: "https://api.stripe.com/v1"}
}
func (s *Stripe) call(ctx context.Context, method, path string, values url.Values, key string, out any) error {
	var body io.Reader
	if method == "GET" {
		if len(values) > 0 {
			path += "?" + values.Encode()
		}
	} else {
		body = strings.NewReader(values.Encode())
	}
	r, err := http.NewRequestWithContext(ctx, method, s.base+path, body)
	if err != nil {
		return ErrUnavailable
	}
	r.SetBasicAuth(s.Config.SecretKey, "")
	r.Header.Set("Stripe-Version", APIVersion)
	if method != "GET" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if key != "" {
		sum := sha256.Sum256([]byte(key))
		r.Header.Set("Idempotency-Key", hex.EncodeToString(sum[:]))
	}
	res, err := s.HTTP.Do(r)
	if err != nil {
		return ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ErrUnavailable
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(out); err != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Stripe) Customer(ctx context.Context, org string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	err := s.call(ctx, "POST", "/customers", url.Values{"metadata[xingdu_organization]": {org}}, "xingdu/customer/"+org, &out)
	if err != nil || !strings.HasPrefix(out.ID, "cus_") {
		return "", ErrUnavailable
	}
	return out.ID, nil
}
func (s *Stripe) price(ctx context.Context, plan, interval string) (string, error) {
	id := s.Config.MonthlyPrice
	amount := int64(500)
	if plan == "premium" {
		if interval == "month" {
			id, amount = s.Config.PremiumMonthlyPrice, 2000
		} else if interval == "year" {
			id, amount = s.Config.PremiumYearlyPrice, 20000
		} else {
			return "", ErrConflict
		}
		if id == "" {
			return "", ErrUnavailable
		}
	} else if plan != "start" {
		return "", ErrConflict
	} else if interval == "year" {
		id = s.Config.YearlyPrice
		amount = 4000
	} else if interval != "month" {
		return "", ErrConflict
	}
	var p struct {
		Active     bool
		Currency   string
		UnitAmount int64 `json:"unit_amount"`
		Livemode   bool
		Recurring  struct {
			Interval      string
			IntervalCount int    `json:"interval_count"`
			UsageType     string `json:"usage_type"`
		}
	}
	if err := s.call(ctx, "GET", "/prices/"+url.PathEscape(id), nil, "", &p); err != nil {
		return "", err
	}
	if !p.Active || p.Livemode != s.Config.Live() || p.Currency != "usd" || p.UnitAmount != amount || p.Recurring.Interval != interval || p.Recurring.IntervalCount != 1 || p.Recurring.UsageType != "licensed" {
		return "", ErrUnavailable
	}
	return id, nil
}
func stripeURL(raw, host string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == host && u.User == nil
}
func (s *Stripe) Checkout(ctx context.Context, r *Record, plan, interval, origin string) (string, error) {
	if err := s.Sync(ctx, r); err != nil {
		return "", err
	}
	if r.Status != "none" && r.Status != "canceled" && r.Status != "incomplete_expired" {
		return "", ErrConflict
	}
	if r.CheckoutID != "" {
		var session struct{ Status, URL string }
		if err := s.call(ctx, "GET", "/checkout/sessions/"+url.PathEscape(r.CheckoutID), nil, "", &session); err != nil {
			return "", err
		}
		if session.Status == "open" {
			if r.CheckoutInterval == interval && (r.CheckoutPlan == plan || r.CheckoutPlan == "" && plan == "start") && stripeURL(session.URL, "checkout.stripe.com") {
				return session.URL, nil
			}
			// Expire the previous checkout before permitting another cadence. A completed
			// session cannot be expired, so a concurrent payment fails closed here.
			var expired struct{ Status string }
			if err := s.call(ctx, "POST", "/checkout/sessions/"+url.PathEscape(r.CheckoutID)+"/expire", url.Values{}, "", &expired); err != nil || expired.Status != "expired" {
				return "", ErrConflict
			}
		} else if session.Status != "expired" && !(session.Status == "complete" && r.SubscriptionID != "" && (r.Status == "canceled" || r.Status == "incomplete_expired")) {
			return "", ErrConflict
		}
	}
	price, err := s.price(ctx, plan, interval)
	if err != nil {
		return "", err
	}
	values := url.Values{"mode": {"subscription"}, "customer": {r.CustomerID}, "line_items[0][price]": {price}, "line_items[0][quantity]": {"1"}, "payment_method_types[0]": {"card"}, "client_reference_id": {r.OrganizationID}, "metadata[xingdu_organization]": {r.OrganizationID}, "subscription_data[metadata][xingdu_organization]": {r.OrganizationID}, "success_url": {origin + "/app/billing?checkout=success&organization=" + r.OrganizationID}, "cancel_url": {origin + "/app/billing?checkout=cancelled&organization=" + r.OrganizationID}}
	var out struct{ ID, URL string }
	key := "xingdu/checkout/" + r.Attempt + "/" + r.CheckoutID + "/" + plan + "/" + interval
	if plan == "start" {
		// Preserve retry identity for existing checkout attempts across upgrades.
		key = "xingdu/checkout/" + r.Attempt + "/" + r.CheckoutID + "/" + interval
	}
	if err = s.call(ctx, "POST", "/checkout/sessions", values, key, &out); err != nil {
		return "", err
	}
	if !strings.HasPrefix(out.ID, "cs_") || !stripeURL(out.URL, "checkout.stripe.com") {
		return "", ErrUnavailable
	}
	r.CheckoutID = out.ID
	r.CheckoutInterval = interval
	r.CheckoutPlan = plan
	return out.URL, nil
}
func (s *Stripe) Portal(ctx context.Context, customer, origin string) (string, error) {
	// Reject configurations that could introduce uncontrolled prices, quantities,
	// prorations or immediate cancellation outside this plan's contract.
	var cfg struct {
		Active   bool
		Features struct {
			SubscriptionUpdate struct{ Enabled bool } `json:"subscription_update"`
			SubscriptionCancel struct {
				Enabled bool
				Mode    string
			} `json:"subscription_cancel"`
			PaymentMethodUpdate struct{ Enabled bool } `json:"payment_method_update"`
			InvoiceHistory      struct{ Enabled bool } `json:"invoice_history"`
		}
	}
	if err := s.call(ctx, "GET", "/billing_portal/configurations/"+url.PathEscape(s.Config.PortalConfiguration), nil, "", &cfg); err != nil {
		return "", err
	}
	if !cfg.Active || cfg.Features.SubscriptionUpdate.Enabled || !cfg.Features.SubscriptionCancel.Enabled || cfg.Features.SubscriptionCancel.Mode != "at_period_end" || !cfg.Features.PaymentMethodUpdate.Enabled || !cfg.Features.InvoiceHistory.Enabled {
		return "", ErrUnavailable
	}
	var out struct{ URL string }
	err := s.call(ctx, "POST", "/billing_portal/sessions", url.Values{"customer": {customer}, "configuration": {s.Config.PortalConfiguration}, "return_url": {origin}}, "", &out)
	if err != nil || !stripeURL(out.URL, "billing.stripe.com") {
		return "", ErrUnavailable
	}
	return out.URL, nil
}

type stripeSubscription struct {
	ID, Status        string
	Customer          string
	Livemode          bool
	CancelAtPeriodEnd bool                    `json:"cancel_at_period_end"`
	PauseCollection   json.RawMessage         `json:"pause_collection"`
	LatestInvoice     struct{ Status string } `json:"latest_invoice"`
	Items             struct {
		Data []struct {
			Quantity         int
			CurrentPeriodEnd int64 `json:"current_period_end"`
			Price            struct {
				ID, Currency string
				UnitAmount   int64 `json:"unit_amount"`
				Recurring    struct {
					Interval      string
					IntervalCount int    `json:"interval_count"`
					UsageType     string `json:"usage_type"`
				}
			}
		}
	}
}

func (s *Stripe) Sync(ctx context.Context, r *Record) error {
	if r.CustomerID == "" {
		return nil
	}
	var list struct {
		Data    []stripeSubscription
		HasMore bool `json:"has_more"`
	}
	if err := s.call(ctx, "GET", "/subscriptions", url.Values{"customer": {r.CustomerID}, "status": {"all"}, "limit": {"100"}, "expand[]": {"data.latest_invoice"}}, "", &list); err != nil {
		return err
	}
	if list.HasMore {
		return ErrUnavailable
	}
	var current *stripeSubscription
	for i := range list.Data {
		sub := &list.Data[i]
		if sub.Status == "canceled" || sub.Status == "incomplete_expired" {
			continue
		}
		if current != nil {
			return ErrConflict
		}
		current = sub
	}
	if current == nil {
		if r.SubscriptionID != "" {
			r.Status = "canceled"
		} else {
			r.Status = "none"
		}
		r.PeriodEnd = 0
		r.CancelAtPeriodEnd = false
		return nil
	}
	sub := current
	if sub.Customer != r.CustomerID || sub.Livemode != s.Config.Live() {
		return ErrUnavailable
	}
	r.SubscriptionID = sub.ID
	r.Status = sub.Status
	r.PeriodEnd = 0
	r.CancelAtPeriodEnd = sub.CancelAtPeriodEnd
	r.Interval = ""
	r.Plan = "start"
	if len(sub.Items.Data) != 1 {
		r.Status = "unsupported"
		return nil
	}
	item := sub.Items.Data[0]
	price := item.Price
	interval := "month"
	amount := int64(500)
	if s.Config.PremiumMonthlyPrice != "" && price.ID == s.Config.PremiumMonthlyPrice {
		r.Plan = "premium"
		amount = 2000
	} else if s.Config.PremiumYearlyPrice != "" && price.ID == s.Config.PremiumYearlyPrice {
		r.Plan = "premium"
		interval, amount = "year", 20000
	} else if price.ID == s.Config.YearlyPrice {
		interval = "year"
		amount = 4000
	} else if price.ID != s.Config.MonthlyPrice {
		r.Status = "unsupported"
		return nil
	}
	if item.Quantity != 1 || price.Currency != "usd" || price.UnitAmount != amount || price.Recurring.Interval != interval || price.Recurring.IntervalCount != 1 || price.Recurring.UsageType != "licensed" {
		r.Status = "unsupported"
		return nil
	}
	r.Interval = interval
	r.PeriodEnd = item.CurrentPeriodEnd
	if r.Status == "active" && (sub.LatestInvoice.Status != "paid" || (len(sub.PauseCollection) > 0 && string(sub.PauseCollection) != "null")) {
		r.Status = "payment_pending"
	}
	return nil
}

// Signature verification follows Stripe's raw-body HMAC protocol. Both old and
// future timestamps are bounded, and any v1 signature may match during rotation.
func (s *Stripe) Event(body []byte, signature string) (string, string, error) {
	if s.Config.WebhookSecret == "" {
		return "", "", ErrSignature
	}
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(signature, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok {
			if k == "t" {
				timestamp = v
			}
			if k == "v1" {
				signatures = append(signatures, v)
			}
		}
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || seconds < time.Now().Add(-5*time.Minute).Unix() || seconds > time.Now().Add(5*time.Minute).Unix() {
		return "", "", ErrSignature
	}
	mac := hmac.New(sha256.New, []byte(s.Config.WebhookSecret))
	fmt.Fprintf(mac, "%s.", timestamp)
	mac.Write(body)
	valid := false
	for _, sig := range signatures {
		decoded, e := hex.DecodeString(sig)
		if e == nil && hmac.Equal(decoded, mac.Sum(nil)) {
			valid = true
		}
	}
	if !valid {
		return "", "", ErrSignature
	}
	var e struct {
		ID, Type string
		Livemode bool
		Account  string
		Data     struct{ Object struct{ Customer string } }
	}
	if json.Unmarshal(body, &e) != nil || e.ID == "" || e.Livemode != s.Config.Live() || e.Account != "" {
		return "", "", ErrSignature
	}
	switch e.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded", "checkout.session.async_payment_failed", "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted", "invoice.paid", "invoice.payment_failed":
		if !strings.HasPrefix(e.Data.Object.Customer, "cus_") {
			return "", "", ErrSignature
		}
		return e.ID, e.Data.Object.Customer, nil
	default:
		return e.ID, "", nil
	}
}
