package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

func (x *protocolExecutor) owned(id string) bool {
	if !protocol.ValidID(id) {
		return false
	}
	dir := filepath.Join(x.stateDir, id)
	st, err := os.Lstat(dir)
	if err != nil || !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return false
	}
	owner, err := os.ReadFile(filepath.Join(dir, "owner"))
	if err != nil || string(owner) != id {
		return false
	}
	unitPath := filepath.Join(x.unitDir, serviceName(id))
	st, err = os.Lstat(unitPath)
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0022 != 0 {
		return false
	}
	unit, err := os.ReadFile(unitPath)
	return err == nil && strings.HasPrefix(string(unit), "# Xingdu managed deployment "+id+"\n")
}
func (x *protocolExecutor) serviceStatus(ctx context.Context, id string) string {
	if !x.owned(id) {
		return "missing"
	}
	err := x.run(ctx, "systemctl", "is-active", "--quiet", serviceName(id))
	if err == nil {
		return "active"
	}
	if ctx.Err() != nil {
		return "unknown"
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && (exitErr.ExitCode() == 3 || exitErr.ExitCode() == 4) {
		return "inactive"
	}
	return "unknown"
}
func (x *protocolExecutor) reportServices(ctx context.Context, c Config) error {
	// Fixed path under the exclusive worker lock throttles across worker restarts.
	stamp := filepath.Join(x.stateDir, ".status-reported")
	if st, err := os.Lstat(stamp); err == nil && time.Since(st.ModTime()) < 30*time.Second {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", c.Server+"/api/v1/agent/deployments/status", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	res, err := x.client.Do(req)
	if err != nil {
		return errors.New("service inventory unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("service inventory rejected")
	}
	var body struct {
		Data []string `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(&body) != nil || len(body.Data) > 100 {
		return errors.New("invalid service inventory")
	}
	reports := []protocol.ServiceStatus{}
	for _, id := range body.Data {
		if !protocol.ValidID(id) {
			return errors.New("invalid service identity")
		}
		reports = append(reports, protocol.ServiceStatus{ID: id, Status: x.serviceStatus(ctx, id)})
	}
	if err = request(ctx, c, "/api/v1/agent/deployments/status", struct {
		Reports []protocol.ServiceStatus `json:"reports"`
	}{reports}, true); err != nil {
		return err
	}
	return atomicProtocolFile(stamp, []byte("ok"), 0600)
}
