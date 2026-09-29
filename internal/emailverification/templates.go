package emailverification

import (
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

// Message contains only transactional fields; never include passwords or API keys.
type Message struct{ Kind, Code, URL, Organization, Role, Status, Plan, PeriodEnd string }
type Content struct{ Subject, HTML, Text string }

type templateData struct{ Title, Intro, Detail, Code, URL, Action string }

var layout = template.Must(template.New("email").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body style="margin:0;background:#f3f5f4;color:#253c35;font-family:Arial,'PingFang SC',sans-serif"><table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td align="center" style="padding:32px 16px"><table role="presentation" width="560" cellspacing="0" cellpadding="0" style="width:100%;max-width:560px;background:#fff;border-radius:16px"><tr><td style="padding:32px"><p style="font-size:14px;letter-spacing:3px;color:#477b71">XINGDU · 星渡</p><h1 style="font-size:24px;line-height:1.4">{{.Title}}</h1><p style="line-height:1.8">{{.Intro}}</p>{{if .Code}}<p style="font-size:32px;letter-spacing:6px;background:#edf4f0;padding:20px;text-align:center;border-radius:10px"><strong>{{.Code}}</strong></p>{{end}}<p style="line-height:1.8;color:#536960">{{.Detail}}</p>{{if .URL}}<p style="margin:28px 0"><a href="{{.URL}}" style="display:inline-block;padding:14px 22px;background:#315c4e;color:#fff;text-decoration:none;border-radius:8px">{{.Action}}</a></p><p style="font-size:12px;line-height:1.8;overflow-wrap:anywhere;word-break:break-all">无法点击按钮？复制链接 / Copy this link:<br><a href="{{.URL}}">{{.URL}}</a></p>{{end}}<hr style="border:0;border-top:1px solid #e5ebe7;margin:28px 0"><p style="font-size:12px;line-height:1.8;color:#65756d">星渡账号与服务通知 · Xingdu account &amp; service notification<br>需要帮助？Help: <a href="mailto:info@xingdu.app">info@xingdu.app</a><br>请勿转发验证码或邀请链接。Do not forward codes or invitation links.</p></td></tr></table></td></tr></table></body></html>`))

func Render(m Message) (Content, error) {
	d := templateData{URL: m.URL, Action: "打开控制台 / Open console"}
	if m.URL != "" {
		u, e := url.Parse(m.URL)
		if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost"))) {
			return Content{}, ErrUnavailable
		}
	}
	switch m.Kind {
	case "registration":
		if !codePattern.MatchString(m.Code) {
			return Content{}, ErrUnavailable
		}
		d.Code = m.Code
		d.Title = "验证你的邮箱 / Verify your email"
		d.Intro = "使用以下验证码完成星渡注册。Use this code to complete your Xingdu registration."
		d.Detail = "验证码 10 分钟内有效。若非本人操作，请忽略；已有账号不会因此改变。Expires in 10 minutes. Ignore this message if you did not request registration; existing accounts remain unchanged."
	case "password_reset":
		if !codePattern.MatchString(m.Code) {
			return Content{}, ErrUnavailable
		}
		d.Code = m.Code
		d.Title = "重置密码 / Reset your password"
		d.Intro = "请在刚才的找回密码页面输入此验证码。Enter this code on the password recovery page you just opened."
		d.Detail = "验证码 10 分钟内有效，仅适用于已验证邮箱且启用密码登录的账号。若使用第三方登录，请继续使用原登录方式。非本人操作请忽略。Expires in 10 minutes. Only verified accounts with password login can reset a password. Social-only accounts should use their existing sign-in provider. Ignore unsolicited requests."
	case "welcome":
		d.Title = "欢迎使用星渡 / Welcome to Xingdu"
		d.Intro = "账号已准备好，现在可以接入你的第一台服务器。Your account is ready. Connect your first server whenever you are ready."
		d.Detail = "在控制台管理服务器、部署节点并生成客户端订阅。Manage servers, deploy nodes and create client subscriptions from your console."
	case "password_changed":
		d.Title = "密码已变更 / Password changed"
		d.Intro = "你的星渡登录密码已更新，所有原有登录会话已失效。Your Xingdu password was updated and all previous login sessions were revoked."
		d.Detail = "若非本人操作，请立即通过登录页找回密码并联系支持。If this was not you, recover your password from the sign-in page and contact support."
	case "invitation":
		if m.URL == "" {
			return Content{}, ErrUnavailable
		}
		d.Title = "组织邀请 / Organization invitation"
		d.Intro = fmt.Sprintf("你受邀加入 %s，角色为 %s。You are invited to join %s as %s.", m.Organization, m.Role, m.Organization, m.Role)
		d.Detail = "邀请 7 天内有效，可使用一次。登录后确认加入。此链接不绑定收件邮箱，请勿转发。Valid for 7 days and one use. Sign in to accept. This bearer link is not bound to the recipient email; do not forward it."
		d.Action = "查看邀请 / View invitation"
	case "billing_active", "billing_attention", "billing_canceling", "billing_ended", "billing_changed":
		titles := map[string]string{"billing_active": "套餐已生效 / Subscription active", "billing_attention": "套餐付款需要处理 / Billing needs attention", "billing_canceling": "套餐将在周期结束后取消 / Cancellation scheduled", "billing_ended": "付费套餐已结束 / Subscription ended", "billing_changed": "套餐状态已更新 / Subscription updated"}
		d.Title = titles[m.Kind]
		d.Intro = fmt.Sprintf("组织 / Organization: %s · 套餐 / Plan: %s · 状态 / Status: %s", m.Organization, m.Plan, m.Status)
		d.Detail = "请在控制台查看当前权益、账单与付款方式。本邮件是状态通知，不是付款收据。Review your current access, billing and payment method in the console. This is a status notification, not a payment receipt."
		if m.PeriodEnd != "" {
			d.Detail += " 当前周期结束 / Current period ends: " + m.PeriodEnd
		}
		d.Action = "查看套餐与账单 / View billing"
	default:
		return Content{}, ErrUnavailable
	}
	var out bytes.Buffer
	if err := layout.Execute(&out, d); err != nil {
		return Content{}, ErrUnavailable
	}
	text := strings.Join([]string{"星渡 Xingdu", d.Title, d.Intro, d.Code, d.Detail, d.URL, "帮助 / Help: info@xingdu.app"}, "\n\n")
	return Content{Subject: "星渡 Xingdu · " + d.Title, HTML: out.String(), Text: text}, nil
}
