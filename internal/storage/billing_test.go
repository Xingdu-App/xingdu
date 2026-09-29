package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/billing"
	"xingdu.app/xingdu/internal/hosts"
)

func TestBillingTenantWebhookAndQuota(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	admin, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err = admin.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("options", "-crole=xingdu_app")
	u.RawQuery = q.Encode()
	s, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.CloudBilling = true
	ids := []string{}
	defer func() {
		for _, id := range ids {
			admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		}
		for _, id := range ids {
			admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
		}
	}()
	register := func() (string, string) {
		id, e := s.Register(ctx, "bill_"+NewID("obj")[4:12], "unused", "Billing test")
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
		orgs, e := s.Organizations(ctx, id)
		if e != nil {
			t.Fatal(e)
		}
		return id, orgs[0].ID
	}
	a, orgA := register()
	b, orgB := register()
	member, _ := register()
	ca, cb := WithTenant(ctx, a, orgA), WithTenant(ctx, b, orgB)
	if _, err = admin.Pool.Exec(ctx, "INSERT INTO memberships(organization_id,user_id,role) VALUES($1,$2,'admin')", orgA, member); err != nil {
		t.Fatal(err)
	}
	for _, c := range []context.Context{WithTenant(ctx, b, orgA), WithTenant(ctx, member, orgA)} {
		if err = s.EnsureBilling(c); !errors.Is(err, ErrForbidden) {
			t.Fatal("non-owner can purchase", err)
		}
	}
	for _, c := range []context.Context{ca, cb} {
		if err = s.EnsureBilling(c); err != nil {
			t.Fatal(err)
		}
	}
	cusA, cusB := "cus_"+NewID("obj"), "cus_"+NewID("obj")
	if err = s.MutateBilling(ca, "", "", func(r *billing.Record) error { r.CustomerID = cusA; return nil }); err != nil {
		t.Fatal(err)
	}
	if err = s.MutateBilling(cb, "", "", func(r *billing.Record) error { r.CustomerID = cusB; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Billing(WithTenant(ctx, b, orgA)); !errors.Is(err, ErrForbidden) {
		t.Fatal("cross tenant read", err)
	}
	if r, e := s.Billing(WithTenant(ctx, member, orgA)); e != nil || r.CustomerID != cusA {
		t.Fatal("member cannot read billing", e)
	}
	var count int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM organization_billing").Scan(&count); err != nil || count != 0 {
		t.Fatal("unscoped billing visible", err)
	}
	host := hosts.Input{Name: "Billing host", Address: "example.invalid", SSHPort: 22, SSHUser: "root", Tags: []string{}}
	expectCode := func(e error, code string) {
		t.Helper()
		var p *pgconn.PgError
		if !errors.As(e, &p) || p.Code != code {
			t.Fatalf("wanted SQLSTATE %s, got %v", code, e)
		}
	}
	_, err = s.CreateHost(ca, host)
	if err != nil {
		t.Fatal("free server rejected", err)
	}
	extra := host
	extra.Address = "second.example.invalid"
	_, err = s.CreateHost(ca, extra)
	expectCode(err, "P0004")
	calls := 0
	event := "evt_" + NewID("obj")
	apply := func(r *billing.Record) error {
		calls++
		if r.OrganizationID != orgA {
			t.Fatal("wrong webhook org")
		}
		r.Status = "active"
		r.PeriodEnd = time.Now().Add(time.Hour).Unix()
		r.SubscriptionID = "sub_test"
		r.Interval = "month"
		return nil
	}
	for range 2 {
		if err = s.MutateBilling(ctx, cusA, event, apply); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("webhook not idempotent")
	}
	if err = s.MutateBilling(ctx, "cus_unknown", "evt_unknown", apply); err != nil || calls != 1 {
		t.Fatal("unknown customer escaped scope", err)
	}
	r, err := s.Billing(cb)
	if err != nil || r.Status != "none" {
		t.Fatal("other organization granted", err)
	}
	_, err = s.CreateHost(cb, host)
	if err != nil {
		t.Fatal("free server rejected", err)
	}
	_, err = s.CreateHost(cb, extra)
	expectCode(err, "P0004")
	// Concurrent inserts share the same organization lock and cannot exceed ten (including the free server).
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for range 12 {
		wg.Go(func() {
			input := host
			input.Name = "Quota " + NewID("obj")
			input.Address = NewID("obj") + ".example.invalid"
			_, e := s.CreateHost(ca, input)
			results <- e
		})
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else {
			expectCode(e, "P0004")
		}
	}
	if success != 9 {
		t.Fatalf("host quota admitted %d", success)
	}
	ops, err := s.OrganizationOperations(ca)
	if err != nil || ops.Usage["hosts"] != (ResourceUsage{10, 10}) {
		t.Fatalf("paid usage: %v %v", ops.Usage, err)
	}
	if err = s.MutateBilling(ctx, cusA, "evt_"+NewID("obj"), func(r *billing.Record) error { r.Status = "past_due"; return nil }); err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateHost(ca, host)
	expectCode(err, "P0004")
	ops, err = s.OrganizationOperations(ca)
	if err != nil || ops.Usage["hosts"] != (ResourceUsage{10, 1}) {
		t.Fatalf("expired usage: %v %v", ops.Usage, err)
	}
	existing, err := s.Hosts(ca)
	if err != nil || len(existing) != 10 {
		t.Fatal("existing hosts should remain readable", err)
	}
	if err = s.DeleteHost(ca, existing[0].ID); err != nil {
		t.Fatal("unpaid organization cannot clean up", err)
	}
	// Self-hosting stays free even with no Stripe subscription.
	s.CloudBilling = false
	if _, err = s.CreateHost(cb, extra); err != nil {
		t.Fatal("self-hosting paywalled", err)
	}
	// Failed provider work must not acknowledge the event or alter entitlement.
	failureEvent := "evt_" + NewID("obj")
	if err = s.MutateBilling(ctx, cusA, failureEvent, func(r *billing.Record) error { r.Status = "active"; return billing.ErrUnavailable }); !errors.Is(err, billing.ErrUnavailable) {
		t.Fatal(err)
	}
	if err = s.MutateBilling(ctx, cusA, failureEvent, apply); err != nil || calls != 2 {
		t.Fatal("failed event not retryable", err)
	}
}
