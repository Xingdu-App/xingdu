package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSelfUpdateMachineRoutesRejectBrowserIdentity(t *testing.T) {
	h := New(nil, Options{})
	for _, action := range []string{"claim", "check", "failed", "applied"} {
		for _, header := range []string{"Origin", "Cookie"} {
			req := httptest.NewRequest("POST", "/api/v1/agent/update/"+action, strings.NewReader("{}"))
			req.Header.Set(header, "browser")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 64))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != 403 || !strings.Contains(w.Body.String(), "agent_only") {
				t.Fatalf("%s %s: %d", action, header, w.Code)
			}
		}
		req := httptest.NewRequest("POST", "/api/v1/agent/update/"+action, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("unauthenticated %s: %d", action, w.Code)
		}
	}
}
