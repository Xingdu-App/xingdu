package protocol

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Optional interop check against an operator-provided official sing-box binary.
// Only the rendered peer server is replaced with the loopback mock endpoint;
// external endpoint policy is tested separately and never relaxed in production.
func TestExternalProxyOfficialRuntime(t *testing.T) {
	binary := os.Getenv("XINGDU_TEST_SINGBOX")
	if binary == "" {
		t.Skip("requires official sing-box binary")
	}
	for _, kind := range []string{"socks", "http"} {
		t.Run(kind, func(t *testing.T) {
			proxy, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer proxy.Close()
			received := make(chan error, 1)
			go func() {
				conn, err := proxy.Accept()
				if err != nil {
					received <- err
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(10 * time.Second))
				r := bufio.NewReader(conn)
				if kind == "http" {
					req, e := http.ReadRequest(r)
					if e != nil {
						received <- e
						return
					}
					if req.Method != "CONNECT" || req.Host != "93.184.216.34:80" || req.Header.Get("Proxy-Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("provider-user:short")) {
						received <- fmt.Errorf("HTTP CONNECT authentication or target mismatch")
						return
					}
					fmt.Fprint(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
				} else {
					head := make([]byte, 2)
					if _, err = io.ReadFull(r, head); err != nil {
						received <- err
						return
					}
					methods := make([]byte, int(head[1]))
					if _, err = io.ReadFull(r, methods); err != nil || head[0] != 5 {
						received <- fmt.Errorf("SOCKS negotiation failed")
						return
					}
					conn.Write([]byte{5, 2})
					if _, err = io.ReadFull(r, head); err != nil {
						received <- err
						return
					}
					user := make([]byte, int(head[1]))
					if _, err = io.ReadFull(r, user); err != nil {
						received <- err
						return
					}
					n, e := r.ReadByte()
					if e != nil {
						received <- e
						return
					}
					password := make([]byte, int(n))
					if _, err = io.ReadFull(r, password); err != nil || head[0] != 1 || string(user) != "provider-user" || string(password) != "short" {
						received <- fmt.Errorf("SOCKS credentials mismatch")
						return
					}
					conn.Write([]byte{1, 0})
					request := make([]byte, 10)
					if _, err = io.ReadFull(r, request); err != nil || request[0] != 5 || request[1] != 1 || request[3] != 1 || net.IP(request[4:8]).String() != "93.184.216.34" || request[8] != 0 || request[9] != 80 {
						received <- fmt.Errorf("SOCKS target mismatch")
						return
					}
					conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
				}
				req, e := http.ReadRequest(r)
				if e != nil {
					received <- e
					return
				}
				if req.URL.Path != "/external-exit-fixture" {
					received <- fmt.Errorf("unexpected request")
					return
				}
				fmt.Fprint(conn, "HTTP/1.1 200 OK\r\nContent-Length: 19\r\nConnection: close\r\n\r\nexternal-exit-good!")
				received <- nil
			}()
			s, err := NewSpec(Input{Name: "entry", Protocol: "socks", Port: 24443})
			if err != nil {
				t.Fatal(err)
			}
			s.Relay = &Peer{ExternalID: "ext_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Address: "93.184.216.34", Port: 1080, Protocol: kind, Username: "provider-user", Credential: "short"}
			raw, err := Render(s)
			if err != nil {
				t.Fatal(err)
			}
			var config map[string]any
			json.Unmarshal(raw, &config)
			config["outbounds"].([]any)[0].(map[string]any)["server"] = "127.0.0.1"
			config["outbounds"].([]any)[0].(map[string]any)["server_port"] = proxy.Addr().(*net.TCPAddr).Port
			reserve, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := reserve.Addr().(*net.TCPAddr).Port
			reserve.Close()
			inbound := config["inbounds"].([]any)[0].(map[string]any)
			inbound["listen"] = "127.0.0.1"
			inbound["listen_port"] = port
			path := filepath.Join(t.TempDir(), "runtime.json")
			raw, _ = json.Marshal(config)
			if err = os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if output, err := exec.CommandContext(ctx, binary, "check", "-c", path).CombinedOutput(); err != nil {
				t.Fatalf("official runtime rejected configuration: %v %s", err, output)
			}
			command := exec.CommandContext(ctx, binary, "run", "-c", path)
			if err = command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { cancel(); command.Wait() }()
			endpoint := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
			for i := 0; i < 100; i++ {
				conn, e := net.DialTimeout("tcp", endpoint, 50*time.Millisecond)
				if e == nil {
					conn.Close()
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			// net/http SOCKS transport handles entry credentials independently of exit credentials.
			proxyURL, err := url.Parse("socks5://xingdu:" + s.Credential + "@" + endpoint)
			if err != nil {
				t.Fatal(err)
			}
			transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
			resp, err := client.Get("http://93.184.216.34/external-exit-fixture")
			if err != nil {
				t.Fatal("entry to external proxy forwarding failed", err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 || string(body) != "external-exit-good!" {
				t.Fatal("unexpected tunnel response")
			}
			select {
			case err := <-received:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("mock proxy never authenticated")
			}
		})
	}
}
