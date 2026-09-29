package storage

import (
	"context"
	"net/netip"
)

type ProbeReport struct {
	OK        bool   `json:"ok"`
	LatencyMS int    `json:"latency_ms"`
	ExitIP    string `json:"exit_ip"`
	Revision  int    `json:"revision"`
}

func (s *Store) RecordProbe(ctx context.Context, host, node string, p ProbeReport) error {
	if p.LatencyMS < 0 || p.LatencyMS > 120000 {
		return ErrInvalid
	}
	if p.OK {
		ip, e := netip.ParseAddr(p.ExitIP)
		if e != nil || !ip.IsGlobalUnicast() || ip.IsPrivate() {
			return ErrInvalid
		}
	} else {
		p.ExitIP = ""
	}
	tx, _, err := s.tenantTx(ctx, true, true)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE protocol_deployments SET probe_ok=$3,probe_at=now(),probe_latency_ms=$4,probe_exit_ip=$5 WHERE id=$1 AND host_id=$2 AND revision=$6 AND state='succeeded' AND action='deploy'`, node, host, p.OK, p.LatencyMS, p.ExitIP, p.Revision)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return tx.Commit(ctx)
}
