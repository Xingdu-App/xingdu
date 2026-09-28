// Package id defines opaque, typed resource identifiers. Protocol credentials
// (for example VLESS UUIDs) are deliberately outside this namespace.
package id

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

var Pattern = regexp.MustCompile(`^(usr|org|srv|node|op|lease|ses|inv|job|sub|bat|obj)_[0-9a-f]{32}$`)
var legacy = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ValidID(value string) bool       { return Pattern.MatchString(value) }
func Valid(prefix, value string) bool { return ValidID(value) && strings.HasPrefix(value, prefix+"_") }
func New(prefix string) string {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		panic(err)
	}
	value := prefix + "_" + hex.EncodeToString(random[:])
	if !ValidID(value) {
		panic("unknown resource ID prefix")
	}
	return value
}

// FromUUID is only for controlled migration of existing records, never input validation.
func FromUUID(prefix, value string) string {
	if Valid(prefix, value) {
		return value
	}
	if !legacy.MatchString(value) {
		return ""
	}
	out := prefix + "_" + strings.ReplaceAll(value, "-", "")
	if !Valid(prefix, out) {
		return ""
	}
	return out
}
