package geoip

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func archive(entries ...string) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, name := range entries {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: 4})
		_, _ = tw.Write([]byte("data"))
	}
	_ = tw.Close()
	_ = gz.Close()
	return b.Bytes()
}
func TestCountryArchive(t *testing.T) {
	for _, tc := range []struct {
		name  string
		data  []byte
		valid bool
	}{
		{"country", archive("release/GeoLite2-Country.mmdb"), true},
		{"missing", archive("release/GeoLite2-ASN.mmdb"), false},
		{"duplicate", archive("a/GeoLite2-Country.mmdb", "b/GeoLite2-Country.mmdb"), false},
		{"corrupt", []byte("invalid"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, e := extractCountry(tc.data)
			if (e == nil) != tc.valid {
				t.Fatal("unexpected archive acceptance")
			}
			if tc.valid && string(b) != "data" {
				t.Fatal("wrong entry extracted")
			}
		})
	}
	broken := archive("release/GeoLite2-Country.mmdb")
	broken[len(broken)-1] ^= 1
	if _, err := extractCountry(broken); err == nil {
		t.Fatal("accepted invalid gzip checksum")
	}
}
func TestDownloadRedirectProtectsCredentials(t *testing.T) {
	for _, target := range []string{"https://" + storageHost + "/signed?token=redacted", "https://evil.invalid/data", "http://" + storageHost + "/data", "https://" + storageHost + ":8443/data"} {
		u, _ := url.Parse(target)
		req := &http.Request{URL: u, Header: make(http.Header)}
		req.SetBasicAuth("fixture-account", "fixture-secret")
		err := checkRedirect(req, nil)
		if req.Header.Get("Authorization") != "" {
			t.Fatal("credentials followed redirect")
		}
		if (err == nil) != (u.Host == storageHost && u.Scheme == "https") {
			t.Fatal("unexpected redirect acceptance")
		}
	}
}
func TestDownloadAuthenticationAndSanitizedErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, k, ok := r.BasicAuth()
		if !ok || a != "account-fixture" || k != "secret-fixture" {
			t.Error("authentication missing")
		}
		if r.URL.RawQuery != "" {
			t.Error("credentials entered query string")
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("secret-fixture"))
	}))
	defer server.Close()
	_, err := download(context.Background(), server.Client(), server.URL, credentials{"account-fixture", "secret-fixture"})
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "secret-fixture") {
		t.Fatal("unsafe download error")
	}
}
func TestAtomicDatabaseReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "country.mmdb")
	if err := persist(path, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := persist(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "new" {
		t.Fatal("database not replaced")
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm()&0077 != 0 {
		t.Fatal("database is not private")
	}
	files, _ := os.ReadDir(filepath.Dir(path))
	if len(files) != 1 {
		t.Fatal("temporary files left behind")
	}
}
