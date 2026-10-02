package protocol

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

func (in Input) RealityEnabled() bool { return in.V2Ray != nil && in.V2Ray.Reality }

// Feature gates supplement the protocol-level minimum without changing old nodes.
func (in Input) RequiredAgentVersion() string {
	if in.RealityEnabled() || in.Protocol == "trojan" && in.V2Ray != nil {
		return "0.17.0-dev"
	}
	return MinimumAgentVersion(in.Protocol)
}

func newRealityKeys(s *Spec) error {
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	s.RealityPrivateKey = base64.RawURLEncoding.EncodeToString(k.Bytes())
	s.RealityPublicKey = base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes())
	b := make([]byte, 8)
	if _, err = rand.Read(b); err != nil {
		return err
	}
	s.RealityShortID = hex.EncodeToString(b)
	return nil
}

func validateRealityKeys(s Spec) error {
	if !s.RealityEnabled() {
		if s.RealityPrivateKey != "" || s.RealityPublicKey != "" || s.RealityShortID != "" {
			return errors.New("unexpected REALITY keys")
		}
		return nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s.RealityPrivateKey)
	if err != nil {
		return errors.New("invalid REALITY private key")
	}
	k, err := ecdh.X25519().NewPrivateKey(b)
	if err != nil || base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()) != s.RealityPublicKey {
		return errors.New("invalid REALITY key pair")
	}
	id, err := hex.DecodeString(s.RealityShortID)
	if err != nil || len(id) != 8 || hex.EncodeToString(id) != s.RealityShortID {
		return errors.New("invalid REALITY short ID")
	}
	return nil
}

// ReconcileFeatureKeys supports edits and recovery without rotating unrelated credentials.
func ReconcileFeatureKeys(s *Spec, rotate bool) error {
	if !s.RealityEnabled() {
		s.RealityPrivateKey, s.RealityPublicKey, s.RealityShortID = "", "", ""
		return nil
	}
	if rotate || s.RealityPrivateKey == "" {
		return newRealityKeys(s)
	}
	return validateRealityKeys(*s)
}
