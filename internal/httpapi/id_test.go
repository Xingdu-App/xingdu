package httpapi

import (
	"net/http/httptest"
	"testing"
	"xingdu.app/xingdu/internal/id"
)

func TestResourcePathRequiresMatchingPrefix(t *testing.T) {
	for _, tc := range []struct{ path, prefix string }{
		{"/api/v1/hosts/", "srv"}, {"/api/v1/members/", "usr"}, {"/api/v1/invitations/", "inv"}, {"/api/v1/subscriptions/", "sub"},
	} {
		for _, candidate := range []string{id.New(tc.prefix), id.New("org"), "00000000-0000-4000-8000-000000000001"} {
			request := httptest.NewRequest("GET", tc.path+candidate, nil)
			request.SetPathValue("id", candidate)
			got := validID(httptest.NewRecorder(), request)
			if got != id.Valid(tc.prefix, candidate) {
				t.Fatal("resource type validation mismatch", tc.path)
			}
		}
	}
}
