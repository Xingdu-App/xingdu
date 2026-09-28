package httpapi

import (
	"context"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http/httptest"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/storage"
)

type operationsFake struct{ fakeStore }

func (operationsFake) OrganizationOperations(context.Context) (storage.Operations, error) {
	return storage.Operations{}, storage.ErrForbidden
}
func TestQuotaResponseRedactsDatabaseDetails(t *testing.T) {
	w := httptest.NewRecorder()
	storeError(w, &pgconn.PgError{Code: "P0004", Message: "private database details"})
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	if !strings.Contains(w.Body.String(), "quota_exceeded") || strings.Contains(w.Body.String(), "private") {
		t.Fatal("quota response missing or database details leaked")
	}
}
func TestOperationsRequiresOrganization(t *testing.T) {
	h := New(operationsFake{}, Options{PublicOrigin: "http://127.0.0.1:15173"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/organization/operations", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
