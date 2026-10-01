package httpapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	"xingdu.app/xingdu/internal/hosts"
	"xingdu.app/xingdu/internal/machine"
	"xingdu.app/xingdu/internal/protocol"
	"xingdu.app/xingdu/internal/storage"
	"xingdu.app/xingdu/internal/vault"
)

func TestDeploymentHTTPSecretsAndMachineBoundary(t *testing.T) {
	for _, kind := range []string{"trojan", "vless", "vmess"} {
		t.Run(kind, func(t *testing.T) { testDeploymentHTTPSecretsAndMachineBoundary(t, kind) })
	}
}
func testDeploymentHTTPSecretsAndMachineBoundary(t *testing.T, kind string) {
	raw := os.Getenv("XINGDU_TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("requires dedicated database")
	}
	ctx := context.Background()
	db, e := storage.Open(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	uri, _ := url.Parse(raw)
	q := uri.Query()
	q.Set("options", "-crole=xingdu_app")
	uri.RawQuery = q.Encode()
	s, e := storage.Open(ctx, uri.String())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	uid, e := s.Register(ctx, "httpdeploy_"+storage.NewID("obj")[4:20], "unused", "Deploy HTTP")
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		db.Pool.Exec(ctx, "DELETE FROM organizations WHERE created_by=$1", uid)
		db.Pool.Exec(ctx, "DELETE FROM users WHERE id=$1", uid)
	}()
	orgs, _ := s.Organizations(ctx, uid)
	org := orgs[0].ID
	sc := storage.WithTenant(ctx, uid, org)
	host, e := s.CreateHost(sc, hosts.Input{Name: "protocol", Address: "test.example.invalid", SSHPort: 22, SSHUser: "root", Tags: []string{}})
	if e != nil {
		t.Fatal(e)
	}
	enrollment, identity := machine.Token(), machine.Token()
	s.Enrollment(sc, host.ID, "manage", machine.Hash(enrollment))
	if e = s.EnrollAgent(ctx, machine.Hash(enrollment), machine.Hash(identity), "manage"); e != nil {
		t.Fatal(e)
	}
	if e = s.Heartbeat(ctx, machine.Hash(identity), machine.Metrics{Version: machine.Version, CPUs: 1}); e != nil {
		t.Fatal(e)
	}
	session := machine.Token()
	if e = s.NewSession(ctx, tokenHash(session), uid, time.Now().Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	v, _ := vault.New(strings.Repeat("ab", 32))
	handler := New(s, Options{PublicOrigin: "http://127.0.0.1:15173", CredentialVault: v})
	requestMethod := func(method, path string, body any, agent bool, want int) *httptest.ResponseRecorder {
		t.Helper()
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if agent {
			r.Header.Set("Authorization", "Bearer "+identity)
		} else {
			r.Header.Set("Origin", "http://127.0.0.1:15173")
			r.Header.Set("X-Xingdu-Request", "1")
			r.Header.Set("X-CSRF-Token", csrfToken(session))
			r.Header.Set("X-Xingdu-Organization", org)
			r.AddCookie(&http.Cookie{Name: cookieName, Value: session})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s: %d want %d: %s", path, w.Code, want, w.Body.String())
		}
		return w
	}
	request := func(path string, body any, agent bool, want int) *httptest.ResponseRecorder {
		t.Helper()
		return requestMethod("POST", path, body, agent, want)
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test.example.invalid"}, DNSNames: []string{"test.example.invalid"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, _ := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	body := map[string]any{"name": "test", "protocol": kind, "port": 443, "server_name": "test.example.invalid", "certificate": certPEM, "private_key": keyPEM, "confirm_install": true}
	if kind != "trojan" {
		body["v2ray"] = map[string]any{"network": "ws", "path": "/original", "host": "test.example.invalid"}
	}
	path := "/api/v1/hosts/" + host.ID + "/deployments"
	preflight := map[string]any{}
	for k, v := range body {
		if k != "confirm_install" {
			preflight[k] = v
		}
	}
	if kind != "trojan" {
		if e = s.Heartbeat(ctx, machine.Hash(identity), machine.Metrics{Version: "0.15.0-rc.1", CPUs: 1}); e != nil {
			t.Fatal(e)
		}
		request(path+"/preflight", preflight, false, 409)
		request(path, body, false, 409)
		if e = s.Heartbeat(ctx, machine.Hash(identity), machine.Metrics{Version: machine.Version, CPUs: 1}); e != nil {
			t.Fatal(e)
		}
	}
	checked := request(path+"/preflight", preflight, false, 200)
	if strings.Contains(checked.Body.String(), "PRIVATE KEY") || !strings.Contains(checked.Body.String(), "inventory_only") {
		t.Fatal("unsafe preflight")
	}
	created := request(path, body, false, 202)
	request(path+"/preflight", preflight, false, 409)
	var queued struct{ Data struct{ ID string } }
	json.Unmarshal(created.Body.Bytes(), &queued)
	var ciphertext []byte
	if e = db.Pool.QueryRow(ctx, "SELECT encrypted FROM protocol_deployments WHERE id=$1", queued.Data.ID).Scan(&ciphertext); e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(ciphertext, []byte("PRIVATE KEY")) {
		t.Fatal("plaintext stored")
	}
	if _, e = v.Open(ciphertext, deploymentAAD(storage.NewID("obj"), host.ID, queued.Data.ID)); e == nil {
		t.Fatal("cross tenant AAD accepted")
	}
	request("/api/v1/agent/deployments/claim", map[string]any{}, false, 403)
	w := request("/api/v1/agent/deployments/claim", map[string]any{}, true, 200)
	var claim struct{ Data protocol.Task }
	json.Unmarshal(w.Body.Bytes(), &claim)
	if claim.Data.DeploymentID != queued.Data.ID || claim.Data.Spec.PrivateKey != keyPEM || claim.Data.Spec.Credential == "" {
		t.Fatal("bad task")
	}
	if kind != "trojan" && (claim.Data.Spec.V2Ray == nil || claim.Data.Spec.V2Ray.Path != "/original") {
		t.Fatal("transport lost in encrypted task")
	}
	request("/api/v1/agent/deployments/result", protocol.Result{ID: claim.Data.ID, Lease: claim.Data.Lease, Success: false, Code: keyPEM}, true, 422)
	request("/api/v1/agent/deployments/result", protocol.Result{ID: claim.Data.ID, Lease: claim.Data.Lease, Success: true, Code: "deployed"}, true, 200)
	request("/api/v1/agent/deployments/result", protocol.Result{ID: claim.Data.ID, Lease: claim.Data.Lease, Success: true, Code: "deployed"}, true, 200)
	request("/api/v1/agent/deployments/result", protocol.Result{ID: claim.Data.ID, Lease: storage.NewID("lease"), Success: true, Code: "deployed"}, true, 409)
	w = request(path+"/"+queued.Data.ID+"/connection", map[string]any{}, false, 200)
	if strings.Contains(w.Body.String(), "private_key") || strings.Contains(w.Body.String(), "PRIVATE KEY") || !strings.Contains(w.Body.String(), claim.Data.Spec.Credential) {
		t.Fatal("unsafe or missing connection fields")
	}
	if kind != "trojan" {
		if !strings.Contains(w.Body.String(), `"path":"/original"`) {
			t.Fatal("transport missing from connection")
		}
		// Revision updates and restores must retain the exact options and credential.
		for _, restore := range []bool{false, true} {
			edit := map[string]any{"confirm": true, "v2ray": map[string]any{"network": "grpc", "service_name": "revised"}}
			if restore {
				edit = map[string]any{"confirm": true, "restore_revision": 1}
			}
			requestMethod("PUT", path+"/"+queued.Data.ID, edit, false, 202)
			update := request("/api/v1/agent/deployments/claim", map[string]any{}, true, 200)
			var task struct{ Data protocol.Task }
			if err := json.Unmarshal(update.Body.Bytes(), &task); err != nil {
				t.Fatal(err)
			}
			options := task.Data.Spec.V2Ray
			if options == nil || (!restore && options.ServiceName != "revised") || (restore && options.Path != "/original") || task.Data.Spec.Credential != claim.Data.Spec.Credential {
				t.Fatal("revision lost transport or credential")
			}
			request("/api/v1/agent/deployments/result", protocol.Result{ID: task.Data.ID, Lease: task.Data.Lease, Success: true, Code: "updated"}, true, 200)
		}
	}
	request("/api/v1/agent/deployments/status", map[string]any{"reports": []protocol.ServiceStatus{{ID: queued.Data.ID, Status: "active"}}}, true, 200)
	request("/api/v1/agent/deployments/status", map[string]any{"reports": []protocol.ServiceStatus{{ID: queued.Data.ID, Status: "active"}}}, false, 403)
	request(path+"/"+queued.Data.ID+"/restart", map[string]bool{"confirm": false}, false, 422)
	if err := s.Heartbeat(ctx, machine.Hash(identity), machine.Metrics{Version: "0.6.0-dev", CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	request(path+"/"+queued.Data.ID+"/restart", map[string]bool{"confirm": true}, false, 409)
	if err := s.Heartbeat(ctx, machine.Hash(identity), machine.Metrics{Version: machine.Version, CPUs: 1}); err != nil {
		t.Fatal(err)
	}
	list, e := s.Deployments(sc, host.ID)
	encoded, _ := json.Marshal(list)
	if e != nil || bytes.Contains(encoded, []byte(claim.Data.Spec.Credential)) || bytes.Contains(encoded, []byte("PRIVATE KEY")) {
		t.Fatal("secret in list", e)
	}
	// Organization-wide inventory is authenticated, sanitized and independent of a host path.
	getNodes := func(authenticated bool, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v1/nodes", nil)
		r.Header.Set("X-Xingdu-Organization", org)
		if authenticated {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: session})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("nodes status %d want %d: %s", w.Code, want, w.Body.String())
		}
		return w
	}
	getNodes(false, 401)
	w = getNodes(true, 200)
	var inventory struct{ Data []storage.Node }
	if e := json.Unmarshal(w.Body.Bytes(), &inventory); e != nil || len(inventory.Data) != 1 || inventory.Data[0].ID != queued.Data.ID || inventory.Data[0].InstalledAt.IsZero() {
		t.Fatal("node inventory", e)
	}
	if strings.Contains(w.Body.String(), claim.Data.Spec.Credential) || strings.Contains(w.Body.String(), "private_key") || strings.Contains(w.Body.String(), "encrypted") {
		t.Fatal("secret in node response")
	}
	if e = s.RemoveDeployment(sc, host.ID, queued.Data.ID); e != nil {
		t.Fatal(e)
	}
	handler = New(s, Options{PublicOrigin: "http://127.0.0.1:15173"})
	w = request("/api/v1/agent/deployments/claim", map[string]any{}, true, 200)
	var removal struct{ Data protocol.Task }
	json.Unmarshal(w.Body.Bytes(), &removal)
	if removal.Data.Action != "remove" || removal.Data.Spec.PrivateKey != "" || removal.Data.Spec.Credential != "" {
		t.Fatal("removal must not need secrets")
	}
	request("/api/v1/agent/deployments/result", protocol.Result{ID: removal.Data.ID, Lease: removal.Data.Lease, Success: true, Code: "removed"}, true, 200)
	w = getNodes(true, 200)
	if e := json.Unmarshal(w.Body.Bytes(), &inventory); e != nil || len(inventory.Data) != 0 {
		t.Fatal("removed node remains visible", e)
	}

}

func TestOpenDeploymentReplacesNestedRevisionFields(t *testing.T) {
	v, err := vault.New(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	a := &api{vault: v}
	for _, historical := range []protocol.Spec{
		{Input: protocol.Input{Protocol: "vless"}},
		{Input: protocol.Input{Protocol: "vless", V2Ray: &protocol.V2RayOptions{Network: "ws", Path: "/original"}}},
	} {
		active := protocol.Spec{Input: protocol.Input{Protocol: "vless", V2Ray: &protocol.V2RayOptions{Network: "grpc", ServiceName: "current"}}}
		plain, _ := json.Marshal(historical)
		d := storage.Deployment{ID: storage.NewID("node"), HostID: storage.NewID("srv")}
		org := storage.NewID("org")
		d.Encrypted = v.Seal(plain, deploymentAAD(org, d.HostID, d.ID))
		if !a.openDeployment(httptest.NewRecorder(), d, org, &active) {
			t.Fatal("decode failed")
		}
		if historical.V2Ray == nil && active.V2Ray != nil {
			t.Fatal("legacy revision inherited current transport")
		}
		if active.V2Ray != nil && (active.V2Ray.Network != "ws" || active.V2Ray.ServiceName != "") {
			t.Fatal("historical WS inherited gRPC service")
		}
	}
}
