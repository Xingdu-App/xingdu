package bootstrap

import (
	"context"
	"errors"
	"net"
)

func (c *Connector) ResolveTarget(ctx context.Context, address string) (string, error) {
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", address)
	if err != nil || len(ips) == 0 {
		return "", errors.New("target_unavailable")
	}
	for _, ip := range ips {
		if !c.allowed(ip) {
			return "", errors.New("target_blocked")
		}
	}
	return ips[0].Unmap().String(), nil
}
