package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeArtifactRoute(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sing-box-linux-arm64"), []byte("artifact fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	a := &api{artifacts: dir}
	mux := http.NewServeMux()
	a.runtimeRoutes(mux)
	for _, tc := range []struct {
		arch   string
		status int
	}{{"arm64", 200}, {"amd64", 404}, {"mips", 404}} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/agent/runtime/"+tc.arch, nil))
		if w.Code != tc.status {
			t.Fatalf("%s: status %d", tc.arch, w.Code)
		}
	}
	if err := os.Symlink(filepath.Join(dir, "sing-box-linux-arm64"), filepath.Join(dir, "sing-box-linux-amd64")); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/agent/runtime/amd64", nil))
	if w.Code != 404 {
		t.Fatal("symlink artifact accepted")
	}
}
