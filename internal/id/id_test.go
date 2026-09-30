package id

import "testing"

func TestTypedRandomIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, prefix := range []string{"usr", "org", "srv", "node", "op", "lease", "ses", "inv", "job", "sub", "bat", "obj", "cert"} {
		for range 1000 {
			v := New(prefix)
			if !Valid(prefix, v) || seen[v] {
				t.Fatal("invalid or duplicate ID")
			}
			seen[v] = true
		}
	}
	for _, v := range []string{"00000000-0000-4000-8000-000000000001", "org_short", "srv_" + "00000000000000000000000000000000/", "org_0000000000000000000000000000000A"} {
		if ValidID(v) {
			t.Fatal("accepted malformed ID")
		}
	}
	if Valid("org", New("srv")) {
		t.Fatal("cross-type accepted")
	}
}
func TestLegacyMigration(t *testing.T) {
	const old = "00000000-0000-4000-8000-000000000001"
	if FromUUID("org", old) != "org_00000000000040008000000000000001" {
		t.Fatal("mapping changed")
	}
	if FromUUID("org", "bad") != "" || FromUUID("org", New("srv")) != "" {
		t.Fatal("invalid migration")
	}
}
