package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
)

type DeploymentStore interface {
	RecordProbe(context.Context, string, string, storage.ProbeReport) error
	Revisions(context.Context, string, string) ([]storage.Revision, error)
	RevisionSecret(context.Context, string, string, int) (storage.Deployment, error)
	UpdateDeployment(context.Context, storage.Deployment, []byte) error
	DeploymentPreflight(context.Context, string, int, string) error
	RestartDeployment(context.Context, string, string) error
	ReportServices(context.Context, string, []protocol.ServiceStatus) error
	ServiceInventory(context.Context, string) ([]string, error)
	Nodes(context.Context) ([]storage.Node, error)
	Deployments(context.Context, string) ([]storage.Deployment, error)
	QueueDeployment(context.Context, storage.Deployment) error
	DeploymentSecret(context.Context, string, string) (storage.Deployment, error)
	RemoveDeployment(context.Context, string, string) error
	ClaimDeployment(context.Context, string) (*storage.Deployment, error)
	FinishDeployment(context.Context, string, string, string, bool, string) error
}

func deploymentAAD(org, host, id string) string {
	return strings.Join([]string{"xingdu-protocol-v1", org, host, id}, ":")
}
func (a *api) deploymentRoutes(mux *http.ServeMux) {
	a.revisionRoutes(mux)
	mux.HandleFunc("POST /api/v1/hosts/{id}/deployments/{deployment}/probe", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validDeploymentID(w, r) {
			return
		}
		var in storage.ProbeReport
		if !decode(w, r, &in) {
			return
		}
		if err := a.store.RecordProbe(r.Context(), r.PathValue("id"), r.PathValue("deployment"), in); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("GET /api/v1/nodes", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		out, err := a.store.Nodes(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, http.StatusOK, map[string]any{"data": out})
	}))
	mux.HandleFunc("GET /api/v1/hosts/{id}/deployments", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		out, err := a.store.Deployments(r.Context(), r.PathValue("id"))
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	mux.HandleFunc("POST /api/v1/hosts/{id}/deployments/preflight", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if !a.attempts.allow("preflight:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		var in protocol.Input
		if !decodeLimit(w, r, &in, 64*1024) {
			return
		}
		if protocol.ValidateInput(in) != nil {
			failure(w, 422, "invalid_deployment", "协议、端口、TLS 域名或证书与私钥无效")
			return
		}
		if a.vault == nil {
			failure(w, 503, "credential_key_required", "部署者尚未配置凭据加密密钥")
			return
		}
		if err := a.store.DeploymentPreflight(r.Context(), r.PathValue("id"), in.Port, in.Protocol); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": map[string]any{"certificate_expires_at": protocol.CertificateExpiry(in.Certificate), "control_plane_ready": true, "port_check": "inventory_only", "external_connectivity": "not_checked"}})
	}))
	mux.HandleFunc("POST /api/v1/hosts/{id}/deployments", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if a.vault == nil {
			failure(w, 503, "credential_key_required", "部署者尚未配置凭据加密密钥")
			return
		}
		if !a.attempts.allow("deployment:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		var in struct {
			protocol.Input
			ConfirmInstall bool `json:"confirm_install"`
		}
		if !decodeLimit(w, r, &in, 64*1024) {
			return
		}
		if !in.ConfirmInstall {
			failure(w, 422, "confirmation_required", "请确认在服务器安装协议服务")
			return
		}
		spec, err := protocol.NewSpec(in.Input)
		if err != nil {
			failure(w, 422, "invalid_deployment", "协议、端口、TLS 域名或证书与私钥无效")
			return
		}
		d := storage.Deployment{ID: storage.NewID("node"), OperationID: storage.NewID("op"), HostID: r.PathValue("id"), Name: spec.Name, Protocol: spec.Protocol, Port: spec.Port, ServerName: spec.ServerName, CertificateExpiresAt: protocol.CertificateExpiry(spec.Certificate)}
		plain, _ := json.Marshal(spec)
		d.Encrypted = a.vault.Seal(plain, deploymentAAD(storage.TenantOrg(r.Context()), d.HostID, d.ID))
		clear(plain)
		if err = a.store.QueueDeployment(r.Context(), d); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 202, map[string]any{"data": map[string]string{"id": d.ID, "state": "queued"}})
	}))
	mux.HandleFunc("POST /api/v1/hosts/{id}/deployments/{deployment}/connection", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validDeploymentID(w, r) {
			return
		}
		if a.vault == nil {
			failure(w, 503, "credential_key_required", "凭据解密不可用")
			return
		}
		d, err := a.store.DeploymentSecret(r.Context(), r.PathValue("id"), r.PathValue("deployment"))
		if err != nil {
			storeError(w, err)
			return
		}
		var spec protocol.Spec
		if !a.openDeployment(w, d, storage.TenantOrg(r.Context()), &spec) {
			return
		}
		reply(w, 200, map[string]any{"data": map[string]any{"protocol": spec.Protocol, "server": d.Server, "port": spec.Port, "server_name": spec.ServerName, "credential": spec.Credential, "password": spec.Password, "username": protocol.Username(spec.Protocol), "cipher": protocol.Cipher(spec.Protocol), "certificate": spec.Certificate}})
	}))
	mux.HandleFunc("DELETE /api/v1/hosts/{id}/deployments/{deployment}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validDeploymentID(w, r) {
			return
		}
		if err := a.store.RemoveDeployment(r.Context(), r.PathValue("id"), r.PathValue("deployment")); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 202, map[string]any{"data": map[string]string{"id": r.PathValue("deployment"), "state": "queued"}})
	}))
	mux.HandleFunc("POST /api/v1/hosts/{id}/deployments/{deployment}/restart", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validDeploymentID(w, r) {
			return
		}
		var in struct {
			Confirm bool `json:"confirm"`
		}
		if !decode(w, r, &in) {
			return
		}
		if !in.Confirm {
			failure(w, 422, "confirmation_required", "重启会短暂中断连接，请确认")
			return
		}
		if err := a.store.RestartDeployment(r.Context(), r.PathValue("id"), r.PathValue("deployment")); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 202, map[string]any{"data": map[string]string{"state": "queued"}})
	}))
	mux.HandleFunc("GET /api/v1/agent/deployments/status", func(w http.ResponseWriter, r *http.Request) {
		hash, ok := deploymentIdentity(w, r)
		if !ok {
			return
		}
		ids, err := a.store.ServiceInventory(r.Context(), hash)
		if err != nil {
			deploymentAgentError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": ids})
	})
	mux.HandleFunc("POST /api/v1/agent/deployments/status", func(w http.ResponseWriter, r *http.Request) {
		hash, ok := deploymentIdentity(w, r)
		if !ok {
			return
		}
		var in struct {
			Reports []protocol.ServiceStatus `json:"reports"`
		}
		if !decodeLimit(w, r, &in, 16384) {
			return
		}
		if err := a.store.ReportServices(r.Context(), hash, in.Reports); err != nil {
			deploymentAgentError(w, err)
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /api/v1/agent/deployments/claim", func(w http.ResponseWriter, r *http.Request) {
		hash, ok := deploymentIdentity(w, r)
		if !ok {
			return
		}
		var in struct{}
		if !decode(w, r, &in) {
			return
		}
		d, err := a.store.ClaimDeployment(r.Context(), hash)
		if err != nil {
			deploymentAgentError(w, err)
			return
		}
		if d == nil {
			reply(w, 200, map[string]any{"data": nil})
			return
		}
		var spec protocol.Spec
		if (d.Action == "deploy" || d.Action == "update") && !a.openDeployment(w, *d, d.OrgID, &spec) {
			return
		}
		reply(w, 200, map[string]any{"data": protocol.Task{ID: d.OperationID, DeploymentID: d.ID, Lease: d.Lease, Action: d.Action, Spec: spec}})
	})
	mux.HandleFunc("POST /api/v1/agent/deployments/result", func(w http.ResponseWriter, r *http.Request) {
		hash, ok := deploymentIdentity(w, r)
		if !ok {
			return
		}
		var in protocol.Result
		if !decode(w, r, &in) {
			return
		}
		if !id.Valid("op", in.ID) || !id.Valid("lease", in.Lease) || !protocol.ValidResultCode(in.Code) {
			failure(w, 422, "invalid_result", "任务结果无效")
			return
		}
		if err := a.store.FinishDeployment(r.Context(), hash, in.ID, in.Lease, in.Success, in.Code); err != nil {
			deploymentAgentError(w, err)
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
	})
}
func (a *api) openDeployment(w http.ResponseWriter, d storage.Deployment, org string, spec *protocol.Spec) bool {
	if a.vault == nil {
		failure(w, 503, "credential_key_required", "凭据解密不可用")
		return false
	}
	plain, err := a.vault.Open(d.Encrypted, deploymentAAD(org, d.HostID, d.ID))
	if err != nil {
		failure(w, 503, "credential_unavailable", "无法解密部署配置")
		return false
	}
	defer clear(plain)
	if json.Unmarshal(plain, spec) != nil {
		failure(w, 503, "credential_unavailable", "部署配置不可用")
		return false
	}
	return true
}
func validDeploymentID(w http.ResponseWriter, r *http.Request) bool {
	if !validID(w, r) {
		return false
	}
	if !id.Valid("node", r.PathValue("deployment")) {
		failure(w, 400, "invalid_id", "部署 ID 无效")
		return false
	}
	return true
}
func deploymentIdentity(w http.ResponseWriter, r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	token := strings.TrimPrefix(header, "Bearer ")
	if !strings.HasPrefix(header, "Bearer ") || !machine.ValidToken(token) {
		failure(w, 401, "invalid_agent", "机器凭据无效")
		return "", false
	}
	return machine.Hash(token), true
}
func deploymentAgentError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrForbidden) {
		failure(w, 401, "invalid_agent_or_task", "机器身份或任务租约已失效")
		return
	}
	storeError(w, err)
}
