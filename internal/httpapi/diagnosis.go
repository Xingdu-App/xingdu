package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"runtime"
	"time"
	"xingdu.app/xingdu/internal/probe"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
)

func (a *api) diagnosisRoutes(mux *http.ServeMux) {
	slots := make(chan struct{}, 2)
	mux.HandleFunc("POST /api/v1/hosts/{id}/deployments/{deployment}/diagnosis", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !validDeploymentID(w, r) {
			return
		}
		if !a.attempts.allow("diagnosis:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		// Authorization and tenant isolation precede every network operation.
		d, err := a.store.DiagnosticDeployment(r.Context(), r.PathValue("id"), r.PathValue("deployment"))
		if err != nil {
			storeError(w, err)
			return
		}
		if a.vault == nil {
			failure(w, 503, "credential_key_required", "部署者尚未配置凭据加密密钥")
			return
		}
		plain, err := a.vault.Open(d.Encrypted, deploymentAAD(storage.TenantOrg(r.Context()), d.HostID, d.ID))
		if err != nil {
			storeError(w, storage.ErrConflict)
			return
		}
		defer clear(plain)
		var spec protocol.Spec
		if json.Unmarshal(plain, &spec) != nil || protocol.ValidateSpec(spec) != nil {
			storeError(w, storage.ErrConflict)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			failure(w, 429, "diagnosis_busy", "诊断正在进行，请稍后重试")
			return
		}
		nodes, err := a.store.Nodes(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		var node *storage.Node
		for i := range nodes {
			if nodes[i].ID == d.ID && nodes[i].HostID == d.HostID {
				node = &nodes[i]
				break
			}
		}
		if node == nil {
			storeError(w, storage.ErrNotFound)
			return
		}
		if node.Revision != d.Revision {
			storeError(w, storage.ErrConflict)
			return
		}
		binary := ""
		if a.artifacts != "" && runtime.GOOS == "linux" {
			path := filepath.Join(a.artifacts, "official-sing-box-linux-"+runtime.GOARCH)
			if probe.VerifyRuntime(path) == nil {
				binary = path
			}
		}
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(30 * time.Second))
		ctx, cancel := context.WithTimeout(r.Context(), 23*time.Second)
		defer cancel()
		report := probe.Diagnose(ctx, d.Server, spec, binary)
		// Do not persist a port/TLS check as a successful protocol probe.
		reply(w, 200, map[string]any{"data": map[string]any{"node_id": d.ID, "revision": node.Revision, "checked_at": time.Now().UTC(), "source": "control_plane", "agent_status": node.HostStatus, "service_status": node.ServiceStatus, "service_checked_at": node.ServiceCheckedAt, "network": report}})
	}))
}
