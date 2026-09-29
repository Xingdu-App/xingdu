package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/url"
	"time"
	"xingdu.app/xingdu/internal/emailverification"
)

// RunEmailDelivery runs only in the API process, where the provider key resides.
func (s *Store) RunEmailDelivery(ctx context.Context, sender emailverification.MessageSender, origin string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if sender == nil || !sender.Configured() {
				continue
			}
			// Sequential requests stay below the default provider rate and bound work.
			for i := 0; i < 2; i++ {
				if !s.deliverEmail(ctx, sender, origin) {
					break
				}
			}
		}
	}
}
func (s *Store) deliverEmail(ctx context.Context, sender emailverification.MessageSender, origin string) bool {
	work, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var id int64
	var to, kind, lease string
	var payload []byte
	err := s.Pool.QueryRow(work, `SELECT id,recipient,kind,payload,lease_token FROM claim_email_delivery()`).Scan(&id, &to, &kind, &payload, &lease)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return false
	}
	var m emailverification.Message
	if json.Unmarshal(payload, &m) != nil {
		return false
	}
	m.Kind = kind
	m.URL = origin + "/app"
	if kind == "password_changed" {
		m.URL = origin + "/login"
	}
	if len(kind) >= 8 && kind[:8] == "billing_" {
		var p struct {
			OrganizationID string `json:"organization_id"`
		}
		_ = json.Unmarshal(payload, &p)
		m.URL = origin + "/app/billing?organization=" + url.QueryEscape(p.OrganizationID)
	}
	err = sender.SendMessage(work, to, m, fmt.Sprintf("xingdu-notice/%d", id))
	// Stable payload + key makes retries safe when provider acceptance precedes a crash.
	_, _ = s.Pool.Exec(work, `SELECT finish_email_delivery($1,$2,$3)`, id, lease, err == nil)
	return true
}
