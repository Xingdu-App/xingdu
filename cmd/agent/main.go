package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"xingdu.app/xingdu/internal/agent"
	"xingdu.app/xingdu/internal/machine"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if !errors.Is(err, agent.ErrRevoked) {
			os.Exit(1)
		}
	}
}
func run() error {
	if strings.HasPrefix(filepath.Base(os.Args[0]), "trusttunnel-") {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return agent.RunTrustTunnel(ctx, os.Args[1:])
	}
	selfUpdate := flag.Bool("self-update", false, "check and execute an authorized update (root updater service only)")
	version := flag.Bool("version", false, "print version")
	upgrade := flag.Bool("upgrade", false, "upgrade an existing systemd installation using an expectation from stdin")
	initialize := flag.Bool("init", false, "enroll using a token read from stdin")
	install := flag.Bool("install", false, "install a Linux systemd service (root required)")
	config := flag.String("config", "/var/lib/xingdu-agent/agent.json", "private configuration path")
	server := flag.String("server", "", "HTTPS control-plane origin")
	mode := flag.String("mode", "manage", "manage or monitor")
	flag.Parse()
	if *version {
		fmt.Println("xingdu-agent " + machine.Version)
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *selfUpdate {
		return agent.SelfUpdate(ctx)
	}
	if *upgrade {
		var expected agent.UpgradeExpectation
		if json.NewDecoder(io.LimitReader(os.Stdin, 4096)).Decode(&expected) != nil {
			return errors.New("invalid upgrade request")
		}
		return agent.Upgrade(ctx, expected)
	}
	if *initialize || *install {
		var token []byte
		var err error
		if term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprint(os.Stderr, "Enrollment token (hidden): ")
			token, err = term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
		} else {
			token, err = io.ReadAll(io.LimitReader(os.Stdin, 66))
		}
		if err != nil {
			return errors.New("could not read enrollment token")
		}
		defer clear(token)
		ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		if *install {
			return agent.Install(ctx, *server, *mode, strings.TrimSpace(string(token)))
		}
		return agent.Initialize(ctx, *config, *server, *mode, strings.TrimSpace(string(token)))
	}
	return agent.Run(ctx, *config)
}
