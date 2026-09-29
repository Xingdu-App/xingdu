package geoip

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestBundledDatabase(t *testing.T) {
	if got := fmt.Sprintf("%x", sha256.Sum256(database)); got != "d284ae2e7427fe33d83465e1506b2b21aae47eb8a9b099f8f4dac6a98c99f041" {
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
