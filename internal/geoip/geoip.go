// Package geoip performs offline country lookups using GeoLite or bundled DB-IP.
package geoip

import (
	_ "embed"
	"errors"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

//go:embed country.mmdb
var database []byte

var bundledReader = sync.OnceValue(func() *maxminddb.Reader {
	db, err := maxminddb.OpenBytes(database)
	if err != nil {
		return nil
	}
	return db
})

var geoLiteReader atomic.Pointer[maxminddb.Reader]
var readerLock sync.RWMutex

// Configure selects an operator-managed GeoLite Country file at process startup.
// Empty paths retain DB-IP. It never downloads data or falls back on invalid files.
// Restart after external file replacement, or use Start for managed refreshes.
func Configure(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.New("cannot read configured GeoLite country database")
	}
	db, err := openGeoLite(data)
	if err != nil {
		return err
	}
	installReader(db)
	return nil
}

func installReader(db *maxminddb.Reader) {
	readerLock.Lock()
	defer readerLock.Unlock()
	if old := geoLiteReader.Swap(db); old != nil {
		old.Close()
	}
}

func openGeoLite(data []byte) (*maxminddb.Reader, error) {
	db, err := maxminddb.OpenBytes(data)
	if err != nil {
		return nil, errors.New("invalid GeoLite country database")
	}
	if err := validateGeoLite(db.Metadata, time.Now()); err != nil {
		db.Close()
		return nil, err
	}
	if err := db.Verify(); err != nil {
		db.Close()
		return nil, errors.New("GeoLite country database integrity check failed")
	}
	return db, nil
}

func validateGeoLite(metadata maxminddb.Metadata, now time.Time) error {
	if metadata.DatabaseType != "GeoLite2-Country" {
		return errors.New("configured database must be GeoLite2-Country")
	}
	built := metadata.BuildTime()
	if built.After(now.Add(24*time.Hour)) || now.Sub(built) > 30*24*time.Hour {
		return errors.New("GeoLite country database must be updated within 30 days")
	}
	return nil
}

func reader() *maxminddb.Reader {
	if db := geoLiteReader.Load(); db != nil {
		// Do not silently keep serving old GeoLite data or switch sources.
		if validateGeoLite(db.Metadata, time.Now()) != nil {
			return nil
		}
		return db
	}
	return bundledReader()
}

// Source identifies the configured source, including when its data has expired.
func Source() string {
	if geoLiteReader.Load() != nil {
		return "maxmind"
	}
	return "dbip"
}

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
	readerLock.RLock()
	defer readerLock.RUnlock()
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
