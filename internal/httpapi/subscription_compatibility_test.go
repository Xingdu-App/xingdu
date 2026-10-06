package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

type compatibilityFake struct {
	fakeStore
	deps []storage.Deployment
}

func (s compatibilityFake) SubscriptionCandidates(context.Context, []string) ([]storage.Deployment, error) {
	return s.deps, nil
}
func TestCompatibilityUsesEncryptedTransportAndTrustBoundaries(t *testing.T) {
	v, _ := vault.New(strings.Repeat("a", 64))
	spec, err := protocol.NewSpec(protocol.Input{Name: "Fixture", Protocol: "vless", Port: 443, ServerName: "www.microsoft.com", V2Ray: &protocol.V2RayOptions{Network: "tcp", Reality: true, Flow: "xtls-rprx-vision"}})
	if err != nil {
		t.Fatal(err)
	}
	d := storage.Deployment{ID: storage.NewID("node"), HostID: storage.NewID("srv"), OrgID: storage.NewID("org"), Name: "Fixture", Server: "192.0.2.1"}
	for _, engine := range []string{"sing-box", "xray"} {
		spec.V2Ray.Engine = engine
		plain, _ := json.Marshal(spec)
		d.Encrypted = v.Seal(plain, deploymentAAD(d.OrgID, d.HostID, d.ID))
		clear(plain)
		a := &api{store: compatibilityFake{deps: []storage.Deployment{d}}, vault: v}
		for _, format := range []string{"stash", "mihomo", "singbox", "uri"} {
			draft := storage.Subscription{Format: format, NodeIDs: []string{d.ID}, FinalAction: "proxy"}
			report, err := a.compatibility(context.Background(), draft, draft.NodeIDs)
			if err != nil {
				t.Fatal(err)
			}
			want := format == "uri" || engine == "sing-box" && format != "stash"
			if len(report.Nodes) != 1 || report.Nodes[0].Compatible != want || ((len(report.Issues) == 0) != want) {
				t.Fatal("transport compatibility mismatch", engine, format)
			}
			body, _ := json.Marshal(report)
			for _, secret := range []string{spec.Credential, spec.RealityPrivateKey} {
				if strings.Contains(string(body), secret) {
					t.Fatal("credential leaked")
				}
			}
		}
		missing := storage.NewID("node")
		report, err := a.compatibility(context.Background(), storage.Subscription{Format: "uri", NodeIDs: []string{missing}, FinalAction: "proxy"}, []string{missing})
		if err != nil || report.Nodes[0].Compatible || len(report.Issues) == 0 {
			t.Fatal("missing candidate accepted", err)
		}
	}
}
