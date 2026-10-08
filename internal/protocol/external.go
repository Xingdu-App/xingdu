package protocol

import (
	"net/netip"
	"unicode"
	"unicode/utf8"
)

const ExternalProxyAgentVersion = "0.18.0-dev"

// External targets never inherit operator SSH private-network exceptions.
var externalBlocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("::/96"), netip.MustParsePrefix("::ffff:0:0/96"), netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"), netip.MustParsePrefix("100::/64"), netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("fc00::/7"), netip.MustParsePrefix("fe80::/10"), netip.MustParsePrefix("ff00::/8"),
}

func ValidExternalEndpoint(address string, port int) bool {
	ip, err := netip.ParseAddr(address)
	if err != nil || ip.Zone() != "" || ip.Is4In6() || !ip.IsGlobalUnicast() || port < 1 || port > 65535 {
		return false
	}
	if ip == netip.MustParseAddr("168.63.129.16") {
		return false
	}
	for _, prefix := range externalBlocked {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
func ValidExternalCredential(value string) bool {
	if len(value) < 1 || len(value) > 255 || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (s Spec) RequiredAgentVersion() string {
	if s.Relay != nil && s.Relay.ExternalID != "" {
		return ExternalProxyAgentVersion
	}
	return s.Input.RequiredAgentVersion()
}
