package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

// Runtime artifacts are verified against compiled-in hashes by managed agents.
// Modified upstream source is available alongside the hardened executables.
func (a *api) runtimeRoutes(mux *http.ServeMux) {
	slots := make(chan struct{}, 4)
	serve := func(w http.ResponseWriter, r *http.Request) {
		arch := r.PathValue("arch")
		if _, ok := protocol.RuntimeSHA256[arch]; !ok || a.artifacts == "" {
			http.NotFound(w, r)
			return
		}
		family := "sing-box-legacy"
		if r.PathValue("family") == "sing-box" {
			family = "sing-box"
		} else if r.PathValue("family") == "xray" {
			family = "xray"
		} else if r.PathValue("family") == "trusttunnel" {
			family = "trusttunnel"
		} else if r.PathValue("family") != "" {
			http.NotFound(w, r)
			return
		}
		if r.PathValue("provenance") == "official" {
			if family != "sing-box" && family != "xray" {
				http.NotFound(w, r)
				return
			}
			family = "official-" + family
		}
		path := filepath.Join(a.artifacts, family+"-linux-"+arch)
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
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(protocol.RuntimeResponseTimeout))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, path)
	}
	mux.HandleFunc("GET /api/v1/agent/runtime-source/{family}", func(w http.ResponseWriter, r *http.Request) {
		family := r.PathValue("family")
		if a.artifacts == "" || (family != "sing-box" && family != "xray") {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(a.artifacts, family+"-source.tar.gz")
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
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(protocol.RuntimeResponseTimeout))
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+family+`-source.tar.gz"`)
		http.ServeFile(w, r, path)
	})
	mux.HandleFunc("GET /api/v1/agent/runtime/official/{family}/{arch}", func(w http.ResponseWriter, r *http.Request) { r.SetPathValue("provenance", "official"); serve(w, r) })
	mux.HandleFunc("GET /api/v1/agent/runtime/{arch}", serve)
	mux.HandleFunc("GET /api/v1/agent/runtime/{family}/{arch}", serve)
}
