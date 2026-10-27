package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/storage"
)

type diagnosisBoundaryStore struct {
	keyBoundaryStore
	calls *int
}

func (s diagnosisBoundaryStore) DiagnosticDeployment(context.Context, string, string) (storage.Deployment, error) {
	*s.calls++
	return storage.Deployment{}, storage.ErrNotFound
}
func TestDiagnosisScopeAndTenantLookupPrecedeNetwork(t *testing.T) {
	for _, scope := range []string{"nodes:read", "nodes:credentials", "nodes:probe"} {
		calls := 0
		s := diagnosisBoundaryStore{keyBoundaryStore: keyBoundaryStore{key: storage.APIKey{ID: storage.NewID("key"), CreatedBy: storage.NewID("usr"), OrganizationID: storage.NewID("org"), Scopes: []string{scope}}}, calls: &calls}
		h := New(s, Options{})
		r := httptest.NewRequest("POST", "/api/v1/hosts/"+storage.NewID("srv")+"/deployments/"+storage.NewID("node")+"/diagnosis", strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer xd_key_"+strings.Repeat("a", 64))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		want := 403
		if scope == "nodes:probe" {
			want = 404
		}
		if w.Code != want || (calls == 1) != (scope == "nodes:probe") {
			t.Fatalf("scope %s status %d calls %d", scope, w.Code, calls)
		}
	}
}
