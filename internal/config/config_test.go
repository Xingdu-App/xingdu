package config

import "testing"

func TestPublicOriginCookiePolicy(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/test")
	for _, tc := range []struct {
		origin        string
		valid, secure bool
	}{
		{"http://127.0.0.1:15173", true, false},
		{"https://console.example.invalid", true, true},
		{"http://console.example.invalid", false, false},
		{"https://example.invalid/path", false, false},
		{"https://user:password@example.invalid", false, false},
		{"https://example.invalid?query=yes", false, false},
	} {
		t.Setenv("XINGDU_PUBLIC_ORIGIN", tc.origin)
		cfg, err := Load()
		if (err == nil) != tc.valid {
			t.Fatalf("unexpected origin validation: %s", tc.origin)
		}
		if err == nil && cfg.SecureCookies != tc.secure {
			t.Fatalf("unexpected cookie policy: %s", tc.origin)
		}
	}
}
