package geoip

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const downloadURL = "https://download.maxmind.com/geoip/databases/GeoLite2-Country/download?suffix=tar.gz"
const downloadHost = "download.maxmind.com"
const storageHost = "mm-prod-geoip-databases.a2649acb697e2c09b632799562c076f2.r2.cloudflarestorage.com"

type credentials struct{ account, key string }

// Start initializes offline country lookup and optionally refreshes official data.
// Download credentials stay in this process and never enter request query strings.
func Start(ctx context.Context) error {
	path := os.Getenv("XINGDU_GEOLITE_DATABASE")
	auth := credentials{os.Getenv("XINGDU_MAXMIND_ACCOUNT_ID"), os.Getenv("XINGDU_MAXMIND_LICENSE_KEY")}
	os.Unsetenv("XINGDU_MAXMIND_ACCOUNT_ID")
	os.Unsetenv("XINGDU_MAXMIND_LICENSE_KEY")
	if auth.account == "" && auth.key == "" {
		return Configure(path)
	}
	if auth.account == "" || auth.key == "" {
		return errors.New("MaxMind updater requires both account ID and license key")
	}
	if path == "" {
		path = filepath.Join(os.TempDir(), "xingdu-geoip", "GeoLite2-Country.mmdb")
	}
	cached := Configure(path) == nil
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: checkRedirect}
	refresh := func() error {
		data, err := download(ctx, client, downloadURL, auth)
		if err != nil {
			return err
		}
		db, err := openGeoLite(data)
		if err != nil {
			return err
		}
		if err := persist(path, data); err != nil {
			db.Close()
			return errors.New("cannot replace GeoLite country database")
		}
		installReader(db)
		slog.Info("GeoLite country database refreshed", "build_date", db.Metadata.BuildTime().UTC().Format(time.DateOnly))
		return nil
	}
	if err := refresh(); err != nil {
		if !cached {
			return err
		}
		slog.Warn("GeoLite refresh failed; using validated local database", "error", err)
	}
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := refresh(); err != nil {
					slog.Warn("GeoLite refresh failed", "error", err)
				}
			}
		}
	}()
	return nil
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	// Basic auth must not follow the redirect to signed object-storage URLs.
	req.Header.Del("Authorization")
	if len(via) >= 5 || req.URL.Scheme != "https" || req.URL.User != nil ||
		(req.URL.Port() != "" && req.URL.Port() != "443") ||
		(req.URL.Hostname() != downloadHost && req.URL.Hostname() != storageHost) {
		return errors.New("unexpected MaxMind download redirect")
	}
	return nil
}

func download(ctx context.Context, client *http.Client, url string, auth credentials) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New("cannot create GeoLite download request")
	}
	req.SetBasicAuth(auth.account, auth.key)
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("GeoLite download request failed")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GeoLite download returned HTTP %d", res.StatusCode)
	}
	compressed, err := io.ReadAll(io.LimitReader(res.Body, (32<<20)+1))
	if err != nil || len(compressed) > 32<<20 {
		return nil, errors.New("invalid GeoLite download size")
	}
	return extractCountry(compressed)
}

func extractCountry(compressed []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, errors.New("invalid GeoLite archive")
	}
	defer gz.Close()
	raw, err := io.ReadAll(io.LimitReader(gz, (64<<20)+1))
	if err != nil || len(raw) > 64<<20 {
		return nil, errors.New("invalid GeoLite archive size or checksum")
	}
	tr := tar.NewReader(bytes.NewReader(raw))
	var data []byte
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("invalid GeoLite archive")
		}
		if filepath.Base(header.Name) != "GeoLite2-Country.mmdb" {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size < 1 || header.Size > 32<<20 || data != nil {
			return nil, errors.New("invalid GeoLite country archive entry")
		}
		data, err = io.ReadAll(tr)
		if err != nil || int64(len(data)) != header.Size {
			return nil, errors.New("incomplete GeoLite country archive entry")
		}
	}
	if data == nil {
		return nil, errors.New("GeoLite country database missing from archive")
	}
	return data, nil
}

func persist(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".country-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
