package backup

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestAuthenticatedStream(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	for _, size := range []int{0, 1, chunkSize - 1, chunkSize, chunkSize + 1, 3*chunkSize + 17} {
		plain := bytes.Repeat([]byte{0xAB}, size)
		var archive, restored bytes.Buffer
		if err := Encrypt(&archive, bytes.NewReader(plain), key); err != nil {
			t.Fatal(err)
		}
		if err := Decrypt(&restored, bytes.NewReader(archive.Bytes()), key); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(plain, restored.Bytes()) {
			t.Fatal("round trip mismatch", size)
		}
		var second bytes.Buffer
		if err := Encrypt(&second, bytes.NewReader(plain), key); err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(archive.Bytes(), second.Bytes()) {
			t.Fatal("per-archive salt reused")
		}
	}
}
func TestTamperTruncationWrongKeyAndReordering(t *testing.T) {
	key := bytes.Repeat([]byte{9}, 32)
	var archive bytes.Buffer
	if err := Encrypt(&archive, bytes.NewReader(bytes.Repeat([]byte{42}, chunkSize+19)), key); err != nil {
		t.Fatal(err)
	}
	original := archive.Bytes()
	for _, offset := range []int{0, 7, 8, 39, 40, 43, 44, 44 + chunkSize, len(original) - 1} {
		corrupted := bytes.Clone(original)
		corrupted[offset] ^= 1
		if err := Decrypt(io.Discard, bytes.NewReader(corrupted), key); !errors.Is(err, ErrArchive) {
			t.Fatalf("tamper accepted at %d: %v", offset, err)
		}
	}
	for _, length := range []int{0, 39, 40, 43, 44, 100, len(original) - 1, len(original) - 20} {
		if err := Decrypt(io.Discard, bytes.NewReader(original[:length]), key); !errors.Is(err, ErrArchive) {
			t.Fatal("truncated archive accepted", length, err)
		}
	}
	if err := Decrypt(io.Discard, bytes.NewReader(original), bytes.Repeat([]byte{1}, 32)); !errors.Is(err, ErrArchive) {
		t.Fatal("wrong key accepted", err)
	}
	if err := Decrypt(io.Discard, bytes.NewReader(append(bytes.Clone(original), 0)), key); !errors.Is(err, ErrArchive) {
		t.Fatal("trailing record accepted", err)
	}
	// Cut-and-paste records cannot change their authenticated sequence position.
	firstEnd := 40 + 4 + chunkSize + 16
	reordered := append(bytes.Clone(original[:40]), original[firstEnd:len(original)-20]...)
	reordered = append(reordered, original[40:firstEnd]...)
	reordered = append(reordered, original[len(original)-20:]...)
	if err := Decrypt(io.Discard, bytes.NewReader(reordered), key); !errors.Is(err, ErrArchive) {
		t.Fatal("reordered records accepted", err)
	}
}
func TestKeyAndIOFailures(t *testing.T) {
	if err := Encrypt(io.Discard, bytes.NewReader(nil), []byte("bad")); err == nil {
		t.Fatal("invalid key accepted")
	}
	if err := Encrypt(brokenWriter{}, bytes.NewReader(nil), make([]byte, 32)); err == nil {
		t.Fatal("write error ignored")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
