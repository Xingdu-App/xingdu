package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"xingdu.app/xingdu/internal/storage"
)

type Store interface {
	Ready(context.Context) error
	Hosts(context.Context) ([]storage.Host, error)
}

func New(store Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ready(r.Context()); err != nil {
			reply(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		reply(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/system", func(w http.ResponseWriter, r *http.Request) {
		ready := store.Ready(r.Context()) == nil
		reply(w, 200, map[string]any{"data": map[string]any{"name": "Xingdu", "version": "0.1.0-dev", "stage": "scaffold", "database_ready": ready, "capabilities": map[string]bool{"host_enrollment": false, "deployment": false, "subscription_export": false}}})
	})
	mux.HandleFunc("GET /api/v1/hosts", func(w http.ResponseWriter, r *http.Request) {
		hosts, err := store.Hosts(r.Context())
		if err != nil {
			reply(w, 503, map[string]any{"error": map[string]string{"code": "database_unavailable", "message": "服务器列表暂时不可用"}})
			return
		}
		reply(w, 200, map[string]any{"data": hosts})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
