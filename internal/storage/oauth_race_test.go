package storage

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"
)

// A request that read an old identity must not delete a replacement binding
// after waiting on the old subject's lock. The replacement transaction holds
// the same identity lock as a real concurrent unlink operation.
func TestOAuthUnlinkDoesNotDeleteReplacedSubject(t *testing.T) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if err = admin.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	uri, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	application := "oauth-race-" + NewID()
	q := uri.Query()
	q.Set("options", "-crole=xingdu_app")
	q.Set("application_name", application)
	uri.RawQuery = q.Encode()
	app, err := Open(ctx, uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	userID, err := app.Register(ctx, "race_"+NewID()[:8], "unused-test-hash", "OAuth race")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		admin.Pool.Exec(context.Background(), "DELETE FROM organizations WHERE created_by=$1", userID)
		admin.Pool.Exec(context.Background(), "DELETE FROM users WHERE id=$1", userID)
	}()
	session := "oauth-race-session-" + NewID()
	if err = app.NewSession(ctx, session, userID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	oldSubject, newSubject := "old-"+NewID(), "new-"+NewID()
	if _, err = app.CompleteOAuth(ctx, OAuthState{Provider: "github", Mode: "link", UserID: userID, SessionHash: session}, oldSubject, "race@example.invalid", "", time.Now().Add(time.Hour), false); err != nil {
		t.Fatal(err)
	}
	replacement, err := admin.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Rollback(context.Background())
	if _, err = replacement.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('identity:github:' || $1,0))`, oldSubject); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		result <- app.UnlinkOAuthIdentity(ctx, userID, "github", session, []string{"github", "google"})
	}()
	// Wait on an observable PostgreSQL lock, not a sleep-based race assumption.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		err = admin.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event='advisory')`, application).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("unlink did not wait on old identity lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err = replacement.Exec(ctx, `DELETE FROM oauth_identities WHERE user_id=$1 AND provider='github' AND subject=$2`, userID, oldSubject); err != nil {
		t.Fatal(err)
	}
	if _, err = replacement.Exec(ctx, `INSERT INTO oauth_identities(provider,subject,user_id,email) VALUES('github',$1,$2,'new@example.invalid')`, newSubject, userID); err != nil {
		t.Fatal(err)
	}
	if err = replacement.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-result; !errors.Is(err, ErrConflict) {
		t.Fatalf("stale unlink must conflict, got %v", err)
	}
	var retained string
	if err = admin.Pool.QueryRow(ctx, `SELECT subject FROM oauth_identities WHERE user_id=$1 AND provider='github'`, userID).Scan(&retained); err != nil || retained != newSubject {
		t.Fatalf("replacement identity was removed: %v", err)
	}
}
