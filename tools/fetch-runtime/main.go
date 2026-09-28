// fetch-runtime acquires only the pinned upstream release. Archives and binaries
// are both verified before publishing any executable into the artifact folder.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"xingdu.app/xingdu/internal/protocol"
)

func main() {
	output := flag.String("output", "bin/agents", "artifact directory")
	cache := flag.String("cache", "", "optional local release archive cache")
	flag.Parse()
	if err := run(*output, *cache); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func matches(b []byte, expected string) bool {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]) == expected
}
func run(output, cache string) error {
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	for _, arch := range []string{"amd64", "arm64"} {
		destination := filepath.Join(output, "sing-box-linux-"+arch)
		if b, e := os.ReadFile(destination); e == nil && matches(b, protocol.RuntimeSHA256[arch]) {
			if license, e := os.ReadFile(filepath.Join(output, "sing-box-LICENSE")); e == nil && len(license) > 0 {
				continue
			}
		}
		name := "sing-box-" + protocol.RuntimeVersion + "-linux-" + arch
		var archive []byte
		if cache != "" {
			archive, _ = os.ReadFile(filepath.Join(cache, name+".tar.gz"))
		}
		if len(archive) == 0 {
			client := http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
				if r.URL.Scheme != "https" || len(via) > 5 {
					return errors.New("unsafe runtime redirect")
				}
				return nil
			}}
			response, err := client.Get("https://github.com/SagerNet/sing-box/releases/download/v" + protocol.RuntimeVersion + "/" + name + ".tar.gz")
			if err != nil {
				return errors.New("runtime download failed")
			}
			if response.StatusCode != 200 {
				response.Body.Close()
				return errors.New("runtime download rejected")
			}
			archive, err = io.ReadAll(io.LimitReader(response.Body, 128<<20))
			response.Body.Close()
			if err != nil {
				return err
			}
		}
		if !matches(archive, protocol.RuntimeArchiveSHA256[arch]) {
			return errors.New("runtime archive checksum mismatch: " + arch)
		}
		gz, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return err
		}
		tr := tar.NewReader(gz)
		var binary, license []byte
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				gz.Close()
				return err
			}
			if h.Name != name+"/sing-box" && h.Name != name+"/LICENSE" {
				continue
			}
			if h.Typeflag != tar.TypeReg || h.Size <= 0 || h.Size > 128<<20 {
				gz.Close()
				return errors.New("invalid runtime archive member")
			}
			data, err := io.ReadAll(io.LimitReader(tr, 128<<20))
			if err != nil {
				gz.Close()
				return err
			}
			if h.Name == name+"/sing-box" {
				binary = data
			} else {
				license = data
			}
		}
		gz.Close()
		if !matches(binary, protocol.RuntimeSHA256[arch]) || len(license) == 0 {
			return errors.New("runtime binary checksum or license missing: " + arch)
		}
		if err = os.WriteFile(filepath.Join(output, "sing-box-LICENSE"), license, 0644); err != nil {
			return err
		}
		f, err := os.CreateTemp(output, ".runtime-*")
		if err != nil {
			return err
		}
		tmp := f.Name()
		_, err = f.Write(binary)
		if err == nil {
			err = f.Chmod(0755)
		}
		if ce := f.Close(); err == nil {
			err = ce
		}
		if err == nil {
			err = os.Rename(tmp, destination)
		}
		os.Remove(tmp)
		if err != nil {
			return err
		}
		fmt.Println("Verified sing-box " + protocol.RuntimeVersion + " linux/" + arch)
	}
	return nil
}
