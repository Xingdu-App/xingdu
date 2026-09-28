// Package backup implements versioned, authenticated streaming backup archives.
// Every record, including the mandatory end marker, is authenticated. Callers
// must not consume decrypted data until Decrypt succeeds for the whole archive.
package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

const chunkSize = 1024 * 1024

var magic = [8]byte{'X', 'I', 'N', 'G', 'D', 'U', 1, 0}
var ErrArchive = errors.New("backup archive authentication failed or archive is incomplete")

func aeadFor(key, header []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("backup key must contain exactly 32 bytes")
	}
	derived, err := hkdf.Key(sha256.New, key, header[8:], "xingdu-backup-v1", 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func recordMetadata(header []byte, sequence uint64, length uint32) ([]byte, []byte) {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce[4:], sequence)
	aad := make([]byte, len(header)+12)
	copy(aad, header)
	binary.BigEndian.PutUint64(aad[len(header):], sequence)
	binary.BigEndian.PutUint32(aad[len(header)+8:], length)
	return nonce, aad
}
func writeAll(w io.Writer, b []byte) error {
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}

func Encrypt(w io.Writer, r io.Reader, key []byte) error {
	header := make([]byte, 40)
	copy(header, magic[:])
	if _, err := rand.Read(header[8:]); err != nil {
		return err
	}
	aead, err := aeadFor(key, header)
	if err != nil {
		return err
	}
	if err = writeAll(w, header); err != nil {
		return err
	}
	buf := make([]byte, chunkSize)
	for sequence := uint64(0); ; sequence++ {
		n, readErr := io.ReadFull(r, buf)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return readErr
		}
		size := uint32(n + aead.Overhead())
		nonce, aad := recordMetadata(header, sequence, size)
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], size)
		if err = writeAll(w, length[:]); err != nil {
			return err
		}
		if err = writeAll(w, aead.Seal(nil, nonce, buf[:n], aad)); err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
		if sequence == ^uint64(0) {
			return errors.New("backup exceeds record limit")
		}
	}
}

func Decrypt(w io.Writer, r io.Reader, key []byte) error {
	header := make([]byte, 40)
	if _, err := io.ReadFull(r, header); err != nil {
		return ErrArchive
	}
	if string(header[:8]) != string(magic[:]) {
		return ErrArchive
	}
	aead, err := aeadFor(key, header)
	if err != nil {
		return err
	}
	for sequence := uint64(0); ; sequence++ {
		var length [4]byte
		if _, err = io.ReadFull(r, length[:]); err != nil {
			return ErrArchive
		}
		size := binary.BigEndian.Uint32(length[:])
		if size < uint32(aead.Overhead()) || size > chunkSize+uint32(aead.Overhead()) {
			return ErrArchive
		}
		record := make([]byte, size)
		if _, err = io.ReadFull(r, record); err != nil {
			return ErrArchive
		}
		nonce, aad := recordMetadata(header, sequence, size)
		plain, err := aead.Open(nil, nonce, record, aad)
		if err != nil {
			return ErrArchive
		}
		if len(plain) == 0 {
			var trailer [1]byte
			n, e := io.ReadFull(r, trailer[:])
			if n != 0 || e != io.EOF {
				return ErrArchive
			}
			return nil
		}
		if err = writeAll(w, plain); err != nil {
			return err
		}
		if sequence == ^uint64(0) {
			return ErrArchive
		}
	}
}
