package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRealityTargetRejectsUnsafeTargets(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "internal.invalid", "www.microsoft.com:443"} {
		if _, err := resolveRealityTarget(context.Background(), host); err == nil {
			t.Fatal("unapproved host accepted")
		}
	}
	cfg := []byte(`{"inbounds":[{"streamSettings":{"realitySettings":{"target":"www.microsoft.com:443","shortIds":["fixture"]}}}]}`)
	for _, target := range []string{"127.0.0.1:443", "[::ffff:127.0.0.1]:443", "10.0.0.1:443", "www.microsoft.com:443", "93.184.216.34:80"} {
		if _, err := pinRealityTarget(cfg, target); err == nil {
			t.Fatal("unsafe target accepted")
		}
	}
	for _, cfg := range []string{`{}`, `{"inbounds":[null]}`, `{"inbounds":[{}]}`, `{"inbounds":[{"streamSettings":{}}]}`} {
		if _, err := pinRealityTarget([]byte(cfg), "93.184.216.34:443"); err == nil {
			t.Fatal("invalid renderer output accepted")
		}
	}
	data, err := pinRealityTarget(cfg, "93.184.216.34:443")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Inbounds []struct {
			StreamSettings struct {
				RealitySettings struct {
					Target   string
					ShortIds []string
				}
			}
		}
	}
	if err = json.Unmarshal(data, &result); err != nil || result.Inbounds[0].StreamSettings.RealitySettings.Target != "93.184.216.34:443" || result.Inbounds[0].StreamSettings.RealitySettings.ShortIds[0] != "fixture" {
		t.Fatal("pinning changed handshake parameters", err)
	}
}
