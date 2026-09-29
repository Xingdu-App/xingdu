package httpapi

import (
	"encoding/json"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

func TestSubscriptionLinkRecovery(t *testing.T) {
	v, _ := vault.New(strings.Repeat("a", 64))
	a := &api{vault: v}
	org, id := storage.NewID("org"), storage.NewID("sub")
	token := machine.Token()
	encrypted := v.Seal([]byte(token), subscriptionTokenAAD(org, id))
	for _, scenario := range []string{"valid", "wrong_org", "wrong_id", "wrong_hash", "no_key"} {
		t.Run(scenario, func(t *testing.T) {
			sub := storage.Subscription{ID: id, EncryptedToken: encrypted, TokenHash: machine.Hash(token)}
			tenant := org
			handler := a
			switch scenario {
			case "wrong_org":
				tenant = storage.NewID("org")
			case "wrong_id":
				sub.ID = storage.NewID("sub")
			case "wrong_hash":
				sub.TokenHash = machine.Hash(machine.Token())
			case "no_key":
				handler = &api{}
			}
			handler.revealSubscriptionLink(tenant, &sub)
			if scenario == "valid" {
				if sub.SubscriptionPath != subscriptionPath(id, token) || sub.LinkState != "available" {
					t.Fatal("valid link not recovered")
				}
			} else if sub.SubscriptionPath != "" || sub.LinkState != "unavailable" {
				t.Fatal("invalid ciphertext exposed")
			}
			if len(sub.EncryptedToken) > 0 || sub.TokenHash != "" {
				t.Fatal("secret intermediates retained")
			}
			body, _ := json.Marshal(sub)
			if strings.Contains(string(body), machine.Hash(token)) {
				t.Fatal("hash leaked")
			}
		})
	}
	legacy := storage.Subscription{ID: id, LinkState: "legacy"}
	a.revealSubscriptionLink(org, &legacy)
	if legacy.SubscriptionPath != "" || legacy.LinkState != "legacy" {
		t.Fatal("legacy URL invented or silently rotated")
	}
}
