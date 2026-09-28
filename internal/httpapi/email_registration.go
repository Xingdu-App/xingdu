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
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
	"xingdu.app/xingdu/internal/storage"
)

type EmailRegistrationStore interface {
	BeginEmailRegistration(context.Context, storage.PendingRegistration) error
	DeliverEmailRegistration(context.Context, string) error
	VerifyEmailRegistration(context.Context, string, string) (string, error)
}

func registrationEmail(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	a, err := mail.ParseAddress(value)
	if err != nil || a.Address != value || len(value) > 254 || strings.ContainsAny(value, "\r\n") || !strings.Contains(value, ".") {
		return "", false
	}
	for _, r := range value {
		if r > 127 {
			return "", false
		}
	}
	return value, true
}
func (a *api) emailReady() bool { return a.emailSender != nil && a.emailSender.Configured() }
func (a *api) register(w http.ResponseWriter, r *http.Request) {
	if !a.registration {
		failure(w, 403, "registration_disabled", "注册未开放")
		return
	}
	if !a.emailReady() {
		failure(w, 503, "email_unavailable", "邮箱验证服务尚未配置，请稍后再试")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.attempts.allowLimit("register-ip:"+ip, 5) || !a.attempts.allowLimit("register-global", 50) {
		failure(w, 429, "rate_limited", "注册过于频繁，请稍后再试")
		return
	}
	var in struct {
		Email        string `json:"email"`
		Password     string `json:"password"`
		Organization string `json:"organization"`
	}
	if !decode(w, r, &in) {
		return
	}
	email, valid := registrationEmail(in.Email)
	in.Organization = strings.TrimSpace(in.Organization)
	if !valid || len(in.Password) < 12 || len(in.Password) > 72 || utf8.RuneCountInString(in.Organization) < 1 || utf8.RuneCountInString(in.Organization) > 64 || strings.IndexFunc(in.Organization, unicode.IsControl) >= 0 {
		failure(w, 422, "invalid_account", "请输入有效邮箱、12–72 字节密码和 1–64 字符组织名称")
		return
	}
	select {
	case a.hashSlots <- struct{}{}:
		defer func() { <-a.hashSlots }()
	default:
		failure(w, 429, "rate_limited", "请稍后再试")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		storeError(w, err)
		return
	}
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		failure(w, 503, "unavailable", "服务暂时不可用")
		return
	}
	token := hex.EncodeToString(random)
	number, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		failure(w, 503, "unavailable", "服务暂时不可用")
		return
	}
	code := fmt.Sprintf("%08d", number.Int64())
	pending := storage.PendingRegistration{TokenHash: tokenHash(token), Email: email, PasswordHash: string(hash), Organization: in.Organization, CodeHash: tokenHash("email-code:" + token + ":" + code)}
	if err = a.store.BeginEmailRegistration(r.Context(), pending); err != nil {
		if errors.Is(err, storage.ErrRegistrationLimited) {
			failure(w, 429, "rate_limited", "请至少等待 60 秒后重试，每个邮箱每天最多请求 5 次")
		} else {
			storeError(w, err)
		}
		return
	}
	if err = a.emailSender.Send(r.Context(), email, code, pending.TokenHash); err != nil {
		failure(w, 503, "email_unavailable", "验证邮件暂时无法发送，请稍后重试")
		return
	}
	if err = a.store.DeliverEmailRegistration(r.Context(), pending.TokenHash); err != nil {
		failure(w, 503, "email_unavailable", "验证邮件暂时无法使用，请稍后重试")
		return
	}
	reply(w, 202, map[string]any{"data": map[string]any{"registration_token": token, "expires_in": 600}})
}
func (a *api) verifyRegistration(w http.ResponseWriter, r *http.Request) {
	if !a.registration {
		failure(w, 403, "registration_disabled", "注册未开放")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !a.attempts.allowLimit("verify-ip:"+ip, 30) {
		failure(w, 429, "rate_limited", "验证过于频繁，请稍后重试")
		return
	}
	var in struct {
		Token string `json:"registration_token"`
		Code  string `json:"code"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(in.Token) || !regexp.MustCompile(`^[0-9]{8}$`).MatchString(in.Code) {
		failure(w, 422, "invalid_verification", "验证码无效或已过期，请重新申请")
		return
	}
	if _, err := a.store.VerifyEmailRegistration(r.Context(), tokenHash(in.Token), tokenHash("email-code:"+in.Token+":"+in.Code)); err != nil {
		if errors.Is(err, storage.ErrVerification) || errors.Is(err, storage.ErrConflict) {
			failure(w, 422, "invalid_verification", "验证码无效或已过期，请重新申请")
		} else {
			storeError(w, err)
		}
		return
	}
	reply(w, 201, map[string]any{"data": map[string]bool{"registered": true}})
}
