package httpapi

import (
	"context"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"xingdu.app/xingdu/internal/storage"
)

type OwnershipStore interface {
	TransferOwnership(context.Context, string, string, string) error
}

func (a *api) transferOwnership(w http.ResponseWriter, r *http.Request, u storage.User, token string) {
	if !a.attempts.allowLimit("ownership:"+u.ID, 5) {
		failure(w, 429, "rate_limited", "请稍后再试")
		return
	}
	var in struct {
		Target   string `json:"target_id"`
		Password string `json:"current_password"`
		Confirm  string `json:"confirmation"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Target) != 36 || strings.Trim(in.Target, "0123456789abcdef-") != "" || len(in.Password) > 72 || in.Confirm != "TRANSFER" {
		failure(w, 422, "invalid_transfer", "请选择已有成员并确认所有权转移")
		return
	}
	select {
	case a.hashSlots <- struct{}{}:
		defer func() { <-a.hashSlots }()
	default:
		failure(w, 429, "rate_limited", "请稍后再试")
		return
	}
	credentials, err := a.store.Credentials(r.Context(), u.Username)
	if err != nil {
		storeError(w, err)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(in.Password)) != nil {
		failure(w, 422, "invalid_password", "当前密码不正确")
		return
	}
	if err = a.store.TransferOwnership(r.Context(), in.Target, credentials.PasswordHash, tokenHash(token)); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(204)
}
