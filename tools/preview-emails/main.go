// Preview transactional templates without contacting an email provider.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"xingdu.app/xingdu/internal/emailverification"
)

func main() {
	dir := ".local/email-previews"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(err)
	}
	var index strings.Builder
	index.WriteString(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>星渡邮件预览</title><body style="font-family:sans-serif;padding:32px"><h1>星渡邮件预览 / Email previews</h1><p>仅示例内容，不发送邮件。Sample content only; no email is sent.</p><ul>`)
	for _, kind := range []string{"registration", "password_reset", "welcome", "password_changed", "invitation", "billing_active", "billing_attention", "billing_canceling", "billing_ended", "billing_changed"} {
		m := emailverification.Message{Kind: kind, Code: "12345678", Organization: "Example workspace", Role: "member", Plan: "start", Status: "active", PeriodEnd: "2030-01-01 00:00 UTC", URL: "https://xingdu.example/app"}
		if kind == "registration" || kind == "password_reset" {
			m.URL = ""
		}
		if kind == "billing_attention" {
			m.Status = "past_due"
		}
		if kind == "billing_ended" {
			m.Status = "canceled"
		}
		c, err := emailverification.Render(m)
		if err != nil {
			panic(err)
		}
		if err = os.WriteFile(filepath.Join(dir, kind+".html"), []byte(c.HTML), 0600); err != nil {
			panic(err)
		}
		if err = os.WriteFile(filepath.Join(dir, kind+".txt"), []byte(c.Text), 0600); err != nil {
			panic(err)
		}
		fmt.Fprintf(&index, `<li><a href="%s.html">%s</a></li>`, kind, kind)
	}
	index.WriteString("</ul></body></html>")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(index.String()), 0600); err != nil {
		panic(err)
	}
	fmt.Println("Email previews generated; no messages sent.")
}
