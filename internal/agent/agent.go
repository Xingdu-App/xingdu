package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"xingdu.app/xingdu/internal/machine"
)

type Config struct {
	Server          string `json:"server"`
	Token           string `json:"token"`
	EnrollmentToken string `json:"enrollment_token,omitempty"`
	Mode            string `json:"mode"`
}

var ErrRevoked = errors.New("agent authorization unavailable; request a new enrollment token")

func Load(path string) (Config, error) {
	var c Config
	st, e := os.Lstat(path)
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return c, errors.New("config must be a regular file with mode 0600")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return c, errors.New("config read failed")
	}
	defer clear(b)
	if e = json.Unmarshal(b, &c); e != nil {
		return c, errors.New("invalid config")
	}
	if e = machine.Origin(c.Server); e != nil {
		return c, e
	}
	if !machine.ValidToken(c.Token) || !machine.ValidMode(c.Mode) {
		return c, errors.New("invalid config")
	}
	return c, nil
}
func save(path string, c Config, exclusive bool) error {
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	defer clear(b)
	if exclusive {
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return errors.New("config already exists or is not writable")
		}
		_, e = f.Write(b)
		if e == nil {
			e = f.Sync()
		}
		ce := f.Close()
		if e != nil {
			return e
		}
		return ce
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".agent-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(name, path)
}
func request(ctx context.Context, c Config, path string, body any, auth bool) error {
	b, e := json.Marshal(body)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", c.Server+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := http.Client{Timeout: 12 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	res, e := client.Do(req)
	if e != nil {
		return errors.New("control plane unreachable")
	}
	defer res.Body.Close()
	io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode == 401 || res.StatusCode == 403 || res.StatusCode == 404 && path != "/api/v1/agent/deployments/result" {
		return ErrRevoked
	}
	if res.StatusCode != 200 {
		return errors.New("control plane rejected request")
	}
	return nil
}
func Initialize(ctx context.Context, path, server, mode, enrollment string) error {
	if e := machine.Origin(server); e != nil {
		return e
	}
	if !machine.ValidMode(mode) || !machine.ValidToken(enrollment) {
		return errors.New("invalid enrollment input")
	}
	c := Config{Server: server, Mode: mode, Token: machine.Token(), EnrollmentToken: enrollment}
	if e := save(path, c, true); e != nil {
		return e
	}
	return Sync(ctx, path, &c)
}
func Sync(ctx context.Context, path string, c *Config) error {
	// If the enrollment response was lost, the pre-persisted machine token still authenticates.
	err := request(ctx, *c, "/api/v1/agent/heartbeat", Collect(), true)
	if errors.Is(err, ErrRevoked) && c.EnrollmentToken != "" {
		err = request(ctx, *c, "/api/v1/agent/enroll", map[string]string{"token": c.EnrollmentToken, "credential": c.Token, "mode": c.Mode}, false)
		if err != nil {
			return err
		}
		err = request(ctx, *c, "/api/v1/agent/heartbeat", Collect(), true)
	}
	if err == nil && c.EnrollmentToken != "" {
		c.EnrollmentToken = ""
		return save(path, *c, false)
	}
	return err
}
func Run(ctx context.Context, path string) error {
	c, e := Load(path)
	if e != nil {
		return e
	}
	if c.Mode == "manage" && os.Geteuid() != 0 {
		return errors.New("managed agent must run as root")
	}
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	workerErrors := make(chan error, 1)
	var workerDone chan struct{}
	defer func() {
		cancel()
		if workerDone != nil {
			<-workerDone
		}
	}()
	for {
		e = Sync(ctx, path, &c)
		if errors.Is(e, ErrRevoked) {
			return e
		}
		if e == nil && c.Mode == "manage" && workerDone == nil {
			// Enrollment is complete; give the sequential worker an immutable copy.
			// Heartbeats remain independent even while an artifact is downloading.
			workerDone = make(chan struct{})
			config := c
			go func() {
				defer close(workerDone)
				for {
					if err := pollProtocols(workerCtx, config); errors.Is(err, ErrRevoked) {
						workerErrors <- err
						return
					}
					select {
					case <-workerCtx.Done():
						return
					case <-time.After(5 * time.Second):
					}
				}
			}()
		}
		select {
		case <-ctx.Done():
			return nil
		case err := <-workerErrors:
			return err
		case <-time.After(30 * time.Second):
		}
	}
}

func Collect() machine.Metrics {
	h, _ := os.Hostname()
	m := machine.Metrics{Hostname: h, OS: runtime.GOOS, Arch: runtime.GOARCH, Version: machine.Version, CPUs: runtime.NumCPU()}
	if b, e := os.ReadFile("/proc/uptime"); e == nil {
		v := strings.Fields(string(b))
		if len(v) > 0 {
			m.Uptime, _ = strconv.ParseFloat(v[0], 64)
		}
	}
	if b, e := os.ReadFile("/proc/loadavg"); e == nil {
		v := strings.Fields(string(b))
		if len(v) > 0 {
			m.Load1, _ = strconv.ParseFloat(v[0], 64)
		}
	}
	if b, e := os.ReadFile("/proc/meminfo"); e == nil {
		for line := range strings.SplitSeq(string(b), "\n") {
			v := strings.Fields(line)
			if len(v) < 2 {
				continue
			}
			n, _ := strconv.ParseUint(v[1], 10, 64)
			switch v[0] {
			case "MemTotal:":
				m.MemoryTotal = n * 1024
			case "MemAvailable:":
				m.MemoryAvailable = n * 1024
			}
		}
	}
	return m
}
