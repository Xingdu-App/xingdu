package emailverification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestResendPlaceholderAndSanitizedFailure(t *testing.T) {
	for _, key := range []string{"", "PLACEHOLDER_REPLACE_WITH_RESEND_API_KEY", "re_xxxxxxxxxxxxxxxxx", "re_placeholder_key"} {
		s := NewResend(key, "Xingdu <noreply@example.invalid>")
		s.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("placeholder sent request"); return nil, nil })
		if s.Configured() {
			t.Fatal("placeholder configured")
		}
		if s.Send(context.Background(), "fixture@example.invalid", "12345678", "test") != ErrUnavailable {
			t.Fatal("placeholder allowed")
		}
	}
	s := NewResend("re_unit-test-fixture-123456", "Xingdu <noreply@example.invalid>")
	s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.resend.com/emails" || r.Header.Get("Idempotency-Key") != "xingdu-registration/id" {
			t.Fatal("wrong endpoint/idem")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || !strings.Contains(body["text"].(string), "12345678") || !strings.Contains(body["html"].(string), "12345678") || body["reply_to"] != "info@xingdu.app" {
			t.Fatal("message missing code")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":"fixture-id"}`))}, nil
	})
	if err := s.Send(context.Background(), "fixture@example.invalid", "12345678", "id"); err != nil {
		t.Fatal(err)
	}
	s.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("sensitive-provider-response"))}, nil
	})
	if err := s.Send(context.Background(), "fixture@example.invalid", "12345678", "id"); err != ErrUnavailable {
		t.Fatal("provider details leaked", err)
	}
}
