package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

func WorkOnce(ctx context.Context, s *storage.Store, v *vault.Vault, c *Connector, origin string) error {
	if v == nil {
		return nil
	}
	claim, err := s.ClaimMachineJob(ctx)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	scoped := storage.WithTenant(ctx, claim.UserID, claim.OrgID)
	job, err := s.LoadMachineJob(scoped, claim)
	if err != nil {
		return s.FailUnauthorizedJob(ctx, claim)
	}
	state, result := "failed", "credential_unavailable"
	plain, err := v.Open(job.Encrypted, machine.AAD(job.OrgID, job.HostID, job.ID))
	if err == nil {
		defer clear(plain)
		var secret machine.Secret
		if json.Unmarshal(plain, &secret) == nil {
			target, e := s.MachineTarget(scoped, job.HostID)
			if e != nil {
				return s.FailUnauthorizedJob(ctx, claim)
			}
			if target != secret.Target {
				result = "target_changed"
			} else {
				op, cancel := context.WithTimeout(scoped, 100*time.Second)
				check := func() error {
					_, err := s.LoadMachineJob(op, claim)
					if err != nil {
						return err
					}
					target, err := s.MachineTarget(op, job.HostID)
					if err != nil {
						return err
					}
					if target != secret.Target {
						return storage.ErrConflict
					}
					return nil
				}
				if job.Action == "upgrade" {
					e = c.UpgradeChecked(op, secret, origin, job, check)
					if e == nil {
						wait, cancelWait := context.WithTimeout(op, 40*time.Second)
						for {
							if e = check(); e != nil {
								e = errors.New("authorization_revoked")
								break
							}
							ready, err := s.UpgradeReady(wait, job)
							if err != nil {
								e = errors.New("upgrade_unconfirmed")
								break
							}
							if ready {
								break
							}
							select {
							case <-wait.Done():
								e = errors.New("upgrade_unconfirmed")
							case <-time.After(time.Second):
							}
							if e != nil {
								break
							}
						}
						cancelWait()
					}
				} else {
					e = c.InstallChecked(op, secret, origin, check)
				}
				cancel()
				if e == nil {
					state, result = "installed", "service_installed"
					if job.Action == "upgrade" {
						result = "agent_upgraded"
					}
				} else {
					result = e.Error()
				}
			}
		}
	}
	finish, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finish = storage.WithTenant(finish, claim.UserID, claim.OrgID)
	if err = s.FinishMachineJob(finish, job, state, result); errors.Is(err, storage.ErrForbidden) {
		return s.FailUnauthorizedJob(finish, claim)
	}
	if err == nil && state == "failed" {
		slog.Warn("machine installation failed", "job_id", job.ID, "host_id", job.HostID,
			"action", job.Action, "result", result)
	}
	return err
}
