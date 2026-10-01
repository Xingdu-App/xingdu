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

func TestPublicRuntimeAcceptsMachineBearer(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "sing-box-linux-arm64"), []byte("runtime"), 0600); e != nil {
		t.Fatal(e)
	}
	h := New(nil, Options{ArtifactDir: dir})
	for _, method := range []string{"GET", "HEAD"} {
		req := httptest.NewRequest(method, "/api/v1/agent/runtime/arm64", nil)
		req.Header.Set("Authorization", "Bearer machine-credential")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatal("machine bearer incorrectly sent to API-key authentication", w.Code)
		}
	}
}

func TestTrustTunnelRuntimeArtifact(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trusttunnel-linux-arm64"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	a := &api{artifacts: dir}
	mux := http.NewServeMux()
	a.runtimeRoutes(mux)
	for path, status := range map[string]int{"/api/v1/agent/runtime/trusttunnel/arm64": 200, "/api/v1/agent/runtime/trusttunnel/mips": 404, "/api/v1/agent/runtime/other/arm64": 404} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != status {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}
