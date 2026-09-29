package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/storage"
)

func (a *api) selfUpdateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/hosts/{id}/agent/upgrade", a.tenant(func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		var in struct {
			Confirm bool `json:"confirm_upgrade"`
		}
		if !decode(w, r, &in) {
			return
		}
		if !in.Confirm {
			failure(w, 422, "confirmation_required", "请确认升级 Agent")
			return
		}
		if !a.attempts.allow("upgrade:" + u.ID) {
			failure(w, 429, "rate_limited", "请稍后重试")
			return
		}
		host := r.PathValue("id")
		j, err := a.store.UpgradeIdentity(r.Context(), host)
		if err != nil {
			storeError(w, err)
			return
		}
		if a.artifacts == "" || (j.Arch != "arm64" && j.Arch != "amd64") {
			failure(w, 409, "upgrade_unavailable", "升级包不可用")
			return
		}
		path := filepath.Join(a.artifacts, "xingdu-agent-linux-"+j.Arch)
		st, err := os.Lstat(path)
		if err != nil || !st.Mode().IsRegular() || st.Size() == 0 || st.Size() > 128<<20 {
			failure(w, 409, "upgrade_unavailable", "升级包不可用")
			return
		}
		binary, err := os.ReadFile(path)
		if err != nil {
			failure(w, 409, "upgrade_unavailable", "升级包不可用")
			return
		}
		digest := sha256.Sum256(binary)
		target, err := a.store.MachineTarget(r.Context(), host)
		if err != nil {
			storeError(w, err)
			return
		}
		j.ID, j.HostID, j.Action, j.Transport, j.TargetVersion, j.ArtifactSHA256 = storage.NewID("job"), host, "upgrade", "agent", machine.Version, hex.EncodeToString(digest[:])
		if err = a.store.QueueInstall(r.Context(), j, "", nil, target); err != nil {
			storeError(w, err)
			return
		}
		reply(w, 202, map[string]any{"data": map[string]string{"id": j.ID, "state": "queued"}})
	}))
	for _, action := range []string{"claim", "check", "failed", "applied"} {
		mux.HandleFunc("POST /api/v1/agent/update/"+action, func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			token := strings.TrimPrefix(auth, "Bearer ")
			if !strings.HasPrefix(auth, "Bearer ") || !machine.ValidToken(token) {
				failure(w, 401, "invalid_agent", "机器凭据无效")
				return
			}
			hash := machine.Hash(token)
			if action == "claim" {
				task, err := a.store.ClaimSelfUpdate(r.Context(), hash)
				if err != nil {
					storeError(w, err)
					return
				}
				reply(w, 200, map[string]any{"data": task})
				return
			}
			var in struct {
				ID    string `json:"id"`
				Lease string `json:"lease"`
			}
			if !decode(w, r, &in) {
				return
			}
			if !id.Valid("job", in.ID) || !id.Valid("lease", in.Lease) {
				failure(w, 422, "invalid_task", "任务无效")
				return
			}
			if err := a.store.CheckSelfUpdate(r.Context(), hash, in.ID, in.Lease, action); err != nil {
				storeError(w, err)
				return
			}
			reply(w, 200, map[string]bool{"ok": true})
		})
	}
}
