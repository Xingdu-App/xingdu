// Package emailverification sends registration codes without logging message
// bodies, recipient addresses, provider responses or credentials.
package emailverification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("email delivery unavailable")

type Sender interface {
	Configured() bool
	Send(context.Context, string, string, string) error
}
type Resend struct {
	key, from string
	client    *http.Client
}

func NewResend(key, from string) *Resend {
	return &Resend{key: strings.TrimSpace(key), from: strings.TrimSpace(from), client: &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (s *Resend) Configured() bool {
	if s == nil || !strings.HasPrefix(s.key, "re_") || len(s.key) < 16 {
		return false
	}
	lower := strings.ToLower(s.key)
	for _, word := range []string{"placeholder", "replace", "example", "xxxx"} {
		if strings.Contains(lower, word) {
			return false
		}
	}
	a, e := mail.ParseAddress(s.from)
	return e == nil && strings.Contains(a.Address, "@") && !strings.ContainsAny(s.from, "\r\n")
}

var codePattern = regexp.MustCompile(`^[0-9]{8}$`)

func (s *Resend) Send(ctx context.Context, to, code, idempotency string) error {
	if !s.Configured() || !codePattern.MatchString(code) {
		return ErrUnavailable
	}
	message := fmt.Sprintf("星渡 Xingdu\n\n你的邮箱验证码 / Your email verification code: %s\n\n验证码有效期为 10 分钟，请勿分享。This code expires in 10 minutes. Do not share it.\n如果你没有请求注册，请忽略此邮件。已有账户不会因本次请求而改变。\nIf you did not request registration, ignore this message. Existing accounts are not changed by this request.\n", code)
	body, _ := json.Marshal(map[string]any{"from": s.from, "to": []string{to}, "subject": "星渡 Xingdu · 邮箱验证 / Verify your email", "text": message})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "xingdu-registration/"+idempotency)
	response, err := s.client.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ErrUnavailable
	}
	var result struct {
		ID string `json:"id"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result) != nil || result.ID == "" {
		return ErrUnavailable
	}
	return nil
}
