package bootstrap

import (
	"errors"
	"net/netip"
	"os"
	"strings"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/vault"
)

func Configuration(fallback string) (*Connector, *vault.Vault, string, error) {
	origin := os.Getenv("XINGDU_AGENT_ORIGIN")
	if origin == "" {
		origin = fallback
	}
	if machine.Origin(origin) != nil || strings.ContainsAny(origin, "'\"\\\n\r") {
		return nil, nil, "", errors.New("invalid agent origin")
	}
	c := &Connector{ArtifactDir: os.Getenv("XINGDU_AGENT_ARTIFACT_DIR")}
	if c.ArtifactDir == "" {
		c.ArtifactDir = "/opt/xingdu/agents"
	}
	for _, raw := range strings.Split(os.Getenv("XINGDU_SSH_ALLOWED_CIDRS"), ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		p, e := netip.ParsePrefix(strings.TrimSpace(raw))
		if e != nil {
			return nil, nil, "", errors.New("invalid SSH allowlist")
		}
		c.Allowed = append(c.Allowed, p)
	}
	var v *vault.Vault
	if key := os.Getenv("XINGDU_CREDENTIAL_KEY"); key != "" {
		var e error
		v, e = vault.New(key)
		if e != nil {
			return nil, nil, "", e
		}
	}
	return c, v, origin, nil
}
