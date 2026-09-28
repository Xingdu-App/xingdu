// Package vault encrypts machine credentials with deployment-owned key material.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
)

type Vault struct{ aead cipher.AEAD }

func New(key string) (*Vault, error) {
	b, err := hex.DecodeString(key)
	if err != nil || len(b) != 32 {
		return nil, errors.New("credential key must be 64 hex characters")
	}
	defer clear(b)
	block, err := aes.NewCipher(b)
	if err != nil {
		return nil, err
	}
	a, err := cipher.NewGCM(block)
	return &Vault{a}, err
}
func (v *Vault) Seal(plain []byte, aad string) []byte {
	nonce := make([]byte, v.aead.NonceSize())
	rand.Read(nonce)
	return v.aead.Seal(nonce, nonce, plain, []byte(aad))
}
func (v *Vault) Open(data []byte, aad string) ([]byte, error) {
	n := v.aead.NonceSize()
	if len(data) < n {
		return nil, errors.New("invalid encrypted credential")
	}
	out, err := v.aead.Open(nil, data[:n], data[n:], []byte(aad))
	if err != nil {
		return nil, errors.New("credential authentication failed")
	}
	return out, nil
}
