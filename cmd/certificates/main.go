// Run once from a daily systemd timer. With no node target, write the initial
// certificate to private state; with a target, renew and queue a versioned update.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/acme"
	"xingdu.app/xingdu/internal/certificates"
	"xingdu.app/xingdu/internal/id"
	"xingdu.app/xingdu/internal/protocol"
)

func main() {
	if run() != nil {
		fmt.Fprintln(os.Stderr, "certificate task failed; check provider configuration, domain authorization and node state")
		os.Exit(1)
	}
}
func run() error {
	dir, domain, email := os.Getenv("XINGDU_ACME_STATE"), os.Getenv("XINGDU_ACME_DOMAIN"), os.Getenv("XINGDU_ACME_EMAIL")
	if dir == "" || email == "" || os.Getenv("XINGDU_ACME_ACCEPT_TOS") != "true" {
		return fmt.Errorf("explicit configuration required")
	}
	key, err := certificates.AccountKey(dir)
	if err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return fmt.Errorf("busy")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	bundle := certificates.Bundle{}
	path := filepath.Join(dir, "certificate.json")
	if st, e := os.Lstat(path); e == nil {
		if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("unsafe certificate file")
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		e = json.Unmarshal(b, &bundle)
		clear(b)
		if e != nil {
			return e
		}
	}
	directory := certificates.Staging
	if os.Getenv("XINGDU_ACME_PRODUCTION") == "true" {
		directory = certificates.Production
	}
	if bundle.Certificate != "" && (bundle.Domain != domain || bundle.Directory != directory) {
		return fmt.Errorf("certificate state belongs to another domain or CA; use separate state")
	}
	expiry := protocol.CertificateExpiry(bundle.Certificate)
	if expiry == nil || time.Until(*expiry) < 30*24*time.Hour {
		client := &acme.Client{Key: key, DirectoryURL: directory, HTTPClient: &http.Client{Timeout: 45 * time.Second}}
		bundle, err = certificates.Issue(ctx, client, certificates.Cloudflare{Token: os.Getenv("XINGDU_ACME_DNS_TOKEN"), Zone: os.Getenv("XINGDU_ACME_ZONE")}, domain, email)
		if err != nil {
			return err
		}
		if err = protocol.ValidateInput(protocol.Input{Name: "certificate", Protocol: "trojan", Port: 443, ServerName: domain, Certificate: bundle.Certificate, PrivateKey: bundle.PrivateKey}); err != nil {
			return err
		}
		bundle.Domain, bundle.Directory = domain, directory
		if err = certificates.Save(dir, bundle); err != nil {
			return err
		}
	}
	if err = protocol.ValidateInput(protocol.Input{Name: "certificate", Protocol: "trojan", Port: 443, ServerName: domain, Certificate: bundle.Certificate, PrivateKey: bundle.PrivateKey}); err != nil {
		return err
	}
	host, node := os.Getenv("XINGDU_ACME_HOST"), os.Getenv("XINGDU_ACME_NODE")
	if host == "" && node == "" {
		return nil
	}
	if !id.Valid("srv", host) || !id.Valid("node", node) {
		return fmt.Errorf("invalid node")
	}
	origin, token, org := os.Getenv("XINGDU_ACME_API"), os.Getenv("XINGDU_ACME_API_KEY"), os.Getenv("XINGDU_ACME_ORG")
	u, e := url.Parse(origin)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || token == "" || org == "" {
		return fmt.Errorf("invalid API configuration")
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(method, path string, input any, out any) error {
		b, _ := json.Marshal(input)
		defer clear(b)
		r, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(origin, "/")+path, bytes.NewReader(b))
		if e != nil {
			return fmt.Errorf("invalid API request")
		}
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Xingdu-Organization", org)
		r.Header.Set("Content-Type", "application/json")
		res, e := client.Do(r)
		if e != nil {
			return fmt.Errorf("API unavailable")
		}
		defer res.Body.Close()
		if res.StatusCode >= 300 {
			return fmt.Errorf("API rejected")
		}
		if out != nil {
			return json.NewDecoder(io.LimitReader(res.Body, 128<<10)).Decode(out)
		}
		return nil
	}
	base := "/api/v1/hosts/" + host + "/deployments/" + node
	var current struct {
		Data struct {
			Certificate string `json:"certificate"`
			ServerName  string `json:"server_name"`
		} `json:"data"`
	}
	if err = call("POST", base+"/connection", nil, &current); err != nil {
		return err
	}
	if current.Data.ServerName != domain {
		return fmt.Errorf("domain does not match target")
	}
	if current.Data.Certificate == bundle.Certificate {
		return nil
	}
	return call("PUT", base, map[string]any{"certificate": bundle.Certificate, "private_key": bundle.PrivateKey, "confirm": true}, nil)
}
