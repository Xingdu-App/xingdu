//go:build linux

package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"xingdu.app/xingdu/internal/protocol"
)

// RunTrustTunnel is invoked only by the installed runtime-named copy of the
// agent. Credentials never enter arguments, environment, logs or temporary files.
func RunTrustTunnel(ctx context.Context, args []string) error {
	fail := errors.New("TrustTunnel runtime failed")
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
	var cfg protocol.TrustTunnelConfig
	decoder := json.NewDecoder(io.LimitReader(input, 128<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cfg) != nil || cfg.Runtime != "trusttunnel" || len(cfg.Inbounds) != 1 || cfg.Inbounds[0].Port != cfg.Spec.Port {
		return fail
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return fail
	}
	data, err := protocol.TrustTunnelFiles(cfg.Spec)
	if err != nil {
		return fail
	}
	defer func() {
		for _, b := range data {
			clear(b)
		}
	}()
	self, err := os.Executable()
	if err != nil {
		return fail
	}
	binary := filepath.Join(filepath.Dir(self), "trusttunnel-endpoint-"+protocol.TrustTunnelVersion)
	st, err := os.Lstat(binary)
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0022 != 0 {
		return fail
	}
	b, err := os.ReadFile(binary)
	if err != nil {
		return fail
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != protocol.TrustTunnelSHA256[runtime.GOARCH] {
		return fail
	}
	files := []*os.File{}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()
	for _, b := range data {
		fd, err := unix.MemfdCreate("xingdu-runtime", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
		if err != nil {
			return fail
		}
		f := os.NewFile(uintptr(fd), "xingdu-runtime")
		files = append(files, f)
		if _, err = f.Write(b); err != nil {
			return fail
		}
		if _, err = f.Seek(0, 0); err != nil {
			return fail
		}
		if _, err = unix.FcntlInt(f.Fd(), unix.F_ADD_SEALS, unix.F_SEAL_SEAL|unix.F_SEAL_SHRINK|unix.F_SEAL_GROW|unix.F_SEAL_WRITE); err != nil {
			return fail
		}
	}
	endpointArgs := []string{"/proc/self/fd/3", "/proc/self/fd/4", "--loglvl", "info"}
	if args[0] == "check" {
		endpointArgs = append(endpointArgs, "--client_config", "xingdu", "--address", "127.0.0.1:443")
	}
	cmd := exec.CommandContext(ctx, binary, endpointArgs...)
	cmd.ExtraFiles = files
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	// Upstream config validation exports client credentials; discard all output.
	if err = cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fail
	}
	return nil
}
