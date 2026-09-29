package storage

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	resourceid "xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/subscription"
)

type Subscription struct {
	EncryptedToken   []byte              `json:"-"`
	TokenHash        string              `json:"-"`
	SubscriptionPath string              `json:"subscription_path,omitempty"`
	LinkState        string              `json:"link_state,omitempty"`
	Format           string              `json:"format"`
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	NodeIDs          []string            `json:"node_ids"`
	Rules            []subscription.Rule `json:"rules"`
	FinalAction      string              `json:"final_action"`
	Enabled          bool                `json:"enabled"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

func (s Subscription) Validate() error {
	if s.Format != "" && !subscription.ValidFormat(s.Format) {
		return ErrInvalid
	}
	if strings.TrimSpace(s.Name) != s.Name || utf8.RuneCountInString(s.Name) < 1 || utf8.RuneCountInString(s.Name) > 80 || strings.IndexFunc(s.Name, unicode.IsControl) >= 0 || len(s.NodeIDs) > 100 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range s.NodeIDs {
		if !resourceid.Valid("node", id) || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	if subscription.ValidateRules(s.Rules, s.FinalAction) != nil {
		return ErrInvalid
	}
	if s.Format == "hysteria2_uri" && (len(s.Rules) > 0 || s.FinalAction != "proxy") {
		return ErrInvalid
	}
	return nil
}

const subscriptionColumns = `s.id::text,s.name,s.format,s.rules,s.final_action,s.enabled,s.created_at,s.updated_at,COALESCE((SELECT array_agg(n.node_id::text ORDER BY n.position) FROM subscription_nodes n WHERE n.subscription_id=s.id),'{}')`

func scanSubscription(row pgx.Row, out *Subscription) error {
	var rules []byte
	if err := row.Scan(&out.ID, &out.Name, &out.Format, &rules, &out.FinalAction, &out.Enabled, &out.CreatedAt, &out.UpdatedAt, &out.NodeIDs); err != nil {
		return err
	}
	return json.Unmarshal(rules, &out.Rules)
}
func (s *Store) Subscriptions(ctx context.Context) ([]Subscription, error) {
	out := []Subscription{}
	tx, role, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT "+subscriptionColumns+" FROM subscriptions s ORDER BY s.created_at DESC,s.id")
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var sub Subscription
		if err = scanSubscription(rows, &sub); err != nil {
			rows.Close()
			return out, err
		}
		out = append(out, sub)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	// Ordinary members and API-key metadata reads never receive bearer links.
	_, apiKey := ctx.Value(apiKeyContext{}).(string)
	if (role == "owner" || role == "admin") && !apiKey {
		for i := range out {
			if err = tx.QueryRow(ctx, "SELECT encrypted_token,token_hash FROM subscriptions WHERE id=$1", out[i].ID).Scan(&out[i].EncryptedToken, &out[i].TokenHash); err != nil {
				return nil, err
			}
			out[i].LinkState = "legacy"
			if len(out[i].EncryptedToken) > 0 {
				out[i].LinkState = "available"
			}
		}
		if len(out) > 0 {
			if err = subscriptionAudit(ctx, tx, out[0].ID, "subscription_links_viewed"); err != nil {
				return nil, err
			}
		}
	}
	return out, tx.Commit(ctx)
}
func subscriptionAudit(ctx context.Context, tx pgx.Tx, id, event string) error {
	_, err := tx.Exec(ctx, `INSERT INTO subscription_audit(organization_id,subscription_id,actor_id,event) VALUES(request_org_id(),$1,request_user_id(),$2)`, id, event)
	return err
}
func (s *Store) SaveSubscription(ctx context.Context, in Subscription, hash string, create bool) (Subscription, error) {
	if create && len(in.NodeIDs) == 0 {
		return in, ErrInvalid
	}
	if err := in.Validate(); err != nil {
		return in, err
	}
	if in.Rules == nil {
		in.Rules = []subscription.Rule{}
	}
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return in, err
	}
	defer tx.Rollback(ctx)
	if in.Format == "" {
		if create {
			in.Format = "stash"
		} else {
			if err = tx.QueryRow(ctx, "SELECT format FROM subscriptions WHERE id=$1", in.ID).Scan(&in.Format); err != nil {
				return in, mapError(err)
			}
		}
	}
	if err = in.Validate(); err != nil {
		return in, err
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM protocol_deployments d WHERE d.id::text=ANY($1::text[]) AND ((installed_at IS NOT NULL AND encrypted IS NOT NULL AND ((state='succeeded' AND action='deploy') OR (action='restart' AND state IN ('queued','running','failed','interrupted')))) OR (NOT $2::boolean AND EXISTS(SELECT 1 FROM subscription_nodes n WHERE n.subscription_id=$3 AND n.node_id=d.id)))`, in.NodeIDs, create, in.ID).Scan(&count); err != nil {
		return in, err
	}
	if count != len(in.NodeIDs) {
		return in, ErrInvalid
	}
	// Reject unsupported combinations at save time, not only on client refresh.
	protocolRows, err := tx.Query(ctx, "SELECT protocol FROM protocol_deployments WHERE id::text=ANY($1::text[])", in.NodeIDs)
	if err != nil {
		return in, err
	}
	for protocolRows.Next() {
		var kind string
		if err = protocolRows.Scan(&kind); err != nil {
			protocolRows.Close()
			return in, err
		}
		if !subscription.Supports(in.Format, kind) {
			protocolRows.Close()
			return in, ErrInvalid
		}
	}
	err = protocolRows.Err()
	protocolRows.Close()
	if err != nil {
		return in, err
	}
	rules, _ := json.Marshal(in.Rules)
	event := "subscription_updated"
	if create {
		event = "subscription_created"
		_, err = tx.Exec(ctx, `INSERT INTO subscriptions(id,organization_id,name,rules,final_action,enabled,token_hash,format,encrypted_token) VALUES($1,request_org_id(),$2,$3,$4,$5,$6,$7,$8)`, in.ID, in.Name, rules, in.FinalAction, in.Enabled, hash, in.Format, in.EncryptedToken)
	} else {
		var found string
		err = tx.QueryRow(ctx, `UPDATE subscriptions SET name=$2,rules=$3,final_action=$4,enabled=$5,format=$6,updated_at=now() WHERE id=$1 RETURNING id::text`, in.ID, in.Name, rules, in.FinalAction, in.Enabled, in.Format).Scan(&found)
	}
	if err != nil {
		return in, mapError(err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM subscription_nodes WHERE subscription_id=$1", in.ID); err != nil {
		return in, err
	}
	for i, id := range in.NodeIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO subscription_nodes(organization_id,subscription_id,node_id,position) VALUES(request_org_id(),$1,$2,$3)`, in.ID, id, i); err != nil {
			return in, mapError(err)
		}
	}
	if err = subscriptionAudit(ctx, tx, in.ID, event); err != nil {
		return in, err
	}
	if err = scanSubscription(tx.QueryRow(ctx, "SELECT "+subscriptionColumns+" FROM subscriptions s WHERE s.id=$1", in.ID), &in); err != nil {
		return in, err
	}
	return in, tx.Commit(ctx)
}
func (s *Store) RotateSubscription(ctx context.Context, id, hash string, encrypted ...[]byte) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var ciphertext []byte
	if len(encrypted) > 0 {
		ciphertext = encrypted[0]
	}
	tag, err := tx.Exec(ctx, "UPDATE subscriptions SET token_hash=$2,encrypted_token=$3,updated_at=now() WHERE id=$1", id, hash, ciphertext)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err = subscriptionAudit(ctx, tx, id, "subscription_token_rotated"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) DeleteSubscription(ctx context.Context, id string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "DELETE FROM subscriptions WHERE id=$1", id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err = subscriptionAudit(ctx, tx, id, "subscription_deleted"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) SubscriptionContent(ctx context.Context, id, hash string) (Subscription, []Deployment, error) {
	var sub Subscription
	nodes := []Deployment{}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return sub, nodes, err
	}
	defer tx.Rollback(ctx)
	var org string
	if err = tx.QueryRow(ctx, `SELECT subscription_context($1,$2)::text`, id, hash).Scan(&org); err != nil {
		return sub, nodes, ErrNotFound
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", org); err != nil {
		return sub, nodes, err
	}
	if err = setScope(ctx, tx, "", org); err != nil {
		return sub, nodes, err
	}
	if _, err = tx.Exec(ctx, `SELECT set_config('app.subscription_id',$1,true),set_config('app.subscription_hash',$2,true)`, id, hash); err != nil {
		return sub, nodes, err
	}
	if err = scanSubscription(tx.QueryRow(ctx, "SELECT "+subscriptionColumns+" FROM subscriptions s WHERE s.id=$1 AND s.enabled AND s.token_hash=$2", id, hash), &sub); err != nil {
		return sub, nodes, mapError(err)
	}
	rows, err := tx.Query(ctx, `SELECT d.id::text,d.host_id::text,d.name,d.protocol,d.encrypted,h.address FROM subscription_nodes n JOIN protocol_deployments d ON d.id=n.node_id AND d.organization_id=n.organization_id JOIN hosts h ON h.id=d.host_id AND h.organization_id=d.organization_id WHERE n.subscription_id=$1 AND d.installed_at IS NOT NULL AND d.encrypted IS NOT NULL AND ((d.state='succeeded' AND d.action='deploy') OR (d.action='restart' AND d.state IN ('queued','running','failed','interrupted'))) ORDER BY n.position`, id)
	if err != nil {
		return sub, nodes, err
	}
	for rows.Next() {
		var d Deployment
		d.OrgID = org
		if err = rows.Scan(&d.ID, &d.HostID, &d.Name, &d.Protocol, &d.Encrypted, &d.Server); err != nil {
			rows.Close()
			return sub, nodes, err
		}
		nodes = append(nodes, d)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return sub, nodes, err
	}
	return sub, nodes, tx.Commit(ctx)
}
