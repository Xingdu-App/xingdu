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
	"os/exec"
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
	for _, family := range []string{"trusttunnel", "sing-box-legacy"} {
		for _, arch := range []string{"amd64", "arm64"} {
			version, hashes, archives := protocol.LegacyRuntimeVersion, protocol.LegacyRuntimeSHA256, protocol.RuntimeArchiveSHA256
			name := "sing-box-" + version + "-linux-" + arch
			binaryName, repo := "sing-box", "SagerNet/sing-box"
			if family == "trusttunnel" {
				version, hashes, archives = protocol.TrustTunnelVersion, protocol.TrustTunnelSHA256, protocol.TrustTunnelArchiveSHA256
				upstreamArch := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[arch]
				name = "trusttunnel-v" + version + "-linux-" + upstreamArch
				binaryName, repo = "trusttunnel_endpoint", "TrustTunnel/TrustTunnel"
			}
			destination := filepath.Join(output, family+"-linux-"+arch)
			if b, e := os.ReadFile(destination); e == nil && matches(b, hashes[arch]) {
				if license, e := os.ReadFile(filepath.Join(output, family+"-LICENSE")); e == nil && len(license) > 0 {
					continue
				}
			}
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
				response, err := client.Get("https://github.com/" + repo + "/releases/download/v" + version + "/" + name + ".tar.gz")
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
			if !matches(archive, archives[arch]) {
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
				if h.Name != name+"/"+binaryName && h.Name != name+"/LICENSE" {
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
				if h.Name == name+"/"+binaryName {
					binary = data
				} else {
					license = data
				}
			}
			gz.Close()
			if !matches(binary, hashes[arch]) || len(license) == 0 {
				return errors.New("runtime binary checksum or license missing: " + arch)
			}
			if err = os.WriteFile(filepath.Join(output, family+"-LICENSE"), license, 0644); err != nil {
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
			fmt.Println("Verified " + family + " " + version + " linux/" + arch)
		}
	}
	if err := fetchOfficial(output, cache); err != nil {
		return err
	}
	for _, family := range []string{"sing-box", "xray"} {
		for _, arch := range []string{"amd64", "arm64"} {
			hashes := protocol.HardenedRuntimeSHA256
			if family == "xray" {
				hashes = protocol.HardenedXraySHA256
			}
			path := filepath.Join(output, family+"-linux-"+arch)
			if b, err := os.ReadFile(path); err == nil && matches(b, hashes[arch]) {
				_, licenseErr := os.Stat(filepath.Join(output, family+"-LICENSE"))
				_, sourceErr := os.Stat(filepath.Join(output, family+"-source.tar.gz"))
				if licenseErr == nil && sourceErr == nil {
					if err := os.Chmod(filepath.Join(output, family+"-source.tar.gz"), 0644); err != nil {
						return err
					}
					continue
				}
			}
			script := "tools/build-runtime/build.sh"
			if _, err := os.Stat(script); err != nil {
				script = "../build-runtime/build.sh"
			}
			cmd := exec.Command("sh", script, family, arch, output)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil || !matches(b, hashes[arch]) {
				return errors.New("built runtime checksum mismatch: " + family + "/" + arch)
			}
		}
	}
	return nil
}
