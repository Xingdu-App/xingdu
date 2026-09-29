package storage

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"xingdu.app/xingdu/internal/billing"
)

const billingColumns = "organization_id::text,coalesce(customer_id,''),attempt,checkout_id,checkout_interval,subscription_id,status,interval,period_end,cancel_at_period_end"

func scanBilling(row scanner) (billing.Record, error) {
	var r billing.Record
	err := row.Scan(&r.OrganizationID, &r.CustomerID, &r.Attempt, &r.CheckoutID, &r.CheckoutInterval, &r.SubscriptionID, &r.Status, &r.Interval, &r.PeriodEnd, &r.CancelAtPeriodEnd)
	return r, mapError(err)
}
func (s *Store) Billing(ctx context.Context) (billing.Record, error) {
	tx, _, err := s.tenantTx(ctx, false, false)
	if err != nil {
		return billing.Record{}, err
	}
	defer tx.Rollback(ctx)
	r, err := scanBilling(tx.QueryRow(ctx, "SELECT "+billingColumns+" FROM organization_billing WHERE organization_id=request_org_id()"))
	if errors.Is(err, ErrNotFound) {
		return billing.Record{Status: "none"}, nil
	}
	return r, err
}

// EnsureBilling commits the stable idempotency seed before any Stripe mutation.
func (s *Store) EnsureBilling(ctx context.Context) error {
	tx, role, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if role != "owner" {
		return ErrForbidden
	}
	_, err = tx.Exec(ctx, "INSERT INTO organization_billing(organization_id,attempt) VALUES(request_org_id(),$1) ON CONFLICT(organization_id) DO NOTHING", NewID("bat"))
	if err != nil {
		return mapError(err)
	}
	return tx.Commit(ctx)
}

// MutateBilling serializes Stripe read/modify/reconcile operations across API
// replicas. Webhooks look up only their verified customer, then acquire the same
// organization lock as browser operations BEFORE fetching canonical Stripe state.
func (s *Store) MutateBilling(ctx context.Context, customer, event string, fn func(*billing.Record) error) error {
	var tx pgx.Tx
	var err error
	var org string
	if customer == "" {
		var role string
		tx, role, err = s.tenantTx(ctx, true, true)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if role != "owner" {
			return ErrForbidden
		}
		org = ctx.Value(scopeKey{}).(scope).Org
	} else {
		tx, err = s.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, "SELECT set_config('app.billing_customer',$1,true)", customer); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, "SELECT organization_id::text FROM organization_billing WHERE customer_id=$1", customer).Scan(&org); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", org); err != nil {
			return err
		}
		if event != "" {
			var exists bool
			if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM billing_events WHERE id=$1)", event).Scan(&exists); err != nil {
				return err
			}
			if exists {
				return nil
			}
		}
	}
	r, err := scanBilling(tx.QueryRow(ctx, "SELECT "+billingColumns+" FROM organization_billing WHERE organization_id=$1 FOR UPDATE", org))
	if err != nil {
		return err
	}
	if err = fn(&r); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE organization_billing SET customer_id=nullif($2,''),checkout_id=$3,checkout_interval=$4,subscription_id=$5,status=$6,interval=$7,period_end=$8,cancel_at_period_end=$9,updated_at=now() WHERE organization_id=$1`, org, r.CustomerID, r.CheckoutID, r.CheckoutInterval, r.SubscriptionID, r.Status, r.Interval, r.PeriodEnd, r.CancelAtPeriodEnd)
	if err != nil {
		return mapError(err)
	}
	if event != "" {
		if _, err = tx.Exec(ctx, "INSERT INTO billing_events(id,customer_id) VALUES($1,$2) ON CONFLICT(id) DO NOTHING", event, customer); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
