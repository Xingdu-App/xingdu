//go:build !linux

package agent

import (
	"context"
	"errors"
)

func RunTrustTunnel(context.Context, []string) error { return errors.New("TrustTunnel requires Linux") }
