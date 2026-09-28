package httpapi

import (
	"context"
	"net/http"
	"xingdu.app/xingdu/internal/storage"
)

type OperationsStore interface {
	OrganizationOperations(context.Context) (storage.Operations, error)
}

func (a *api) operationsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/organization/operations", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		data, err := a.store.OrganizationOperations(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": data})
	}))
}
