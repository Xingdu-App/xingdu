package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

func (x *protocolExecutor) xrayBinary(ctx context.Context, c Config) (string, error) {
	if _, err := x.runtimeBinary(ctx, c, "xray-core", protocol.XrayVersion, protocol.XrayLicense, protocol.XraySHA256, "/api/v1/agent/runtime/official/xray/"); err != nil {
		return "", err
	}
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(self)
	if err != nil {
		return "", err
	}
	path := filepath.Join(x.binaryDir, "xray-"+protocol.XrayVersion)
	if st, e := os.Lstat(path); e == nil && (!st.Mode().IsRegular() || st.Mode().Perm()&0022 != 0) {
		return "", errors.New("unsafe launcher")
	}
	if err = atomicProtocolFile(path, b, 0755); err != nil {
		return "", err
	}
	return path, nil
}

// RunXray never passes credentials through arguments, logs or the environment.
// The XHTTP frontend terminates HTTP versions only; Xray owns protocol/auth.
func RunXray(ctx context.Context, args []string) error {
	fail := errors.New("Xray runtime failed")
	if len(args) != 3 || (args[0] != "run" && args[0] != "check") || args[1] != "-c" {
		return fail
	}
	input := os.Stdin
	if args[2] != "stdin" {
		f, err := os.Open(args[2])
		if err != nil {
			return fail
		}
		defer f.Close()
		input = f
	}
	var cfg protocol.XrayConfig
	d := json.NewDecoder(io.LimitReader(input, 128<<10))
	d.DisallowUnknownFields()
	if d.Decode(&cfg) != nil || cfg.Runtime != "xray" || len(cfg.Inbounds) != 1 || cfg.Inbounds[0].Port != cfg.Spec.Port {
		return fail
	}
	if d.Decode(new(any)) != io.EOF || protocol.ValidateSpec(cfg.Spec) != nil {
		return fail
	}
	self, err := os.Executable()
	if err != nil {
		return fail
	}
	binary := filepath.Join(filepath.Dir(self), "xray-core-"+protocol.XrayVersion)
	st, err := os.Lstat(binary)
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0022 != 0 {
		return fail
	}
	b, err := os.ReadFile(binary)
	if err != nil {
		return fail
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != protocol.XraySHA256[runtime.GOARCH] {
		return fail
	}
	dir, err := os.MkdirTemp("", "xingdu-xhttp-")
	if err != nil {
		return fail
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "transport.sock")
	config, err := protocol.XrayServer(cfg.Spec, socket)
	if err != nil {
		return fail
	}
	defer clear(config)
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	command := []string{"run", "-config", "stdin:"}
	if args[0] == "check" {
		command = append(command, "-test")
	}
	child := exec.CommandContext(childCtx, binary, command...)
	child.Stdin = bytes.NewReader(config)
	child.Env = []string{"PATH=/usr/bin:/bin"}
	if args[0] == "check" {
		if child.Run() != nil {
			return fail
		}
		return nil
	}
	if err = child.Start(); err != nil {
		return fail
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	if cfg.Spec.V2Ray == nil || cfg.Spec.V2Ray.Network != "xhttp" {
		select {
		case <-ctx.Done():
			cancel()
			<-done
			return nil
		case err = <-done:
			if err != nil {
				return fail
			}
			return nil
		}
	}
	// Wait for the private listener before accepting public traffic.
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	ready := false
	for !ready {
		select {
		case <-ticker.C:
			if st, e := os.Lstat(socket); e == nil && st.Mode()&os.ModeSocket != 0 {
				ready = true
			}
		case <-deadline.C:
			cancel()
			<-done
			return fail
		case <-ctx.Done():
			cancel()
			<-done
			return nil
		case <-done:
			return fail
		}
	}
	frontCtx, stop := context.WithCancel(ctx)
	defer stop()
	front := make(chan error, 1)
	go func() { front <- serveXHTTP(frontCtx, cfg.Spec, socket) }()
	select {
	case <-ctx.Done():
		stop()
		cancel()
		<-front
		<-done
		return nil
	case <-done:
		stop()
		<-front
		return fail
	case <-front:
		cancel()
		<-done
		return fail
	}
}
