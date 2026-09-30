package certificates

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/billing"
)

type delegatedFixture struct {
	name    string
	removed bool
}

type cfTransport func(*http.Request) (*http.Response, error)

func (f cfTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPlatformDNSNeverRetargetsOrProxies(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		created := false
		c := Cloudflare{Token: "fixture", Zone: strings.Repeat("a", 32), Client: &http.Client{Transport: cfTransport(func(r *http.Request) (*http.Response, error) {
			result := `{"name":"example.com"}`
			if strings.HasSuffix(r.URL.Path, "/dns_records") {
				if r.Method == "GET" {
					result = `[]`
					if conflict {
						result = `[{"type":"A","content":"1.1.1.1","proxied":false}]`
					}
				}
				if r.Method == "POST" {
					created = true
					b, _ := io.ReadAll(r.Body)
					if !strings.Contains(string(b), `"proxied":false`) || !strings.Contains(string(b), `"content":"8.8.8.8"`) {
						t.Fatal("unsafe platform record")
					}
					result = `{}`
				}
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true,"result":` + result + `}`)), Header: make(http.Header)}, nil
		})}}
		err := c.EnsureAddress(context.Background(), "random.nodes.example.com", "8.8.8.8")
		if conflict {
			if err == nil || created {
				t.Fatal("conflicting DNS overwritten")
			}
		} else if err != nil || !created {
			t.Fatal("platform record not created", err)
		}
	}
	for _, address := range []string{"192.168.1.1", "127.0.0.1", "100.100.100.200", "203.0.113.1", "::ffff:192.168.1.1"} {
		if PublicAddress(address) {
			t.Fatal("unsafe IP accepted", address)
		}
	}
}

func (d *delegatedFixture) Present(_ context.Context, name, value string) (string, error) {
	d.name = name
	if value != "proof" {
		return "", errors.New("wrong proof")
	}
	return "record", nil
}
func (d *delegatedFixture) Remove(_ context.Context, id string) error {
	d.removed = id == "record"
	return nil
}
func TestDelegatedDNSRequiresExactTarget(t *testing.T) {
	ctx := context.Background()
	dns := &delegatedFixture{}
	target := "_acme-challenge.random.validation.example.com"
	d := DelegatedDNS{DNS: dns, Domain: "node.example.com", Target: target, LookupCNAME: func(context.Context, string) (string, error) { return target + ".", nil }}
	if _, err := d.Present(ctx, "_acme-challenge.node.example.com", "proof"); err != nil || dns.name != target {
		t.Fatal("delegated challenge not rewritten", err)
	}
	if err := d.Remove(ctx, "record"); err != nil || !dns.removed {
		t.Fatal("cleanup not delegated")
	}
	d.LookupCNAME = func(context.Context, string) (string, error) {
		return "_acme-challenge.other.validation.example.com.", nil
	}
	dns.name = ""
	if _, err := d.Present(ctx, "_acme-challenge.node.example.com", "proof"); err == nil || dns.name != "" {
		t.Fatal("foreign delegation accepted")
	}
	if _, err := d.Present(ctx, "_acme-challenge.other.example.com", "proof"); err == nil {
		t.Fatal("unexpected challenge accepted")
	}
}
func TestManagedDomainValidation(t *testing.T) {
	for _, domain := range []string{"https://example.com", "*.example.com", "EXAMPLE.com", "127.0.0.1", "node.local", "node.invalid", "example.com/path", "localhost"} {
		if ValidDomain(domain) {
			t.Fatalf("accepted unsafe domain %q", domain)
		}
	}
	if !ValidDomain("node.example.com") {
		t.Fatal("valid domain rejected")
	}
	if (ManagedConfig{}).Configured() {
		t.Fatal("missing provider enabled")
	}
}
func TestCertificateLimits(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		record billing.Record
		want   int
	}{{billing.Record{}, 0}, {billing.Record{Status: "trialing", PeriodEnd: now.Unix() + 3600}, 0}, {billing.Record{Status: "active", PeriodEnd: now.Unix()}, 0}, {billing.Record{Status: "active", PeriodEnd: now.Unix() + 3600, Plan: "start", CancelAtPeriodEnd: true}, 10}, {billing.Record{Status: "active", PeriodEnd: now.Unix() + 3600, Plan: "premium"}, 50}} {
		if got := tc.record.CertificateLimit(now); got != tc.want {
			t.Fatalf("limit %d != %d", got, tc.want)
		}
	}
}
