package geoip

import (
	"crypto/sha256"
	"fmt"
	"github.com/oschwald/maxminddb-golang/v2"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBundledDatabase(t *testing.T) {
	if got := fmt.Sprintf("%x", sha256.Sum256(database)); got != "dbd70ccfa2a13627eaf4913d19920a1a229f1b3c5439ac01c509995b6364202a" {
		t.Fatal("database changed; review provenance and refresh the pinned digest")
	}
	db := reader()
	if db == nil {
		t.Fatal("bundled database cannot be opened")
	}
	if err := db.Verify(); err != nil {
		t.Fatal(err)
	}
	// These are snapshot values for public anycast resolvers, not physical locations.
	for address, want := range map[string]string{"8.8.8.8": "US", "2001:4860:4860::8888": "CA", "::ffff:8.8.8.8": "US"} {
		if got := Country(address); got != want {
			t.Errorf("%s: got %q", address, got)
		}
	}
}

func TestUnknownAndPrivateAddresses(t *testing.T) {
	for _, address := range []string{"", "invalid", "example.com", "8.8.8.8:22", "127.0.0.1", "10.0.0.1", "192.168.1.1", "100.64.0.1", "192.0.2.1", "198.51.100.1", "203.0.113.1", "::1", "fc00::1", "fe80::1%eth0", "::ffff:192.168.1.1", "2001:db8::1", "224.0.0.1"} {
		if got := Country(address); got != "" {
			t.Errorf("%s incorrectly located as %s", address, got)
		}
	}
}

func TestGeoLiteMetadata(t *testing.T) {
	now := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, kind string
		age        time.Duration
		valid      bool
	}{
		{"fresh", "GeoLite2-Country", 2 * 24 * time.Hour, true},
		{"expired", "GeoLite2-Country", 31 * 24 * time.Hour, false},
		{"future", "GeoLite2-Country", -48 * time.Hour, false},
		{"ASN is not location", "GeoLite2-ASN", time.Hour, false},
		{"bundled fallback is not an override", "DBIP-Country-Lite", time.Hour, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := maxminddb.Metadata{DatabaseType: tc.kind, BuildEpoch: uint(now.Add(-tc.age).Unix())}
			if (validateGeoLite(m, now) == nil) != tc.valid {
				t.Fatal("unexpected metadata acceptance")
			}
		})
	}
}

func TestConfigureRejectsInvalidFiles(t *testing.T) {
	old := geoLiteReader.Load()
	for _, tc := range []struct {
		name     string
		contents []byte
	}{
		{"corrupt", []byte("not an MMDB")}, {"wrong provider", database},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "country.mmdb")
			if err := os.WriteFile(path, tc.contents, 0600); err != nil {
				t.Fatal(err)
			}
			if Configure(path) == nil {
				t.Fatal("accepted invalid GeoLite file")
			}
			if geoLiteReader.Load() != old {
				t.Fatal("changed reader after rejected configuration")
			}
		})
	}
	if Configure(filepath.Join(t.TempDir(), "missing.mmdb")) == nil {
		t.Fatal("accepted missing file")
	}
}

// Operators can exercise the adapter against a licensed local download without
// redistributing a GeoLite snapshot as a permanent test fixture.
func TestLocalGeoLiteDatabase(t *testing.T) {
	path := os.Getenv("XINGDU_TEST_GEOLITE_DATABASE")
	if path == "" {
		t.Skip("no local GeoLite database configured")
	}
	old := geoLiteReader.Load()
	defer func() {
		db := geoLiteReader.Swap(old)
		if db != nil && db != old {
			db.Close()
		}
	}()
	if err := Configure(path); err != nil {
		t.Fatal(err)
	}
	if Source() != "maxmind" {
		t.Fatal("wrong source")
	}
	if Country("8.8.8.8") != "US" {
		t.Fatal("public resolver lookup failed")
	}
	TestUnknownAndPrivateAddresses(t)
}
