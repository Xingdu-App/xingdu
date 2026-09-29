// Package geoip performs offline country lookups against a bundled DB-IP snapshot.
package geoip

import (
	_ "embed"
	"net/netip"
	"sync"

	"github.com/oschwald/maxminddb-golang/v2"
)

//go:embed country.mmdb
var database []byte

var reader = sync.OnceValue(func() *maxminddb.Reader {
	db, err := maxminddb.OpenBytes(database)
	if err != nil {
		return nil
	}
	return db
})

// Country returns an ISO 3166-1 alpha-2 code, or empty for unknown addresses.
// It never resolves hostnames or sends addresses to an external service.
func Country(address string) string {
	ip, err := netip.ParseAddr(address)
	if err != nil || ip.Zone() != "" {
		return ""
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || reserved(ip) {
		return ""
	}
	db := reader()
	if db == nil {
		return ""
	}
	var record struct {
		Country struct {
			Code string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	if err := db.Lookup(ip).Decode(&record); err != nil {
		return ""
	}
	code := record.Country.Code
	if len(code) != 2 || code == "ZZ" || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return ""
	}
	return code
}

var special = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func reserved(ip netip.Addr) bool {
	for _, prefix := range special {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}
