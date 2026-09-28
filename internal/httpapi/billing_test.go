package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/billing"
)

type billingStoreFake struct {
	fakeStore
	record            billing.Record
	customerCommitted bool
	events            int
}

func (s *billingStoreFake) Billing(context.Context) (billing.Record, error) { return s.record, nil }
func (s *billingStoreFake) EnsureBilling(context.Context) error             { return nil }
func (s *billingStoreFake) MutateBilling(_ context.Context, customer, event string, fn func(*billing.Record) error) error {
	if event != "" {
		s.events++
	}
	err := fn(&s.record)
	if err == nil && s.record.CustomerID != "" {
		s.customerCommitted = true
	}
	return err
}

type gatewayFake struct {
	s    *billingStoreFake
	fail bool
}

func (g gatewayFake) Customer(context.Context, string) (string, error) { return "cus_test", nil }
func (g gatewayFake) Checkout(ctx context.Context, _ *billing.Record, _ string, _ string) (string, error) {
	if end, ok := ctx.Deadline(); !ok || time.Until(end) < 20*time.Second {
		return "", errors.New("billing timeout too short for provider round trips")
	}
	if !g.s.customerCommitted {
		return "", errors.New("customer not committed before checkout")
	}
	return "https://checkout.stripe.com/test", nil
}
func (g gatewayFake) Portal(context.Context, string, string) (string, error) {
	return "https://billing.stripe.com/test", nil
}
func (g gatewayFake) Sync(context.Context, *billing.Record) error {
	if g.fail {
		return billing.ErrUnavailable
	}
	return nil
}
func (g gatewayFake) Event(_ []byte, sig string) (string, string, error) {
	if sig != "test_signature" {
		return "", "", billing.ErrSignature
	}
	return "evt_test", "cus_test", nil
}
func TestBillingRoutesAuthAndWebhookBoundary(t *testing.T) {
	s := &billingStoreFake{record: billing.Record{OrganizationID: "org", Status: "none"}}
	h := New(s, Options{PublicOrigin: "http://localhost", Billing: gatewayFake{s: s}, BillingCloud: true})
	token := strings.Repeat("a", 64)
	send := func(path, method, body string, auth, csrf bool, signature string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if auth {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
			r.Header.Set("Origin", "http://localhost")
			r.Header.Set("X-Xingdu-Request", "1")
			r.Header.Set("X-Xingdu-Organization", "11111111-1111-4111-8111-111111111111")
		}
		if csrf {
			r.Header.Set("X-CSRF-Token", csrfToken(token))
		}
		r.Header.Set("Stripe-Signature", signature)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		path, method, body string
		auth, csrf         bool
		signature          string
		want               int
	}{
		{"/api/v1/billing", "GET", "", false, false, "", 401},
		{"/api/v1/billing/checkout", "POST", `{"interval":"year"}`, true, false, "", 403},
		{"/api/v1/billing/checkout", "POST", `{"interval":"year","price":"price_attacker"}`, true, true, "", 400},
		{"/api/v1/billing/checkout", "POST", `{"interval":"day"}`, true, true, "", 422},
		{"/api/v1/billing/checkout", "POST", `{"interval":"year"}`, true, true, "", 200},
		{stripeWebhookPath, "POST", `{}`, false, false, "", 400},
		{stripeWebhookPath, "POST", `{}`, false, false, "test_signature", 204},
	} {
		w := send(tc.path, tc.method, tc.body, tc.auth, tc.csrf, tc.signature)
		if w.Code != tc.want {
			t.Fatalf("%s got %d want %d: %s", tc.path, w.Code, tc.want, w.Body.String())
		}
	}
	if s.events != 1 {
		t.Fatal("invalid signature reached persistence")
	}
	w := send("/api/v1/billing", "GET", "", true, false, "")
	if strings.Contains(w.Body.String(), "cus_test") || strings.Contains(w.Body.String(), "org\"") {
		t.Fatal("private IDs exposed")
	}
	h = New(s, Options{PublicOrigin: "http://localhost", Billing: gatewayFake{s: s, fail: true}, BillingCloud: true})
	if w = send(stripeWebhookPath, "POST", `{}`, false, false, "test_signature"); w.Code != 503 {
		t.Fatal("failed reconciliation acknowledged")
	}
}
