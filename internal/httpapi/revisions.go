package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
)

func (a *api) revisionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/hosts/{id}/deployments/{deployment}/revisions", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validDeploymentID(w, r) {
			return
		}
		out, err := a.store.Revisions(r.Context(), r.PathValue("id"), r.PathValue("deployment"))
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	mux.HandleFunc("PUT /api/v1/hosts/{id}/deployments/{deployment}", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !validDeploymentID(w, r) {
			return
		}
		if a.vault == nil {
			failure(w, 503, "credential_key_required", "配置加密不可用")
			return
		}
		if !a.attempts.allow("deployment:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		var in struct {
			Port             *int    `json:"port"`
			ExitNodeID       *string `json:"exit_node_id"`
			Name             string  `json:"name"`
			ServerName       string  `json:"server_name"`
			Certificate      string  `json:"certificate"`
			PrivateKey       string  `json:"private_key"`
			RotateCredential bool    `json:"rotate_credential"`
			Revision         int     `json:"restore_revision"`
			Confirm          bool    `json:"confirm"`
		}
		if !decodeLimit(w, r, &in, 64*1024) {
			return
		}
		if !in.Confirm {
			failure(w, 422, "confirmation_required", "更新会短暂中断连接，请确认")
			return
		}
		d, err := a.store.DeploymentSecret(r.Context(), r.PathValue("id"), r.PathValue("deployment"))
		if err != nil {
			storeError(w, err)
			return
		}
		previous := d.Encrypted
		var spec protocol.Spec
		if !a.openDeployment(w, d, storage.TenantOrg(r.Context()), &spec) {
			return
		}
		if in.Revision > 0 {
			historical, err := a.store.RevisionSecret(r.Context(), d.HostID, d.ID, in.Revision)
			if err != nil {
				storeError(w, err)
				return
			}
			if !a.openDeployment(w, historical, storage.TenantOrg(r.Context()), &spec) {
				return
			}
		} else {
			if in.Port != nil {
				spec.Port = *in.Port
			}
			if in.Name != "" {
				spec.Name = in.Name
			}
			if in.ServerName != "" {
				spec.ServerName = in.ServerName
			}
			if in.Certificate != "" || in.PrivateKey != "" {
				spec.Certificate = in.Certificate
				spec.PrivateKey = in.PrivateKey
			}
			if in.RotateCredential {
				fresh, err := protocol.NewSpec(spec.Input)
				if err != nil {
					failure(w, 422, "invalid_deployment", "配置无效")
					return
				}
				spec.Credential = fresh.Credential
				spec.Password = fresh.Password
			}
		}

		exitID := ""
		if spec.Relay != nil {
			exitID = spec.Relay.NodeID
		}
		if in.ExitNodeID != nil {
			exitID = *in.ExitNodeID
		}
		spec.Relay = nil
		if exitID != "" {
			peer, cipher, err := a.relayPeer(r.Context(), exitID, d.HostID)
			if err != nil {
				storeError(w, err)
				return
			}
			spec.Relay = peer
			d.RelayExitID = peer.NodeID
			d.ExitCipher = cipher
		}
		if protocol.ValidateSpec(spec) != nil {
			failure(w, 422, "invalid_deployment", "配置或证书无效，历史证书可能已过期")
			return
		}
		plain, _ := json.Marshal(spec)
		d.Encrypted = a.vault.Seal(plain, deploymentAAD(storage.TenantOrg(r.Context()), d.HostID, d.ID))
		clear(plain)
		d.Name = spec.Name
		d.Protocol = spec.Protocol
		d.Port = spec.Port
		d.ServerName = spec.ServerName
		d.CertificateExpiresAt = protocol.CertificateExpiry(spec.Certificate)
		if err = a.store.UpdateDeployment(r.Context(), d, previous); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 202, map[string]any{"data": map[string]string{"state": "queued"}})
	}))
}

func (a *api) relayPeer(ctx context.Context, node, entryHost string) (*protocol.Peer, []byte, error) {
	nodes, err := a.store.Nodes(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, n := range nodes {
		if n.ID != node {
			continue
		}
		if n.HostID == entryHost || n.RelayExitID != "" || n.State != "succeeded" || n.Action != "deploy" {
			return nil, nil, storage.ErrConflict
		}
		d, err := a.store.DeploymentSecret(ctx, n.HostID, n.ID)
		if err != nil {
			return nil, nil, err
		}
		plain, err := a.vault.Open(d.Encrypted, deploymentAAD(storage.TenantOrg(ctx), d.HostID, d.ID))
		if err != nil {
			return nil, nil, storage.ErrConflict
		}
		defer clear(plain)
		var spec protocol.Spec
		if json.Unmarshal(plain, &spec) != nil || spec.Relay != nil {
			return nil, nil, storage.ErrConflict
		}
		resolve, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		address, err := a.connector.ResolveTarget(resolve, d.Server)
		if err != nil {
			return nil, nil, storage.ErrInvalid
		}
		return &protocol.Peer{NodeID: n.ID, Address: address, Protocol: spec.Protocol, Port: spec.Port, ServerName: spec.ServerName, Certificate: spec.Certificate, Credential: spec.Credential, Password: spec.Password}, d.Encrypted, nil
	}
	return nil, nil, storage.ErrNotFound
}
