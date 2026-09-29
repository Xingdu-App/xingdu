// Package emailverification sends registration codes without logging message
// bodies, recipient addresses, provider responses or credentials.
package emailverification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	if strings.TrimSpace(from) == "" {
		from = "Xingdu <noreply@xingdu.app>"
	}
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
	return s.SendMessage(ctx, to, Message{Kind: "registration", Code: code}, "xingdu-registration/"+idempotency)
}

type MessageSender interface {
	Configured() bool
	SendMessage(context.Context, string, Message, string) error
}

func (s *Resend) SendMessage(ctx context.Context, to string, m Message, idempotency string) error {
	if !s.Configured() {
		return ErrUnavailable
	}
	address, err := mail.ParseAddress(to)
	if err != nil || address.Address != to || strings.ContainsAny(to, "\r\n") || idempotency == "" || len(idempotency) > 256 || strings.ContainsAny(idempotency, "\r\n") {
		return ErrUnavailable
	}
	content, err := Render(m)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"from": s.from, "to": []string{to}, "subject": content.Subject, "html": content.HTML, "text": content.Text, "reply_to": "info@xingdu.app"})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotency)
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
