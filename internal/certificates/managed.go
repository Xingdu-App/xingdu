package certificates

import (
	"context"
	"errors"
	"golang.org/x/crypto/acme"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

type ManagedConfig struct {
	ValidationDomain, PlatformDomain, StateDir, Email, Token, Zone, Directory string
	AcceptTerms                                                               bool
}

func ManagedEnvironment() ManagedConfig {
	directory := Staging
	if os.Getenv("XINGDU_CERTIFICATE_PRODUCTION") == "true" {
		directory = Production
	}
	return ManagedConfig{PlatformDomain: os.Getenv("XINGDU_CERTIFICATE_PLATFORM_DOMAIN"), ValidationDomain: os.Getenv("XINGDU_CERTIFICATE_VALIDATION_DOMAIN"), StateDir: os.Getenv("XINGDU_CERTIFICATE_STATE_DIR"), Email: os.Getenv("XINGDU_CERTIFICATE_EMAIL"), Token: os.Getenv("XINGDU_CERTIFICATE_CLOUDFLARE_TOKEN"), Zone: os.Getenv("XINGDU_CERTIFICATE_CLOUDFLARE_ZONE"), Directory: directory, AcceptTerms: os.Getenv("XINGDU_CERTIFICATE_ACCEPT_TOS") == "true"}
}
func ValidDomain(domain string) bool {
	if domain != strings.ToLower(domain) || !domainName.MatchString(domain) || len(domain) > 253 {
		return false
	}
	if _, err := netip.ParseAddr(domain); err == nil {
		return false
	}
	suffix := domain[strings.LastIndex(domain, ".")+1:]
	return len(suffix) >= 2 && strings.IndexFunc(suffix, func(r rune) bool { return r >= 'a' && r <= 'z' }) >= 0 && suffix != "localhost" && suffix != "local" && suffix != "invalid" && suffix != "test" && suffix != "example"
}
func (c ManagedConfig) Configured() bool {
	return ValidDomain(c.ValidationDomain) && len(c.ValidationDomain) <= 204 && c.StateDir != "" && c.Email != "" && c.Token != "" && zoneID.MatchString(c.Zone) && c.AcceptTerms && (c.Directory == Staging || c.Directory == Production)
}
func (c ManagedConfig) Target(id string) string {
	return "_acme-challenge." + strings.TrimPrefix(id, "cert_") + "." + c.ValidationDomain
}

// Verify exact delegated ownership even if the CA reuses a valid authorization.
type DelegatedDNS struct {
	DNS            DNS
	Domain, Target string
	LookupCNAME    func(context.Context, string) (string, error)
}

func (d DelegatedDNS) Verify(ctx context.Context) error {
	lookup := d.LookupCNAME
	if lookup == nil {
		lookup = net.DefaultResolver.LookupCNAME
	}
	cname, err := lookup(ctx, "_acme-challenge."+d.Domain)
	if err != nil || strings.TrimSuffix(strings.ToLower(cname), ".") != d.Target {
		return errors.New("dns_delegation_required")
	}
	return nil
}
func (d DelegatedDNS) Present(ctx context.Context, name, value string) (string, error) {
	if name != "_acme-challenge."+d.Domain {
		return "", errors.New("unexpected challenge name")
	}
	if err := d.Verify(ctx); err != nil {
		return "", err
	}
	return d.DNS.Present(ctx, d.Target, value)
}
func (d DelegatedDNS) Remove(ctx context.Context, id string) error { return d.DNS.Remove(ctx, id) }
func (c ManagedConfig) Issue(ctx context.Context, domain, target string) (Bundle, error) {
	if !c.Configured() || !ValidDomain(domain) || !strings.HasPrefix(target, "_acme-challenge.") || !strings.HasSuffix(target, "."+c.ValidationDomain) {
		return Bundle{}, errors.New("provider_unavailable")
	}
	dns := DelegatedDNS{DNS: Cloudflare{Token: c.Token, Zone: c.Zone}, Domain: domain, Target: target}
	if err := dns.Verify(ctx); err != nil {
		return Bundle{}, err
	}
	key, err := AccountKey(c.StateDir)
	if err != nil {
		return Bundle{}, errors.New("account_state_unavailable")
	}
	client := &acme.Client{Key: key, DirectoryURL: c.Directory, HTTPClient: &http.Client{Timeout: 30 * time.Second}}
	out, err := Issue(ctx, client, dns, domain, c.Email)
	out.Domain, out.Directory = domain, c.Directory
	return out, err
}

func (c ManagedConfig) PlatformConfigured() bool {
	return c.Configured() && ValidDomain(c.PlatformDomain) && len(c.PlatformDomain) <= 220
}
func PublicAddress(address string) bool {
	ip, err := netip.ParseAddr(address)
	if err != nil || ip.Is4In6() || ip == netip.MustParseAddr("168.63.129.16") {
		return false
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.Zone() != "" {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}

// Platform hostnames are opaque allocations. Existing records are never retargeted.
func (c Cloudflare) EnsureAddress(ctx context.Context, domain, address string) error {
	if !ValidDomain(domain) || !PublicAddress(address) {
		return errors.New("invalid_platform_address")
	}
	var zone struct {
		Name string `json:"name"`
	}
	if err := c.call(ctx, "GET", "", nil, &zone); err != nil {
		return err
	}
	if !strings.HasSuffix(domain, "."+zone.Name) {
		return errors.New("platform domain outside zone")
	}
	var records []struct {
		Type, Content string
		Proxied       bool
	}
	if err := c.call(ctx, "GET", "/dns_records?name="+url.QueryEscape(domain), nil, &records); err != nil {
		return err
	}
	kind := "A"
	ip, _ := netip.ParseAddr(address)
	if ip.Is6() && !ip.Is4In6() {
		kind = "AAAA"
	}
	if len(records) > 0 {
		if len(records) == 1 && records[0].Type == kind && records[0].Content == address && !records[0].Proxied {
			return nil
		}
		return errors.New("platform_dns_conflict")
	}
	return c.call(ctx, "POST", "/dns_records", map[string]any{"type": kind, "name": domain, "content": address, "ttl": 60, "proxied": false}, nil)
}
func (c ManagedConfig) IssuePlatform(ctx context.Context, domain, address string) (Bundle, error) {
	if !c.PlatformConfigured() || !strings.HasSuffix(domain, "."+c.PlatformDomain) {
		return Bundle{}, errors.New("provider_unavailable")
	}
	dns := Cloudflare{Token: c.Token, Zone: c.Zone}
	if err := dns.EnsureAddress(ctx, domain, address); err != nil {
		return Bundle{}, err
	}
	key, err := AccountKey(c.StateDir)
	if err != nil {
		return Bundle{}, errors.New("account_state_unavailable")
	}
	out, err := Issue(ctx, &acme.Client{Key: key, DirectoryURL: c.Directory, HTTPClient: &http.Client{Timeout: 30 * time.Second}}, dns, domain, c.Email)
	out.Domain, out.Directory = domain, c.Directory
	return out, err
}
