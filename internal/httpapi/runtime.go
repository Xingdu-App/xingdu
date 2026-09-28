package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

// A separate, unmodified upstream executable is served from the build artifact
// folder. Managed agents verify their compiled-in hash before running it.
func (a *api) runtimeRoutes(mux *http.ServeMux) {
	slots := make(chan struct{}, 4)
	mux.HandleFunc("GET /api/v1/agent/runtime/{arch}", func(w http.ResponseWriter, r *http.Request) {
		arch := r.PathValue("arch")
		if _, ok := protocol.RuntimeSHA256[arch]; !ok || a.artifacts == "" {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(a.artifacts, "sing-box-linux-"+arch)
		st, err := os.Lstat(path)
		if err != nil || !st.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "artifact downloads busy", http.StatusTooManyRequests)
			return
		}
		// Runtime artifacts are larger than ordinary API responses. Keep the normal
		// API write deadline, but permit this bounded streaming transfer to finish.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(90 * time.Second))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, path)
	})
}
