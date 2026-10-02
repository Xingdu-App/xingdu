package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

func fetchOfficial(output, cache string) error {
	// The legacy sing-box asset already is the official release, verified above.
	for _, arch := range []string{"amd64", "arm64"} {
		b, err := os.ReadFile(filepath.Join(output, "sing-box-legacy-linux-"+arch))
		if err != nil || !matches(b, protocol.RuntimeSHA256[arch]) {
			return errors.New("official sing-box checksum mismatch")
		}
		if err = os.WriteFile(filepath.Join(output, "official-sing-box-linux-"+arch), b, 0755); err != nil {
			return err
		}
		name := map[string]string{"amd64": "Xray-linux-64.zip", "arm64": "Xray-linux-arm64-v8a.zip"}[arch]
		target := filepath.Join(output, "official-xray-linux-"+arch)
		licenseCache, _ := os.ReadFile(filepath.Join(output, "official-xray-LICENSE"))
		if b, err := os.ReadFile(target); err == nil && matches(b, protocol.XraySHA256[arch]) && len(licenseCache) > 0 {
			continue
		}
		var archive []byte
		if cache != "" {
			archive, _ = os.ReadFile(filepath.Join(cache, name))
		}
		if len(archive) == 0 {
			client := http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
				if r.URL.Scheme != "https" || len(via) > 5 {
					return errors.New("unsafe runtime redirect")
				}
				return nil
			}}
			res, err := client.Get("https://github.com/XTLS/Xray-core/releases/download/v" + protocol.XrayVersion + "/" + name)
			if err != nil {
				return errors.New("official runtime download failed")
			}
			if res.StatusCode != 200 {
				res.Body.Close()
				return errors.New("official runtime download rejected")
			}
			archive, err = io.ReadAll(io.LimitReader(res.Body, 128<<20))
			res.Body.Close()
			if err != nil {
				return err
			}
		}
		binary, license, err := officialXray(archive, protocol.OfficialXrayArchiveSHA256[arch], protocol.XraySHA256[arch])
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(output, "official-xray-LICENSE"), license, 0644); err != nil {
			return err
		}
		// Atomic publication avoids exposing a partially written executable.
		f, err := os.CreateTemp(output, ".official-*")
		if err != nil {
			return err
		}
		tmp := f.Name()
		_, err = f.Write(binary)
		if err == nil {
			err = f.Chmod(0755)
		}
		if e := f.Close(); err == nil {
			err = e
		}
		if err == nil {
			err = os.Rename(tmp, target)
		}
		os.Remove(tmp)
		if err != nil {
			return err
		}
	}
	return nil
}
func officialXray(archive []byte, archiveHash, binaryHash string) ([]byte, []byte, error) {
	if !matches(archive, archiveHash) {
		return nil, nil, errors.New("official runtime archive checksum mismatch")
	}
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, nil, err
	}
	var binary, license []byte
	for _, f := range z.File {
		if f.Name != "xray" && f.Name != "LICENSE" {
			continue
		}
		if !f.Mode().IsRegular() || f.UncompressedSize64 == 0 || f.UncompressedSize64 > 128<<20 {
			return nil, nil, errors.New("invalid official runtime member")
		}
		r, e := f.Open()
		if e != nil {
			return nil, nil, e
		}
		b, e := io.ReadAll(io.LimitReader(r, 128<<20))
		r.Close()
		if e != nil {
			return nil, nil, e
		}
		if f.Name == "xray" {
			if binary != nil {
				return nil, nil, errors.New("duplicate runtime")
			}
			binary = b
		} else {
			license = b
		}
	}
	if !matches(binary, binaryHash) || len(license) == 0 {
		return nil, nil, errors.New("official runtime binary checksum or license missing")
	}
	return binary, license, nil
}
