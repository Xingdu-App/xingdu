// An independently deployed, organization-scoped probe. Configuration and the
// revocable API key are supplied only through the process environment.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"xingdu.app/xingdu/internal/bootstrap"
	"xingdu.app/xingdu/internal/probe"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "probe stopped: configuration or control-plane failure")
		os.Exit(1)
	}
}
func run() error {
	origin, key, org, binary, endpoint := os.Getenv("XINGDU_PROBE_API"), os.Getenv("XINGDU_PROBE_API_KEY"), os.Getenv("XINGDU_PROBE_ORG"), os.Getenv("XINGDU_PROBE_RUNTIME"), os.Getenv("XINGDU_PROBE_ENDPOINT")
	u, e := url.Parse(origin)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || key == "" || org == "" {
		return fmt.Errorf("invalid configuration")
	}
	if e = probe.VerifyRuntime(binary); e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	client := http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(method, path string, in, out any) error {
		b, _ := json.Marshal(in)
		r, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(origin, "/")+path, bytes.NewReader(b))
		if e != nil {
			return e
		}
		r.Header.Set("Authorization", "Bearer "+key)
		r.Header.Set("X-Xingdu-Organization", org)
		r.Header.Set("Content-Type", "application/json")
		res, e := client.Do(r)
		if e != nil {
			return fmt.Errorf("request failed")
		}
		defer res.Body.Close()
		if res.StatusCode >= 300 {
			return fmt.Errorf("request rejected")
		}
		if out == nil {
			return nil
		}
		return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(out)
	}
	connector := &bootstrap.Connector{}
	for _, raw := range strings.Split(os.Getenv("XINGDU_PROBE_ALLOWED_CIDRS"), ",") {
		if raw == "" {
			continue
		}
		prefix, e := netip.ParsePrefix(strings.TrimSpace(raw))
		if e != nil {
			return e
		}
		connector.Allowed = append(connector.Allowed, prefix)
	}
	for {
		var nodes struct {
			Data []storage.Node `json:"data"`
		}
		if e = call("GET", "/api/v1/nodes", nil, &nodes); e != nil {
			return e
		}
		for _, n := range nodes.Data {
			if n.State != "succeeded" || n.Action != "deploy" {
				continue
			}
			path := "/api/v1/hosts/" + n.HostID + "/deployments/" + n.ID
			var connection struct {
				Data struct {
					Protocol, Server, Credential, Password, Certificate string
					Port                                                int
					ServerName                                          string `json:"server_name"`
				} `json:"data"`
			}
			if e = call("POST", path+"/connection", nil, &connection); e != nil {
				continue
			}
			c := connection.Data
			resolve, done := context.WithTimeout(ctx, 5*time.Second)
			address, e := connector.ResolveTarget(resolve, c.Server)
			done()
			report := storage.ProbeReport{Revision: n.Revision}
			if e == nil {
				ms, ip, err := probe.Run(ctx, binary, endpoint, protocol.Peer{Address: address, Protocol: c.Protocol, Port: c.Port, ServerName: c.ServerName, Certificate: c.Certificate, Credential: c.Credential, Password: c.Password})
				report.LatencyMS = ms
				report.ExitIP = ip
				report.OK = err == nil
			}
			if e = call("POST", path+"/probe", report, nil); e != nil {
				fmt.Fprintln(os.Stderr, "probe report rejected")
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Minute):
		}
	}
}
