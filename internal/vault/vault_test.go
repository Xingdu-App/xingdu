package vault

import (
	"bytes"
	"strings"
	"testing"
)

func TestCredentialEnvelope(t *testing.T) {
	v, e := New(strings.Repeat("ab", 32))
	if e != nil {
		t.Fatal(e)
	}
	plain := []byte("private credential")
	a := v.Seal(plain, "org:host:job")
	b := v.Seal(plain, "org:host:job")
	if bytes.Equal(a, b) || bytes.Contains(a, plain) {
		t.Fatal("nonce reused or plaintext exposed")
	}
	if out, e := v.Open(a, "org:host:job"); e != nil || !bytes.Equal(out, plain) {
		t.Fatal(e)
	}
	for _, aad := range []string{"other:host:job", "org:other:job", "org:host:other"} {
		if _, e = v.Open(a, aad); e == nil {
			t.Fatal("AAD isolation missing")
		}
	}
	a[len(a)-1] ^= 1
	if _, e = v.Open(a, "org:host:job"); e == nil {
		t.Fatal("tampering accepted")
	}
	if _, e = New("short"); e == nil {
		t.Fatal("weak key accepted")
	}
}
