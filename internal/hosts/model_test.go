package hosts

import "testing"

func TestValidateHost(t *testing.T) {
	base := Input{Name: "  东京  ", Address: "VPS.Example.COM", SSHPort: 22, SSHUser: "root", Tags: []string{"日本", "日本"}}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	if base.Name != "东京" || base.Address != "vps.example.com" || len(base.Tags) != 1 {
		t.Fatalf("not normalized: %+v", base)
	}
	for _, address := range []string{"https://example.com", "example.com:22", "example.com/path", "-bad.example", "example.com;id", "fe80::1%en0"} {
		in := base
		in.Address = address
		if in.Validate() == nil {
			t.Errorf("accepted %s", address)
		}
	}
	in := base
	in.Address = "2001:db8::1"
	if err := in.Validate(); err != nil {
		t.Fatal(err)
	}
	in = base
	in.SSHPort = 0
	if in.Validate() == nil {
		t.Fatal("accepted invalid port")
	}
	in = base
	in.SSHUser = "root;id"
	if in.Validate() == nil {
		t.Fatal("accepted shell-like username")
	}
}
