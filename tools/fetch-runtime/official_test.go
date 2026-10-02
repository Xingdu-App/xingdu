package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestOfficialArchiveRejectsTamperingAndMissingLicense(t *testing.T) {
	hash := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	for _, license := range []bool{true, false} {
		var buffer bytes.Buffer
		z := zip.NewWriter(&buffer)
		f, _ := z.Create("xray")
		f.Write([]byte("official binary fixture"))
		if license {
			f, _ = z.Create("LICENSE")
			f.Write([]byte("license"))
		}
		z.Close()
		data := buffer.Bytes()
		binary, _, err := officialXray(data, hash(data), hash([]byte("official binary fixture")))
		if license && (err != nil || string(binary) != "official binary fixture") {
			t.Fatal(err)
		}
		if !license && err == nil {
			t.Fatal("missing license accepted")
		}
		if _, _, err = officialXray(data, hash([]byte("tampered")), hash(binary)); err == nil {
			t.Fatal("archive tampering accepted")
		}
		if _, _, err = officialXray(data, hash(data), hash([]byte("wrong executable"))); err == nil {
			t.Fatal("executable tampering accepted")
		}
	}
}
