package emailverification

import (
	"strings"
	"testing"
)

func TestTransactionalTemplates(t *testing.T) {
	for _, kind := range []string{"registration", "password_reset", "welcome", "password_changed", "invitation", "billing_active", "billing_attention", "billing_canceling", "billing_ended", "billing_changed"} {
		t.Run(kind, func(t *testing.T) {
			c, err := Render(Message{Kind: kind, Code: "12345678", URL: "https://xingdu.example/app#invite=fixture", Organization: `<script>alert("x")</script>`, Role: "member", Plan: "start", Status: "active"})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(c.HTML, "<script>") || !strings.Contains(c.HTML, "<html") || !strings.Contains(c.Text, "info@xingdu.app") || c.Subject == "" {
				t.Fatal("unsafe or incomplete template")
			}
		})
	}
	for _, url := range []string{"javascript:alert(1)", "https://user:secret@example.com", "http://public.example/app"} {
		if _, err := Render(Message{Kind: "invitation", URL: url}); err == nil {
			t.Fatal("unsafe URL accepted")
		}
	}
	if _, err := Render(Message{Kind: "registration", Code: "<script>"}); err == nil {
		t.Fatal("invalid code accepted")
	}
	if _, err := Render(Message{Kind: "unknown"}); err == nil {
		t.Fatal("unknown template accepted")
	}
}
