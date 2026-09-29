package httpapi

import (
	"net/http"
	"xingdu.app/xingdu/internal/storage"
)

func (a *api) ruleTemplateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/rule-templates", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		out, err := a.store.RuleTemplates(r.Context())
		if err != nil {
			storeError(w, err)
			return
		}
		reply(w, 200, map[string]any{"data": out})
	}))
	save := func(create bool) func(http.ResponseWriter, *http.Request, storage.User, string) {
		return func(w http.ResponseWriter, r *http.Request, u storage.User, _ string) {
			if !create && !validID(w, r) {
				return
			}
			if !a.attempts.allow("rule-template-write:" + u.ID) {
				failure(w, 429, "rate_limited", "请稍后重试")
				return
			}
			var in storage.RuleTemplate
			if !decodeLimit(w, r, &in, 64*1024) {
				return
			}
			in.ID = r.PathValue("id")
			out, err := a.store.SaveRuleTemplate(r.Context(), in, create)
			if err != nil {
				storeError(w, err)
				return
			}
			status := 200
			if create {
				status = 201
			}
			reply(w, status, map[string]any{"data": out})
		}
	}
	mux.HandleFunc("POST /api/v1/rule-templates", a.tenant(save(true)))
	mux.HandleFunc("PUT /api/v1/rule-templates/{id}", a.tenant(save(false)))
	mux.HandleFunc("DELETE /api/v1/rule-templates/{id}", a.tenant(func(w http.ResponseWriter, r *http.Request, _ storage.User, _ string) {
		if !validID(w, r) {
			return
		}
		if err := a.store.DeleteRuleTemplate(r.Context(), r.PathValue("id")); err != nil {
			storeError(w, err)
			return
		}
		w.WriteHeader(204)
	}))
}
