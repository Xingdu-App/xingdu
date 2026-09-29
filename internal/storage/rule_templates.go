package storage

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"time"
	"xingdu.app/xingdu/internal/subscription"
)

type RuleTemplate struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Rules       []subscription.Rule `json:"rules"`
	FinalAction string              `json:"final_action"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

func scanRuleTemplate(row pgx.Row, t *RuleTemplate) error {
	var rules []byte
	if err := row.Scan(&t.ID, &t.Name, &rules, &t.FinalAction, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return mapError(err)
	}
	return json.Unmarshal(rules, &t.Rules)
}

const templateColumns = "id,name,rules,final_action,created_at,updated_at"

func (s *Store) RuleTemplates(ctx context.Context) ([]RuleTemplate, error) {
	out := []RuleTemplate{}
	tx, _, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT "+templateColumns+" FROM rule_templates ORDER BY created_at DESC,id")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t RuleTemplate
		if err = scanRuleTemplate(rows, &t); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	return out, tx.Commit(ctx)
}
func (s *Store) SaveRuleTemplate(ctx context.Context, in RuleTemplate, create bool) (RuleTemplate, error) {
	if err := (Subscription{Name: in.Name, Rules: in.Rules, FinalAction: in.FinalAction}).Validate(); err != nil {
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
	rules, _ := json.Marshal(in.Rules)
	if create {
		var n int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM rule_templates").Scan(&n); err != nil {
			return in, err
		}
		if n >= 100 {
			return in, ErrConflict
		}
		in.ID = NewID("obj")
		err = scanRuleTemplate(tx.QueryRow(ctx, "INSERT INTO rule_templates(id,organization_id,name,rules,final_action) VALUES($1,request_org_id(),$2,$3,$4) RETURNING "+templateColumns, in.ID, in.Name, rules, in.FinalAction), &in)
	} else {
		err = scanRuleTemplate(tx.QueryRow(ctx, "UPDATE rule_templates SET name=$2,rules=$3,final_action=$4,updated_at=now() WHERE id=$1 RETURNING "+templateColumns, in.ID, in.Name, rules, in.FinalAction), &in)
	}
	if err != nil {
		return in, err
	}
	return in, tx.Commit(ctx)
}
func (s *Store) DeleteRuleTemplate(ctx context.Context, id string) error {
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "DELETE FROM rule_templates WHERE id=$1", id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
