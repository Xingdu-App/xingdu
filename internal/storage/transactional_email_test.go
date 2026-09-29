package storage

import (
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/billing"
	"xingdu.app/xingdu/internal/emailverification"
)

type mailRecorder struct {
	messages []emailverification.Message
	fail     bool
}

func (m *mailRecorder) Configured() bool { return true }
func (m *mailRecorder) SendMessage(_ context.Context, _ string, v emailverification.Message, _ string) error {
	m.messages = append(m.messages, v)
	if m.fail {
		return emailverification.ErrUnavailable
	}
	return nil
}
func TestTransactionalEmailRecoveryAndQueue(t *testing.T) {
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
	uri, _ := url.Parse(raw)
	q := uri.Query()
	q.Set("options", "-crole=xingdu_app")
	uri.RawQuery = q.Encode()
	app, err := Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	email := "mail-" + NewID("obj") + "@example.invalid"
	uid := NewID("usr")
	hash, _ := bcrypt.GenerateFromPassword([]byte("original-test-password"), bcrypt.MinCost)
	if _, err = admin.Pool.Exec(ctx, `INSERT INTO users(id,username,email,email_verified_at,password_hash) VALUES($1,$2,$2,now(),$3)`, uid, email, string(hash)); err != nil {
		t.Fatal(err)
	}
	defer admin.Pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, uid)
	defer admin.Pool.Exec(ctx, `DELETE FROM password_recoveries WHERE email=$1`, email)
	defer admin.Pool.Exec(ctx, `DELETE FROM email_deliveries WHERE recipient=$1`, email)
	if _, err = app.Pool.Exec(ctx, `SELECT * FROM email_deliveries`); err == nil {
		t.Fatal("app can directly read global mail queue")
	}
	token := strings.Repeat("a", 64)
	code := strings.Repeat("b", 64)
	if err = app.BeginPasswordRecovery(ctx, token, email, code); err != nil {
		t.Fatal(err)
	}
	if err = app.BeginPasswordRecovery(ctx, strings.Repeat("c", 64), email, code); !errors.Is(err, ErrRegistrationLimited) {
		t.Fatal("cooldown missing", err)
	}
	replacement, _ := bcrypt.GenerateFromPassword([]byte("replacement-test-password"), bcrypt.MinCost)
	if err = app.CompletePasswordRecovery(ctx, token, code, string(replacement)); !errors.Is(err, ErrVerification) {
		t.Fatal("undelivered code accepted", err)
	}
	if err = app.DeliverPasswordRecovery(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err = app.NewSession(ctx, strings.Repeat("d", 64), uid, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = app.CompletePasswordRecovery(ctx, token, strings.Repeat("e", 64), string(replacement)); !errors.Is(err, ErrVerification) {
		t.Fatal("wrong code accepted", err)
	}
	var attempts int
	_ = admin.Pool.QueryRow(ctx, `SELECT attempts FROM password_recoveries WHERE token_hash=$1`, token).Scan(&attempts)
	if attempts != 1 {
		t.Fatal("failed attempts not committed", attempts)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- app.CompletePasswordRecovery(ctx, token, code, string(replacement))
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		} else if !errors.Is(e, ErrVerification) {
			t.Fatal(e)
		}
	}
	if success != 1 {
		t.Fatal("reset not one-use", success)
	}
	var count int
	_ = admin.Pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id=$1`, uid).Scan(&count)
	if count != 0 {
		t.Fatal("sessions survived recovery")
	}
	_ = admin.Pool.QueryRow(ctx, `SELECT count(*) FROM email_deliveries WHERE recipient=$1`, email).Scan(&count)
	if count != 2 {
		t.Fatal("welcome/password notice not atomically enqueued", count)
	}
	// Deliver only this dedicated test database's queue, with a mock provider.
	sender := &mailRecorder{fail: true}
	if !app.deliverEmail(ctx, sender, "https://xingdu.example") {
		t.Fatal("queue claim failed")
	}
	_ = admin.Pool.QueryRow(ctx, `SELECT count(*) FROM email_deliveries WHERE recipient=$1 AND sent_at IS NULL`, email).Scan(&count)
	if count != 2 {
		t.Fatal("failed delivery marked sent")
	}
	_, _ = admin.Pool.Exec(ctx, `UPDATE email_deliveries SET available_at=now() WHERE recipient=$1`, email)
	sender.fail = false
	for i := 0; i < 2; i++ {
		if !app.deliverEmail(ctx, sender, "https://xingdu.example") {
			t.Fatal("queue retry failed")
		}
	}
	_ = admin.Pool.QueryRow(ctx, `SELECT count(*) FROM email_deliveries WHERE recipient=$1 AND sent_at IS NOT NULL`, email).Scan(&count)
	if count != 2 {
		t.Fatal("successful delivery not recorded", count)
	}
	org, err := app.CreateOrganization(ctx, uid, "Mail billing fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Pool.Exec(ctx, `DELETE FROM organizations WHERE id=$1`, org.ID)
	scoped := WithTenant(ctx, uid, org.ID)
	if err = app.EnsureBilling(scoped); err != nil {
		t.Fatal(err)
	}
	if err = app.MutateBilling(scoped, "", "", func(r *billing.Record) error {
		r.CustomerID = "cus_mail_fixture"
		r.Status = "active"
		r.Plan = "premium"
		return nil
	}); err != nil {
		t.Fatal("billing notification transaction", err)
	}
	if err = app.MutateBilling(ctx, "cus_mail_fixture", "evt_mail_fixture", func(r *billing.Record) error { r.Status = "past_due"; return nil }); err != nil {
		t.Fatal("webhook mail RLS", err)
	}
	if err = app.MutateBilling(ctx, "cus_mail_fixture", "evt_mail_fixture", func(r *billing.Record) error { t.Fatal("duplicate webhook reprocessed"); return nil }); err != nil {
		t.Fatal(err)
	}
	_ = admin.Pool.QueryRow(ctx, `SELECT count(*) FROM email_deliveries WHERE recipient=$1 AND kind LIKE 'billing_%'`, email).Scan(&count)
	if count != 2 {
		t.Fatal("billing duplicate/missing", count)
	}
	if err = app.MutateBilling(scoped, "", "", func(r *billing.Record) error { r.CancelAtPeriodEnd = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if err = app.MutateBilling(scoped, "", "", func(r *billing.Record) error { r.Status = "canceled"; return nil }); err != nil {
		t.Fatal(err)
	}
	var kinds []string
	rows, err := admin.Pool.Query(ctx, `SELECT kind FROM email_deliveries WHERE recipient=$1 AND kind LIKE 'billing_%' ORDER BY id`, email)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k string
		_ = rows.Scan(&k)
		kinds = append(kinds, k)
	}
	rows.Close()
	if len(kinds) != 4 || kinds[0] != "billing_active" || kinds[1] != "billing_attention" || kinds[3] != "billing_ended" {
		t.Fatal("unexpected billing mail", kinds)
	}

	// A stale snapshot cannot overwrite a password changed after code issuance.
	_, _ = admin.Pool.Exec(ctx, `UPDATE password_recoveries SET created_at=now()-interval '61 seconds' WHERE email=$1`, email)
	token = strings.Repeat("f", 64)
	if err = app.BeginPasswordRecovery(ctx, token, email, code); err != nil {
		t.Fatal(err)
	}
	_ = app.DeliverPasswordRecovery(ctx, token)
	_, _ = admin.Pool.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, uid, string(hash))
	if err = app.CompletePasswordRecovery(ctx, token, code, string(replacement)); !errors.Is(err, ErrVerification) {
		t.Fatal("stale password snapshot accepted", err)
	}
	// Codes expire and at most five failed guesses are accepted per challenge.
	_, _ = admin.Pool.Exec(ctx, `UPDATE password_recoveries SET created_at=now()-interval '61 seconds' WHERE email=$1`, email)
	retryToken := strings.Repeat("1", 64)
	if err = app.BeginPasswordRecovery(ctx, retryToken, email, code); err != nil {
		t.Fatal(err)
	}
	_ = app.DeliverPasswordRecovery(ctx, retryToken)
	for i := 0; i < 5; i++ {
		if err = app.CompletePasswordRecovery(ctx, retryToken, strings.Repeat("2", 64), string(replacement)); !errors.Is(err, ErrVerification) {
			t.Fatal("wrong code accepted")
		}
	}
	if err = app.CompletePasswordRecovery(ctx, retryToken, code, string(replacement)); !errors.Is(err, ErrVerification) {
		t.Fatal("attempt cap bypassed")
	}
	_, _ = admin.Pool.Exec(ctx, `UPDATE password_recoveries SET attempts=0,expires_at=now()-interval '1 second' WHERE token_hash=$1`, retryToken)
	if err = app.CompletePasswordRecovery(ctx, retryToken, code, string(replacement)); !errors.Is(err, ErrVerification) {
		t.Fatal("expired recovery accepted")
	}

	// Social-only accounts must not gain a password through recovery.
	_, _ = admin.Pool.Exec(ctx, `UPDATE users SET password_login_enabled=false,password_hash='' WHERE id=$1`, uid)
	if err = app.CompletePasswordRecovery(ctx, token, code, string(replacement)); !errors.Is(err, ErrVerification) {
		t.Fatal("social-only reset accepted", err)
	}
}
