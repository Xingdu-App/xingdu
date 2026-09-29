package main

import (
	"os"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/certificates"
)

func TestCachedCertificateCannotCrossDomainOrCA(t *testing.T) {
	for _, tc := range []struct{ name, domain, directory string }{
		{"another domain", "other.example.com", certificates.Staging},
		{"another CA", "node.example.com", certificates.Production},
		{"legacy unbound cache", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if _, err := certificates.AccountKey(dir); err != nil {
				t.Fatal(err)
			}
			if err := certificates.Save(dir, certificates.Bundle{Certificate: "cached certificate", Domain: tc.domain, Directory: tc.directory}); err != nil {
				t.Fatal(err)
			}
			t.Setenv("XINGDU_ACME_STATE", dir)
			t.Setenv("XINGDU_ACME_DOMAIN", "node.example.com")
			t.Setenv("XINGDU_ACME_EMAIL", "operator@example.com")
			t.Setenv("XINGDU_ACME_ACCEPT_TOS", "true")
			t.Setenv("XINGDU_ACME_PRODUCTION", "false")
			if err := run(); err == nil || !strings.Contains(err.Error(), "another domain or CA") {
				t.Fatalf("cache scope not rejected: %v", err)
			}
		})
	}
}
