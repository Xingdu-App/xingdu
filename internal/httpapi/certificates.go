package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"xingdu.app/xingdu/internal/certificates"
	resourceid "xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
)

type CertificateStore interface {
	Certificates(context.Context) ([]storage.ManagedCertificate, int, error)
	CreatePlatformCertificate(context.Context, string, string, string) (storage.ManagedCertificate, error)
	CreateCertificate(context.Context, string, string) (storage.ManagedCertificate, error)
	QueueCertificate(context.Context, string, string) error
	DeleteCertificate(context.Context, string) error
	CertificateSecret(context.Context, string) (storage.ManagedCertificate, error)
}

func certificateError(w http.ResponseWriter, err error) {
	if errors.Is(err, storage.ErrCertificatePaid) {
		failure(w, 403, "paid_subscription_required", "自动证书需要有效的付费套餐")
		return
	}
	if errors.Is(err, storage.ErrCertificateQuota) {
		failure(w, 409, "certificate_quota_exceeded", "证书数量已达到套餐上限")
		return
	}
	storeError(w, err)
}

// Resolve managed material inside the tenant boundary; never return its key.
func (a *api) deploymentCertificate(w http.ResponseWriter, r *http.Request, id string, in *protocol.Input) (*storage.ManagedCertificate, bool) {
	if k, ok := r.Context().Value(apiPrincipal{}).(storage.APIKey); ok {
		allowed := false
		for _, scope := range k.Scopes {
			allowed = allowed || scope == "certificates:write"
		}
		if !allowed {
			failure(w, 403, "insufficient_scope", "托管证书部署还需要 certificates:write 权限")
			return nil, false
		}
	}
	if !resourceid.Valid("cert", id) || !in.NeedsCertificate() || in.Certificate != "" || in.PrivateKey != "" {
		failure(w, 422, "invalid_certificate", "请选择 TLS 协议及托管证书，不要同时提交 PEM")
		return nil, false
	}
	s, ok := a.store.(CertificateStore)
	if !ok || a.vault == nil {
		failure(w, 503, "certificates_unavailable", "证书管理不可用")
		return nil, false
	}
	c, err := s.CertificateSecret(r.Context(), id)
	if err != nil {
		certificateError(w, err)
		return nil, false
	}
	if c.Platform && c.HostID != r.PathValue("id") {
		failure(w, 422, "host_mismatch", "平台域名只能应用到绑定服务器")
		return nil, false
	}
	if in.ServerName != "" && in.ServerName != c.Domain {
		failure(w, 422, "invalid_certificate", "TLS 域名必须与托管证书一致")
		return nil, false
	}
	plain, err := a.vault.Open(c.Encrypted, storage.CertificateAAD(storage.TenantOrg(r.Context()), c.ID))
	if err != nil {
		failure(w, 503, "certificate_unavailable", "证书解密不可用")
		return nil, false
	}
	defer clear(plain)
	var bundle certificates.Bundle
	if json.Unmarshal(plain, &bundle) != nil || bundle.Directory != certificates.Production {
		failure(w, 422, "test_certificate", "测试 CA 证书不能应用到节点，请先签发正式证书")
		return nil, false
	}
	in.ServerName, in.Certificate, in.PrivateKey = c.Domain, bundle.Certificate, bundle.PrivateKey
	return &c, true
}
func (a *api) certificateRoutes(mux *http.ServeMux, cfg certificates.ManagedConfig) {
	// Optional interface keeps existing adapters usable; production Store implements it.
	get := func(w http.ResponseWriter) (CertificateStore, bool) {
		s, ok := a.store.(CertificateStore)
		if !ok {
			failure(w, 503, "certificates_unavailable", "证书管理不可用")
		}
		return s, ok
	}
	available := func(w http.ResponseWriter) bool {
		if !cfg.Configured() || a.vault == nil {
			failure(w, 503, "certificate_provider_unavailable", "运营者尚未配置自动证书服务")
			return false
		}
		return true
	}
	valid := func(w http.ResponseWriter, r *http.Request) bool {
		if !resourceid.Valid("cert", r.PathValue("certificate")) {
			failure(w, 404, "not_found", "证书不存在")
			return false
		}
		return true
	}
	mux.HandleFunc("GET /api/v1/certificates", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		configHeaders(w)
		s, ok := get(w)
		if !ok {
			return
		}
		out, limit, err := s.Certificates(r.Context())
		if err != nil {
			certificateError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": map[string]any{"certificates": out, "limit": limit, "configured": cfg.Configured() && a.vault != nil, "test_mode": cfg.Directory != certificates.Production, "platform_domains_configured": cfg.PlatformConfigured()}})
	}))
	mux.HandleFunc("POST /api/v1/certificates", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !a.attempts.allow("certificates:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		s, ok := get(w)
		if !ok {
			return
		}
		if !available(w) {
			return
		}
		var in struct {
			Domain   string `json:"domain"`
			HostID   string `json:"host_id"`
			Platform bool   `json:"platform"`
		}
		if !decode(w, r, &in) {
			return
		}
		var c storage.ManagedCertificate
		var err error
		if in.Platform {
			if !cfg.PlatformConfigured() {
				failure(w, 503, "platform_domain_unavailable", "平台域名尚未配置")
				return
			}
			if !resourceid.Valid("srv", in.HostID) || in.Domain != "" {
				failure(w, 422, "invalid_domain", "请选择服务器，平台域名自动生成")
				return
			}
			c, err = s.CreatePlatformCertificate(r.Context(), in.HostID, cfg.PlatformDomain, cfg.ValidationDomain)
		} else {
			c, err = s.CreateCertificate(r.Context(), in.Domain, cfg.ValidationDomain)
		}
		if err != nil {
			certificateError(w, err)
			return
		}
		configHeaders(w)
		reply(w, 201, map[string]any{"data": c})
	}))
	mux.HandleFunc("POST /api/v1/certificates/{certificate}/issue", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !valid(w, r) {
			return
		}
		s, ok := get(w)
		if !ok || !available(w) {
			return
		}
		if !a.attempts.allow("certificates:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		if err := s.QueueCertificate(r.Context(), r.PathValue("certificate"), cfg.Directory); err != nil {
			certificateError(w, err)
			return
		}
		reply(w, 202, map[string]any{"data": map[string]string{"state": "queued"}})
	}))
	mux.HandleFunc("DELETE /api/v1/certificates/{certificate}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !valid(w, r) {
			return
		}
		s, ok := get(w)
		if !ok {
			return
		}
		if err := s.DeleteCertificate(r.Context(), r.PathValue("certificate")); err != nil {
			certificateError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /api/v1/certificates/{certificate}/apply", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !a.attempts.allow("deployment:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		if !valid(w, r) {
			return
		}
		s, ok := get(w)
		if !ok {
			return
		}
		if a.vault == nil {
			failure(w, 503, "credential_key_required", "配置加密不可用")
			return
		}
		var in struct {
			HostID  string `json:"host_id"`
			NodeID  string `json:"node_id"`
			Confirm bool   `json:"confirm"`
		}
		if !decode(w, r, &in) {
			return
		}
		if !in.Confirm || !resourceid.Valid("srv", in.HostID) || !resourceid.Valid("node", in.NodeID) {
			failure(w, 422, "confirmation_required", "请选择节点并确认更新会短暂中断连接")
			return
		}
		c, err := s.CertificateSecret(r.Context(), r.PathValue("certificate"))
		if err != nil {
			certificateError(w, err)
			return
		}
		plain, err := a.vault.Open(c.Encrypted, storage.CertificateAAD(storage.TenantOrg(r.Context()), c.ID))
		if err != nil {
			failure(w, 503, "certificate_unavailable", "证书解密不可用")
			return
		}
		defer clear(plain)
		var bundle certificates.Bundle
		if json.Unmarshal(plain, &bundle) != nil || bundle.Directory != certificates.Production {
			failure(w, 422, "test_certificate", "测试 CA 证书不能应用到节点，请配置正式 CA 后重新签发")
			return
		}
		d, err := a.store.DeploymentSecret(r.Context(), in.HostID, in.NodeID)
		if err != nil {
			storeError(w, err)
			return
		}
		previous := d.Encrypted
		var spec protocol.Spec
		if !a.openDeployment(w, d, storage.TenantOrg(r.Context()), &spec) {
			return
		}
		if c.Platform && c.HostID != in.HostID {
			failure(w, 422, "host_mismatch", "平台域名只能应用到绑定服务器")
			return
		}
		if !spec.NeedsCertificate() {
			failure(w, 422, "tls_required", "此协议不使用托管 TLS 证书")
			return
		}
		spec.ServerName = c.Domain
		d.ServerName = c.Domain
		if spec.Relay != nil {
			failure(w, 422, "relay_certificate_unsupported", "中转节点证书更新需要协调维护，请先使用现有节点配置更新流程")
			return
		}
		spec.Certificate, spec.PrivateKey = bundle.Certificate, bundle.PrivateKey
		if protocol.ValidateSpec(spec) != nil {
			failure(w, 422, "invalid_certificate", "证书不适用于此节点协议")
			return
		}
		encoded, _ := json.Marshal(spec)
		d.Encrypted = a.vault.Seal(encoded, deploymentAAD(storage.TenantOrg(r.Context()), d.HostID, d.ID))
		clear(encoded)
		d.CertificateID, d.CertificateCipher = c.ID, c.Encrypted
		d.CertificateExpiresAt = protocol.CertificateExpiry(bundle.Certificate)
		if err = a.store.UpdateDeployment(r.Context(), d, previous); err != nil {
			certificateError(w, err)
			return
		}
		reply(w, 202, map[string]any{"data": map[string]string{"state": "queued", "node_id": d.ID}})
	}))
}
