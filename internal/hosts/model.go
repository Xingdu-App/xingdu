package hosts

import (
	"errors"
	"net/netip"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type Input struct {
	Name    string   `json:"name"`
	Address string   `json:"address"`
	SSHPort int      `json:"ssh_port"`
	SSHUser string   `json:"ssh_user"`
	Tags    []string `json:"tags"`
	Notes   string   `json:"notes"`
}
type Host struct {
	ID string `json:"id"`
	Input
	Status       string     `json:"status"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	AgentVersion *string    `json:"agent_version"`
}

var label = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
var user = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

func (in *Input) Validate() error {
	in.Name = strings.TrimSpace(in.Name)
	in.Address = strings.ToLower(strings.TrimSpace(in.Address))
	in.SSHUser = strings.TrimSpace(in.SSHUser)
	in.Notes = strings.TrimSpace(in.Notes)
	if utf8.RuneCountInString(in.Name) < 1 || utf8.RuneCountInString(in.Name) > 64 {
		return errors.New("名称需要 1–64 个字符")
	}
	if len(in.Address) == 0 || len(in.Address) > 253 {
		return errors.New("请输入有效的 IP 地址或主机名")
	}
	if ip, err := netip.ParseAddr(in.Address); err == nil {
		if ip.Zone() != "" {
			return errors.New("不支持带区域标识的 IP 地址")
		}
		in.Address = ip.String()
	} else {
		for _, part := range strings.Split(in.Address, ".") {
			if !label.MatchString(part) {
				return errors.New("请输入 IP 地址或主机名，不要包含协议、端口或路径")
			}
		}
	}
	if in.SSHPort < 1 || in.SSHPort > 65535 {
		return errors.New("SSH 端口应在 1–65535 之间")
	}
	if !user.MatchString(in.SSHUser) {
		return errors.New("SSH 用户名格式不正确")
	}
	if utf8.RuneCountInString(in.Notes) > 1000 {
		return errors.New("备注最多 1000 个字符")
	}
	if len(in.Tags) > 10 {
		return errors.New("最多设置 10 个标签")
	}
	tags := make([]string, 0, len(in.Tags))
	seen := map[string]bool{}
	for _, tag := range in.Tags {
		tag = strings.TrimSpace(tag)
		if utf8.RuneCountInString(tag) < 1 || utf8.RuneCountInString(tag) > 24 {
			return errors.New("每个标签需要 1–24 个字符")
		}
		if !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}
	in.Tags = tags
	return nil
}
