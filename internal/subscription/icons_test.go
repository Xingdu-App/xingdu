package subscription

import (
	"strings"
	"testing"
)

func TestIconURLValidation(t *testing.T) {
	for _, s := range []string{"https://assets.example.com/icon.png", ""} {
		if !ValidIconURL(s) {
			t.Fatal("valid icon rejected", s)
		}
	}
	for _, s := range []string{"http://assets.example.com/a.png", "https://user:pass@assets.example.com/a.png", "https://localhost/a.png", "https://127.0.0.1/a.png", "https://10.0.0.1/a.png", "data:image/png;base64,xxx", "https://assets.example.com/a.png\n", "https://assets.example.com/a.png#fragment", strings.Repeat("x", 2049)} {
		if ValidIconURL(s) {
			t.Fatal("unsafe icon accepted", s)
		}
	}
}
func TestIconWarnings(t *testing.T) {
	r := &Routing{Groups: []RoutingGroup{{Icon: "https://assets.example.com/a.png"}}}
	if len(IconWarnings(r, "stash")) != 0 || len(IconWarnings(r, "surge")) != 1 || len(IconWarnings(r, "mihomo")) != 1 {
		t.Fatal("wrong icon capabilities")
	}
}

func TestGroupIconExport(t *testing.T) {
	node := fixture(t, "shadowsocks", 1)
	r := routingFixture(t, "balanced-v1")
	r.Groups[0].Icon = "https://assets.example.com/icon.png"
	for _, format := range []string{"stash", "mihomo", "surge"} {
		data, err := RenderRouting(format, "Example", []Node{node}, nil, "proxy", r, []string{node.ID})
		if err != nil {
			t.Fatal(format, err)
		}
		present := strings.Contains(string(data), r.Groups[0].Icon)
		if present != (format == "stash" || format == "mihomo") {
			t.Fatal("wrong icon export", format)
		}
	}
}
