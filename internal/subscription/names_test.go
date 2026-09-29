package subscription

import (
	"go.yaml.in/yaml/v3"
	"reflect"
	"strings"
	"testing"
)

func TestExportNamesWithoutResourceIDs(t *testing.T) {
	nodes := []Node{fixture(t, "shadowsocks", 1), fixture(t, "shadowsocks", 2), fixture(t, "shadowsocks", 3), fixture(t, "shadowsocks", 4), fixture(t, "shadowsocks", 5)}
	for i, name := range []string{"Tokyo", "Tokyo", "Tokyo - 2", "DIRECT", "Xingdu"} {
		nodes[i].Name = name
	}
	want := []string{"Tokyo", "Tokyo - 3", "Tokyo - 2", "DIRECT - 2", "Xingdu - 2"}
	if got := nodeLabels(nodes, false); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	for _, format := range []string{"stash", "mihomo", "surge"} {
		b, err := Render(format, "Demo", nodes, nil, "proxy")
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range nodes {
			if strings.Contains(string(b), n.ID) {
				t.Fatal("resource ID in export", format)
			}
		}
		if format != "surge" {
			var cfg struct {
				Proxies []map[string]any `yaml:"proxies"`
			}
			if err = yaml.Unmarshal(b, &cfg); err != nil {
				t.Fatal(err)
			}
			for i, p := range cfg.Proxies {
				if p["name"] != want[i] {
					t.Fatal(p["name"])
				}
			}
		}
	}
	nodes[0].Name = "A,B"
	nodes[1].Name = "A=B"
	names := nodeLabels(nodes, true)
	if names[0] == names[1] || strings.ContainsAny(names[0]+names[1], ",=") {
		t.Fatal(names)
	}
}
