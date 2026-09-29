package subscription

import (
	"encoding/json"
	"os"
	"testing"
)

func TestBundledRulePresets(t *testing.T) {
	b, err := os.ReadFile("../../apps/web/src/rule-presets.json")
	if err != nil {
		t.Fatal(err)
	}
	var presets []struct {
		ID    string `json:"id"`
		Rules []Rule `json:"rules"`
		Final string `json:"final_action"`
	}
	if err = json.Unmarshal(b, &presets); err != nil {
		t.Fatal(err)
	}
	if len(presets) < 3 {
		t.Fatal("missing presets")
	}
	ids := map[string]bool{}
	for _, p := range presets {
		if ids[p.ID] {
			t.Fatal("duplicate preset")
		}
		ids[p.ID] = true
		if err = ValidateRules(p.Rules, p.Final); err != nil {
			t.Fatal(p.ID, err)
		}
		for _, format := range []string{"stash", "mihomo", "surge"} {
			if _, err = Render(format, "Preset", []Node{fixture(t, "trojan", 1)}, p.Rules, p.Final); err != nil {
				t.Fatal(p.ID, format, err)
			}
		}
	}
}
