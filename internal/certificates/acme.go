// Package certificates issues DNS-01 certificates using operator-owned DNS.
package certificates

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"golang.org/x/crypto/acme"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const Staging = "https://acme-staging-v02.api.letsencrypt.org/directory"
const Production = "https://acme-v02.api.letsencrypt.org/directory"

var zoneID = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
var domainName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

type Bundle struct {
	Domain      string `json:"domain,omitempty"`
	Directory   string `json:"directory,omitempty"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
}
type DNS interface {
	Present(context.Context, string, string) (string, error)
	Remove(context.Context, string) error
}
type Cloudflare struct {
	Token, Zone string
	Client      *http.Client
}

func (c Cloudflare) call(ctx context.Context, method, path string, in, out any) error {
	if !zoneID.MatchString(c.Zone) || c.Token == "" {
		return errors.New("invalid DNS provider configuration")
	}
	b, _ := json.Marshal(in)
	req, _ := http.NewRequestWithContext(ctx, method, "https://api.cloudflare.com/client/v4/zones/"+c.Zone+path, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, err := client.Do(req)
	if err != nil {
		return errors.New("DNS provider unavailable")
	}
	defer res.Body.Close()
	var envelope struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
	}
	if res.StatusCode >= 300 || json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&envelope) != nil || !envelope.Success {
		return errors.New("DNS provider rejected operation")
	}
	if out != nil {
		return json.Unmarshal(envelope.Result, out)
	}
	return nil
}
func (c Cloudflare) Present(ctx context.Context, name, value string) (string, error) {
	var zone struct {
		Name string `json:"name"`
	}
	if err := c.call(ctx, "GET", "", nil, &zone); err != nil {
		return "", err
	}
	if !strings.HasPrefix(name, "_acme-challenge.") || !strings.HasSuffix(name, "."+zone.Name) {
		return "", errors.New("challenge outside configured zone")
	}
	var record struct {
		ID string `json:"id"`
	}
	err := c.call(ctx, "POST", "/dns_records", map[string]any{"type": "TXT", "name": name, "content": value, "ttl": 60}, &record)
	if err == nil && !zoneID.MatchString(record.ID) {
		err = errors.New("invalid DNS record ID")
	}
	return record.ID, err
}
func (c Cloudflare) Remove(ctx context.Context, id string) error {
	if !zoneID.MatchString(id) {
		return errors.New("invalid record ID")
	}
	return c.call(ctx, "DELETE", "/dns_records/"+id, nil, nil)
}
func AccountKey(dir string) (*ecdsa.PrivateKey, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	st, err := os.Lstat(dir)
	if err != nil || !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private state directory required")
	}
	path := filepath.Join(dir, "account.pem")
	st, err = os.Lstat(path)
	if err == nil {
		if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
			return nil, errors.New("unsafe account file")
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return nil, e
		}
		defer clear(b)
		block, _ := pem.Decode(b)
		if block == nil {
			return nil, errors.New("invalid account key")
		}
		return x509.ParseECPrivateKey(block.Bytes)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, _ := x509.MarshalECPrivateKey(key)
	b := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	defer clear(b)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err != nil {
		return nil, err
	}
	return key, ce
}
func Issue(ctx context.Context, client *acme.Client, dns DNS, domain, email string) (Bundle, error) {
	return issue(ctx, client, dns, domain, email, net.DefaultResolver.LookupTXT)
}
func issue(ctx context.Context, client *acme.Client, dns DNS, domain, email string, lookup func(context.Context, string) ([]string, error)) (Bundle, error) {
	if len(domain) > 253 || !domainName.MatchString(domain) {
		return Bundle{}, errors.New("invalid domain")
	}
	if _, err := client.Register(ctx, &acme.Account{Contact: []string{"mailto:" + email}}, acme.AcceptTOS); err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
		return Bundle{}, errors.New("ACME account unavailable")
	}
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(domain))
	if err != nil {
		return Bundle{}, errors.New("ACME order rejected")
	}
	for _, authorization := range order.AuthzURLs {
		auth, err := client.GetAuthorization(ctx, authorization)
		if err != nil {
			return Bundle{}, errors.New("ACME authorization unavailable")
		}
		if auth.Identifier.Value != domain || auth.Wildcard {
			return Bundle{}, errors.New("unexpected authorization")
		}
		if auth.Status == acme.StatusValid {
			continue
		}
		var challenge *acme.Challenge
		for _, c := range auth.Challenges {
			if c.Type == "dns-01" {
				challenge = c
				break
			}
		}
		if challenge == nil {
			return Bundle{}, errors.New("DNS challenge unavailable")
		}
		value, err := client.DNS01ChallengeRecord(challenge.Token)
		if err != nil {
			return Bundle{}, err
		}
		name := "_acme-challenge." + domain
		id, err := dns.Present(ctx, name, value)
		if err != nil {
			return Bundle{}, err
		}
		// Cleanup uses its own bounded context, including after cancellation.
		err = func() (operationErr error) {
			defer func() {
				cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				if e := dns.Remove(cleanup, id); e != nil && operationErr == nil {
					operationErr = errors.New("DNS challenge cleanup failed")
				}
			}()
			propagated := false
			for i := 0; i < 60; i++ {
				records, _ := lookup(ctx, name)
				for _, r := range records {
					if r == value {
						propagated = true
					}
				}
				if propagated {
					break
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
			}
			if !propagated {
				return errors.New("DNS propagation timed out")
			}
			if _, err := client.Accept(ctx, challenge); err != nil {
				return errors.New("ACME challenge rejected")
			}
			if _, err := client.WaitAuthorization(ctx, authorization); err != nil {
				return errors.New("ACME validation failed")
			}
			return nil
		}()
		if err != nil {
			return Bundle{}, err
		}
	}
	if _, err = client.WaitOrder(ctx, order.URI); err != nil {
		return Bundle{}, errors.New("ACME order not ready")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Bundle{}, err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{DNSNames: []string{domain}}, key)
	if err != nil {
		return Bundle{}, err
	}
	chain, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return Bundle{}, errors.New("ACME issuance failed")
	}
	out := Bundle{}
	for _, der := range chain {
		out.Certificate += string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	}
	der, _ := x509.MarshalECPrivateKey(key)
	out.PrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
	return out, nil
}
func Save(dir string, bundle Bundle) error {
	b, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	defer clear(b)
	f, err := os.CreateTemp(dir, ".certificate-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err != nil {
		return err
	}
	if ce != nil {
		return ce
	}
	return os.Rename(f.Name(), filepath.Join(dir, "certificate.json"))
}
