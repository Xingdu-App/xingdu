package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/protocol"
)

func TestRejectsCorruptCachedArchiveBeforeWritingExecutable(t *testing.T) {
	cache, output := t.TempDir(), t.TempDir()
	path := filepath.Join(cache, "sing-box-"+protocol.RuntimeVersion+"-linux-amd64.tar.gz")
	if err := os.WriteFile(path, []byte("untrusted archive"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(output, cache); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unexpected result: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "sing-box-linux-amd64")); !os.IsNotExist(err) {
		t.Fatal("unverified executable published")
	}
}
