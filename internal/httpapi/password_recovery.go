package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"xingdu.app/xingdu/internal/emailverification"
	"xingdu.app/xingdu/internal/storage"
)

type recoveryStore interface {
	BeginPasswordRecovery(context.Context, string, string, string) error
	DeliverPasswordRecovery(context.Context, string) error
	CompletePasswordRecovery(context.Context, string, string, string) error
}

func (a *api) requestPasswordRecovery(w http.ResponseWriter, r *http.Request) {
	store, ok := a.store.(recoveryStore)
	sender, mailOK := a.emailSender.(emailverification.MessageSender)
	if !ok || !mailOK || !sender.Configured() {
		failure(w, 503, "email_unavailable", "邮件服务暂不可用")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.attempts.allowLimit("recovery-ip:"+ip, 5) || !a.attempts.allowLimit("recovery-global", 50) {
		failure(w, 429, "rate_limited", "请求过于频繁，请稍后重试")
		return
	}
	var in struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	email, valid := registrationEmail(in.Email)
	if !valid {
		failure(w, 422, "invalid_email", "请输入有效邮箱")
		return
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		failure(w, 503, "unavailable", "服务暂不可用")
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		failure(w, 503, "unavailable", "服务暂不可用")
		return
	}
	token := hex.EncodeToString(b)
	code := fmt.Sprintf("%08d", n.Int64())
	hash := tokenHash(token)
	if err = store.BeginPasswordRecovery(r.Context(), hash, email, tokenHash("recovery:"+token+":"+code)); err != nil {
		if errors.Is(err, storage.ErrRegistrationLimited) {
			failure(w, 429, "rate_limited", "请至少等待 60 秒后重试，每个邮箱每天最多请求 5 次")
		} else {
			storeError(w, err)
		}
		return
	}
	// Same generic code email for every valid address, including unknown accounts.
	if err = sender.SendMessage(r.Context(), email, emailverification.Message{Kind: "password_reset", Code: code}, "xingdu-recovery/"+hash); err != nil {
		failure(w, 503, "email_unavailable", "邮件暂时无法发送，请稍后重试")
		return
	}
	if err = store.DeliverPasswordRecovery(r.Context(), hash); err != nil {
		failure(w, 503, "email_unavailable", "验证码暂不可用，请重新申请")
		return
	}
	reply(w, 202, map[string]any{"data": map[string]any{"recovery_token": token, "expires_in": 600}})
}
func (a *api) completePasswordRecovery(w http.ResponseWriter, r *http.Request) {
	store, ok := a.store.(recoveryStore)
	if !ok {
		failure(w, 503, "unavailable", "服务暂不可用")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.attempts.allowLimit("recovery-verify:"+ip, 20) {
		failure(w, 429, "rate_limited", "请求过于频繁，请稍后重试")
		return
	}
	var in struct {
		Token    string `json:"recovery_token"`
		Code     string `json:"code"`
		Password string `json:"new_password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(in.Token) || !regexp.MustCompile(`^[0-9]{8}$`).MatchString(in.Code) || len(in.Password) < 12 || len(in.Password) > 72 {
		failure(w, 422, "invalid_recovery", "请检查验证码并使用 12–72 字节的新密码")
		return
	}
	select {
	case a.hashSlots <- struct{}{}:
		defer func() { <-a.hashSlots }()
	default:
		failure(w, 429, "rate_limited", "请稍后重试")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		failure(w, 503, "unavailable", "服务暂不可用")
		return
	}
	err = store.CompletePasswordRecovery(r.Context(), tokenHash(in.Token), tokenHash("recovery:"+in.Token+":"+in.Code), string(hash))
	if errors.Is(err, storage.ErrVerification) {
		failure(w, 422, "invalid_recovery", "验证码无效、已过期或账号不支持密码重置")
		return
	}
	if err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(204)
}
