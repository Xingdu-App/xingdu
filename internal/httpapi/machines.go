package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"xingdu.app/xingdu/internal/bootstrap"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/storage"
)

type MachineStore interface {
	MachineTarget(context.Context, string) (machine.Target, error)
	MachineState(context.Context, string) (storage.MachineState, error)
	Enrollment(context.Context, string, string, string) (time.Time, error)
	EnrollAgent(context.Context, string, string, string) error
	Heartbeat(context.Context, string, machine.Metrics) error
	RevokeMachine(context.Context, string) error
	SavedCredential(context.Context, string) (machine.SavedCredential, error)
	DeleteCredential(context.Context, string) error
	QueueInstall(context.Context, machine.Job, string, *machine.SavedCredential, machine.Target) error
}

func (a *api) machineRoutes(mux *http.ServeMux) {
	a.runtimeRoutes(mux)
	mux.HandleFunc("POST /api/v1/agent/enroll", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Token      string `json:"token"`
			Credential string `json:"credential"`
			Mode       string `json:"mode"`
		}
		if !decode(w, r, &in) {
			return
		}
		if !machine.ValidToken(in.Token) || !machine.ValidToken(in.Credential) || !machine.ValidMode(in.Mode) {
			failure(w, 401, "invalid_enrollment", "机器注册信息无效")
			return
		}
		if err := a.store.EnrollAgent(r.Context(), machine.Hash(in.Token), machine.Hash(in.Credential), in.Mode); err != nil {
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrForbidden) || errors.Is(err, storage.ErrConflict) {
				failure(w, 401, "invalid_enrollment", "注册令牌已过期、已使用或已撤销")
			} else {
				storeError(w, err)
			}
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /api/v1/agent/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || !machine.ValidToken(token) {
			failure(w, 401, "invalid_agent", "机器凭据无效")
			return
		}
		var in machine.Metrics
		if !decode(w, r, &in) {
			return
		}
		if in.Validate() != nil {
			failure(w, 422, "invalid_metrics", "探针数据无效")
			return
		}
		if err := a.store.Heartbeat(r.Context(), machine.Hash(token), in); err != nil {
			if errors.Is(err, storage.ErrNotFound) || errors.Is(err, storage.ErrForbidden) {
				failure(w, 401, "invalid_agent", "机器凭据已撤销")
			} else {
				storeError(w, err)
			}
			return
		}
		reply(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /api/v1/agent/download/{arch}", func(w http.ResponseWriter, r *http.Request) {
		arch := r.PathValue("arch")
		if arch != "amd64" && arch != "arm64" {
			http.NotFound(w, r)
			return
		}
		if a.artifacts == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, filepath.Join(a.artifacts, "xingdu-agent-linux-"+arch))
	})
	mux.HandleFunc("GET /api/v1/agent/install.sh", a.installer)
	mux.HandleFunc("GET /api/v1/hosts/{id}/machine", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		out, err := a.store.MachineState(r.Context(), r.PathValue("id"))
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	mux.HandleFunc("POST /api/v1/hosts/{id}/enrollment", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		var in struct {
			Mode          string `json:"mode"`
			ConfirmManage bool   `json:"confirm_manage"`
		}
		if !decode(w, r, &in) {
			return
		}
		if !machine.ValidMode(in.Mode) || (in.Mode == "manage" && !in.ConfirmManage) {
			failure(w, 422, "mode_confirmation_required", "请选择模式并确认托管权限")
			return
		}
		token := machine.Token()
		expires, err := a.store.Enrollment(r.Context(), r.PathValue("id"), in.Mode, machine.Hash(token))
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 201, map[string]any{"data": map[string]any{"token": token, "expires_at": expires, "origin": a.agentOrigin, "command": "curl -fsS '" + a.agentOrigin + "/api/v1/agent/install.sh' -o xingdu-install.sh && sudo sh xingdu-install.sh " + in.Mode}})
	}))
	mux.HandleFunc("DELETE /api/v1/hosts/{id}/agent", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if err := a.store.RevokeMachine(r.Context(), r.PathValue("id")); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("DELETE /api/v1/hosts/{id}/credential", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if err := a.store.DeleteCredential(r.Context(), r.PathValue("id")); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST /api/v1/hosts/{id}/ssh/fingerprint", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if !a.attempts.allow("ssh:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		target, err := a.store.MachineTarget(r.Context(), r.PathValue("id"))
		if err != nil {
			storeError(w, err)
			return
		}
		select {
		case a.connectionSlots <- struct{}{}:
			defer func() { <-a.connectionSlots }()
		default:
			failure(w, 429, "busy", "连接检查繁忙")
			return
		}
		fp, err := a.connector.Inspect(r.Context(), target)
		if err != nil {
			failure(w, 422, err.Error(), "无法检查 SSH 主机指纹，请检查地址、网络和访问策略")
			return
		}
		reply(w, 200, map[string]any{"data": map[string]string{"fingerprint": fp}})
	}))
	mux.HandleFunc("POST /api/v1/hosts/{id}/ssh/install", a.tenant(a.queueInstall))
}
func (a *api) queueInstall(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
	if !validID(w, r) {
		return
	}
	if a.vault == nil {
		failure(w, 503, "credential_key_required", "部署者尚未配置凭据加密密钥")
		return
	}
	if !strings.HasPrefix(a.agentOrigin, "https://") {
		failure(w, 422, "public_https_origin_required", "远程安装需要机器可访问的 HTTPS 控制端地址")
		return
	}
	if !a.attempts.allow("ssh:" + u.ID) {
		failure(w, 429, "rate_limited", "请稍后重试")
		return
	}
	id := r.PathValue("id")
	target, err := a.store.MachineTarget(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	var in struct {
		Method             string `json:"method"`
		Password           string `json:"password"`
		PrivateKey         string `json:"private_key"`
		Passphrase         string `json:"passphrase"`
		Fingerprint        string `json:"fingerprint"`
		Mode               string `json:"mode"`
		Retain             bool   `json:"retain"`
		UseSaved           bool   `json:"use_saved"`
		ConfirmManage      bool   `json:"confirm_manage"`
		ConfirmFingerprint bool   `json:"confirm_fingerprint"`
	}
	if !decodeLimit(w, r, &in, 40*1024) {
		return
	}
	if !machine.ValidMode(in.Mode) || (in.Mode == "manage" && !in.ConfirmManage) || !in.ConfirmFingerprint {
		failure(w, 422, "confirmation_required", "请确认主机指纹及所选权限模式")
		return
	}
	secret := machine.Secret{Target: target, Method: in.Method, Password: in.Password, PrivateKey: in.PrivateKey, Passphrase: in.Passphrase, Fingerprint: in.Fingerprint, Mode: in.Mode}
	org := storage.TenantOrg(r.Context())
	if in.UseSaved {
		saved, err := a.store.SavedCredential(r.Context(), id)
		if err != nil {
			storeError(w, err)
			return
		}
		plain, err := a.vault.Open(saved.Encrypted, machine.AAD(org, id, "saved"))
		if err != nil {
			failure(w, 503, "credential_unavailable", "无法解密凭据，请检查密钥或重新保存")
			return
		}
		defer clear(plain)
		if json.Unmarshal(plain, &secret) != nil || secret.Target != target {
			failure(w, 409, "credential_target_changed", "SSH 连接信息已改变，请重新提供凭据")
			return
		}
		secret.Mode = in.Mode
		if secret.Fingerprint != in.Fingerprint {
			failure(w, 409, "host_key_changed", "保存的主机指纹不匹配，请重新核实并提供凭据")
			return
		}
	}
	if err = bootstrap.ValidateSecret(secret); err != nil {
		failure(w, 422, err.Error(), "SSH 凭据或主机指纹格式不正确")
		return
	}
	var saved *machine.SavedCredential
	if in.Retain && !in.UseSaved {
		plain, _ := json.Marshal(secret)
		saved = &machine.SavedCredential{Method: secret.Method, Fingerprint: secret.Fingerprint, Encrypted: a.vault.Seal(plain, machine.AAD(org, id, "saved"))}
		clear(plain)
	}
	secret.EnrollmentToken = machine.Token()
	j := machine.Job{ID: storage.NewID("job"), HostID: id, Mode: in.Mode}
	plain, _ := json.Marshal(secret)
	j.Encrypted = a.vault.Seal(plain, machine.AAD(org, id, j.ID))
	clear(plain)
	if err = a.store.QueueInstall(r.Context(), j, machine.Hash(secret.EnrollmentToken), saved, target); err != nil {
		storeError(w, err)
		return
	}
	reply(w, 202, map[string]any{"data": map[string]string{"id": j.ID, "state": "queued"}})
}
func (a *api) installer(w http.ResponseWriter, r *http.Request) {
	sums := map[string]string{}
	for _, arch := range []string{"amd64", "arm64"} {
		b, err := os.ReadFile(filepath.Join(a.artifacts, "xingdu-agent-linux-"+arch))
		if a.artifacts == "" || err != nil {
			failure(w, 503, "artifact_missing", "Agent 构建尚不可用")
			return
		}
		sum := sha256.Sum256(b)
		sums[arch] = hex.EncodeToString(sum[:])
	}
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	fmt.Fprintf(w, `#!/bin/sh
set -eu
mode=${1:-manage}
case "$mode" in monitor|manage) ;; *) echo 'Mode must be monitor or manage' >&2; exit 1;; esac
[ "$(id -u)" = 0 ] || { echo 'Run with sudo' >&2; exit 1; }
[ "$(uname -s)" = Linux ] || exit 1
case "$(uname -m)" in x86_64) arch=amd64; expected='%s';; aarch64|arm64) arch=arm64; expected='%s';; *) echo 'Unsupported architecture' >&2; exit 1;; esac
umask 077
mkdir -p -m 0755 /usr/local/bin
d=$(mktemp -d /usr/local/bin/.xingdu-download.XXXXXXXX)
trap 'rm -rf "$d"' EXIT HUP INT TERM
curl --fail --silent --show-error --max-time 120 '%s/api/v1/agent/download/'"$arch" -o "$d/agent"
printf '%%s  %%s\n' "$expected" "$d/agent" | sha256sum -c - >/dev/null
chmod 700 "$d/agent"
echo "Installing Xingdu in $mode mode. Existing installations are never overwritten."
"$d/agent" --install --server '%s' --mode "$mode"
`, sums["amd64"], sums["arm64"], a.agentOrigin, a.agentOrigin)
}
