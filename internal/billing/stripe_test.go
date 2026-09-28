package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{Mode: "cloud", SecretKey: "sk_test_example", WebhookSecret: "whsec_example", MonthlyPrice: "price_month", YearlyPrice: "price_year", PortalConfiguration: "bpc_example"}
}
func signed(body, secret string, at int64) string {
	stamp := fmt.Sprint(at)
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(stamp + "." + body))
	return "t=" + stamp + ",v1=" + hex.EncodeToString(m.Sum(nil))
}
func TestEventVerification(t *testing.T) {
	s := New(testConfig())
	body := `{"id":"evt_1","type":"invoice.paid","livemode":false,"data":{"object":{"customer":"cus_1"}}}`
	sig := signed(body, s.Config.WebhookSecret, time.Now().Unix())
	id, customer, err := s.Event([]byte(body), sig)
	if err != nil || id != "evt_1" || customer != "cus_1" {
		t.Fatal("valid event rejected", err)
	}
	for _, tc := range []struct{ body, sig string }{
		{body + " ", sig}, {body, signed(body, "wrong", time.Now().Unix())}, {body, signed(body, s.Config.WebhookSecret, time.Now().Add(-6*time.Minute).Unix())}, {body, signed(body, s.Config.WebhookSecret, time.Now().Add(6*time.Minute).Unix())}, {body, ""},
	} {
		if _, _, err = s.Event([]byte(tc.body), tc.sig); err == nil {
			t.Fatal("invalid signature accepted")
		}
	}
	live := strings.Replace(body, `"livemode":false`, `"livemode":true`, 1)
	if _, _, err = s.Event([]byte(live), signed(live, s.Config.WebhookSecret, time.Now().Unix())); err == nil {
		t.Fatal("mixed Stripe modes")
	}
	if _, _, err = s.Event([]byte(body), sig+",v1=000000"); err != nil {
		t.Fatal("rotation signature rejected")
	}
}
func subscription(status string) map[string]any {
	return map[string]any{"id": "sub_1", "customer": "cus_1", "status": status, "livemode": false, "cancel_at_period_end": false, "latest_invoice": map[string]any{"status": "paid"}, "items": map[string]any{"data": []any{map[string]any{"quantity": 1, "current_period_end": time.Now().Add(time.Hour).Unix(), "price": map[string]any{"id": "price_month", "currency": "usd", "unit_amount": 500, "recurring": map[string]any{"interval": "month", "interval_count": 1, "usage_type": "licensed"}}}}}}
}
func TestCanonicalSubscriptionAndPaymentState(t *testing.T) {
	state := subscription("active")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("customer") != "cus_1" || r.URL.Query().Get("expand[]") != "data.latest_invoice" {
			t.Error("wrong customer/expansion")
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []any{state}, "has_more": false})
	}))
	defer server.Close()
	s := New(testConfig())
	s.base = server.URL
	for _, tc := range []struct {
		status                 string
		paid, cancel, entitled bool
	}{{"active", true, false, true}, {"active", true, true, true}, {"past_due", false, false, false}, {"unpaid", false, false, false}, {"incomplete", false, false, false}, {"active", false, false, false}, {"canceled", true, false, false}} {
		state = subscription(tc.status)
		state["cancel_at_period_end"] = tc.cancel
		if !tc.paid {
			state["latest_invoice"] = map[string]any{"status": "open"}
		}
		r := Record{CustomerID: "cus_1", SubscriptionID: "sub_1"}
		if err := s.Sync(context.Background(), &r); err != nil {
			t.Fatal(err)
		}
		if r.Entitled(time.Now()) != tc.entitled {
			t.Fatalf("incorrect entitlement: %+v -> %+v", tc, r)
		}
	}
	state = subscription("active")
	state["items"] = map[string]any{"data": []any{map[string]any{"quantity": 1, "price": map[string]any{"id": "price_external"}}}}
	r := Record{CustomerID: "cus_1"}
	if err := s.Sync(context.Background(), &r); err != nil || r.Status != "unsupported" || r.Entitled(time.Now()) {
		t.Fatal("unknown plan granted", err)
	}
	state = subscription("active")
	state["customer"] = "cus_other"
	if err := s.Sync(context.Background(), &r); err == nil {
		t.Fatal("customer mismatch allowed")
	}
}
func TestCheckoutAmountIdempotencyAndReuse(t *testing.T) {
	creates := 0
	amount := 4000
	active := false
	keys := []string{}
	sessionStatus := "open"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Stripe-Version") != APIVersion {
			t.Error("version missing")
		}
		user, _, _ := r.BasicAuth()
		if user != "sk_test_example" {
			t.Error("missing authentication")
		}
		switch r.URL.Path {
		case "/subscriptions":
			if active {
				json.NewEncoder(w).Encode(map[string]any{"data": []any{subscription("active")}})
			} else {
				fmt.Fprint(w, `{"data":[],"has_more":false}`)
			}
		case "/prices/price_year":
			fmt.Fprintf(w, `{"active":true,"livemode":false,"currency":"usd","unit_amount":%d,"recurring":{"interval":"year","interval_count":1,"usage_type":"licensed"}}`, amount)
		case "/checkout/sessions":
			creates++
			r.ParseForm()
			keys = append(keys, r.Header.Get("Idempotency-Key"))
			if r.Form.Get("line_items[0][price]") != "price_year" || r.Form.Get("line_items[0][quantity]") != "1" || r.Form.Get("customer") != "cus_1" || r.Form.Get("mode") != "subscription" || r.Form.Get("subscription_data[metadata][xingdu_organization]") != "org_1" {
				t.Error("wrong checkout parameters")
			}
			if !strings.Contains(r.Form.Get("success_url"), "organization=org_1") {
				t.Error("organization not preserved")
			}
			fmt.Fprint(w, `{"id":"cs_1","url":"https://checkout.stripe.com/c/pay/test"}`)
		case "/checkout/sessions/cs_1":
			fmt.Fprintf(w, `{"status":%q,"url":"https://checkout.stripe.com/c/pay/test"}`, sessionStatus)
		default:
			t.Errorf("unexpected Stripe endpoint %s", r.URL.Path)
			http.Error(w, "unexpected", 500)
		}
	}))
	defer server.Close()
	s := New(testConfig())
	s.base = server.URL
	r := Record{OrganizationID: "org_1", CustomerID: "cus_1", Attempt: "attempt_1", Status: "none"}
	if _, err := s.Checkout(context.Background(), &r, "year", "https://xingdu.app"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Checkout(context.Background(), &r, "year", "https://xingdu.app"); err != nil || creates != 1 {
		t.Fatal("open session not reused", err)
	}
	// A lost local commit retries the same external mutation with the same key.
	r.CheckoutID = ""
	if _, err := s.Checkout(context.Background(), &r, "year", "https://xingdu.app"); err != nil || len(keys) != 2 || keys[0] == "" || keys[0] != keys[1] {
		t.Fatal("unstable idempotency key", err)
	}
	active = true
	if _, err := s.Checkout(context.Background(), &r, "year", "https://xingdu.app"); err != ErrConflict {
		t.Fatal("duplicate subscription allowed", err)
	}
	active = false
	amount = 5000
	r.CheckoutID = ""
	if _, err := s.Checkout(context.Background(), &r, "year", "https://xingdu.app"); err != ErrUnavailable {
		t.Fatal("incorrect price allowed", err)
	}
	amount = 4000
	r.CheckoutID = "cs_1"
	r.SubscriptionID = "sub_old"
	r.Status = "canceled"
	sessionStatus = "complete"
	if _, err := s.Checkout(context.Background(), &r, "year", "https://xingdu.app"); err != nil {
		t.Fatal("resubscribe after cancellation blocked", err)
	}
}
func TestPortalPolicy(t *testing.T) {
	unsafe := true
	created := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "configurations") {
			fmt.Fprintf(w, `{"active":true,"features":{"subscription_update":{"enabled":%t},"subscription_cancel":{"enabled":true,"mode":"at_period_end"},"payment_method_update":{"enabled":true},"invoice_history":{"enabled":true}}}`, unsafe)
			return
		}
		created = true
		r.ParseForm()
		if r.Form.Get("customer") != "cus_1" || r.Form.Get("return_url") != "https://xingdu.app/app/billing?organization=org_1" {
			t.Error("wrong portal scope")
		}
		fmt.Fprint(w, `{"url":"https://billing.stripe.com/p/session/test"}`)
	}))
	defer server.Close()
	s := New(testConfig())
	s.base = server.URL
	if _, err := s.Portal(context.Background(), "cus_1", "https://xingdu.app/app/billing?organization=org_1"); err != ErrUnavailable || created {
		t.Fatal("unsafe portal configuration allowed")
	}
	unsafe = false
	if _, err := s.Portal(context.Background(), "cus_1", "https://xingdu.app/app/billing?organization=org_1"); err != nil || !created {
		t.Fatal(err)
	}
}
func TestConfigurationFailsClosed(t *testing.T) {
	c := testConfig()
	if c.Validate() != nil {
		t.Fatal("valid config rejected")
	}
	c.YearlyPrice = c.MonthlyPrice
	if c.Validate() == nil {
		t.Fatal("same price accepted")
	}
	c = Config{Mode: "cloud"}
	if c.Validate() == nil {
		t.Fatal("missing secrets accepted")
	}
	c = Config{}
	if c.Validate() != nil || c.Cloud() {
		t.Fatal("self hosting requires no Stripe")
	}
}
