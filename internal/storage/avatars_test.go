package storage

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
)

func TestAvatarAccountIsolation(t *testing.T) {
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
	var ids []string
	defer func() {
		for _, id := range ids {
			admin.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", id)
		}
		for _, id := range ids {
			admin.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", id)
		}
	}()
	for range 2 {
		id, e := s.Register(ctx, "avatar_"+NewID("obj")[4:12], "unused", "Avatar test")
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	for i, id := range ids {
		if err = s.SaveAvatar(ctx, id, []byte{byte(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	for i, id := range ids {
		got, e := s.Avatar(ctx, id)
		if e != nil || !bytes.Equal(got, []byte{byte(i + 1)}) {
			t.Fatal("avatar not persisted", e)
		}
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = setScope(ctx, tx, ids[0], ""); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM user_avatars WHERE user_id=$1", ids[1]).Scan(&count); err != nil || count != 0 {
		t.Fatal("other account avatar visible", err)
	}
	result, err := tx.Exec(ctx, "UPDATE user_avatars SET image=$1 WHERE user_id=$2", []byte{9}, ids[1])
	if err != nil || result.RowsAffected() != 0 {
		t.Fatal("other account avatar writable", err)
	}
	tx.Rollback(ctx)
	if err = s.SaveAvatar(ctx, ids[0], nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Avatar(ctx, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Fatal("avatar was not removed", err)
	}
}
