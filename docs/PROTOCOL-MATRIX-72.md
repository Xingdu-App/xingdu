# 72-case protocol matrix

Source parameters are preserved in `internal/protocol/testdata/compatibility72.json`; endpoint addresses, names, paths and credentials are lab replacements. Every case has its own listener. No Hiddify installation is required.

Acceptance is tracked separately for local reference-client forwarding, AWS deployment/public forwarding, and Stash compatibility. Local Linux arm64 reference-client acceptance passed all 72 cases (929 seconds), plus four HTTPUpgrade combinations. AWS rollout is pending. No Stash app runtime result is implied.

Stash differences were checked against iOS Rust Core commit `febe040c`: `crates/config/src/proxies/vless.rs`, `crates/config/src/proxies/tuic.rs`, `crates/proxy/src/quic_client.rs`, and ADR 0024.

| Case | Protocol / transport | Upload / download | Additional parameter | Stash difference |
| --- | --- | --- | --- | --- |
| 1 | vless / ws | http/1.1 | — | Export candidate; app runtime unverified |
| 2 | vmess / ws | http/1.1 | — | Export candidate; app runtime unverified |
| 3 | vless / grpc | h2 | — | Export candidate; app runtime unverified |
| 4 | vmess / grpc | h2 | — | Export candidate; app runtime unverified |
| 5 | vmess / ws | plaintext | — | Export candidate; app runtime unverified |
| 6 | socks | plaintext | — | Export candidate; app runtime unverified |
| 7 | wireguard | WireGuard | MTU 1380, keepalive 25, PSK | Export candidate; app runtime unverified |
| 8 | tuic | TLS | — | Export candidate; app runtime unverified |
| 9 | hysteria2 | TLS | Salamander | Export candidate; app runtime unverified |
| 10 | anytls | TLS | — | Export candidate; app runtime unverified |
| 11 | vless / xhttp | plaintext → plaintext | VLESS Encryption, Vision | Export candidate; app runtime unverified |
| 12 | vless / xhttp | plaintext → plaintext | — | Export candidate; app runtime unverified |
| 13 | vless / xhttp | http/1.1 → http/1.1 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 14 | vless / xhttp | http/1.1 → http/1.1 | — | XHTTP requires h2/h2c |
| 15 | vless / xhttp | h2 → h2 | VLESS Encryption, Vision | Export candidate; app runtime unverified |
| 16 | vless / xhttp | h2 → h2 | — | Export candidate; app runtime unverified |
| 17 | vless / xhttp | h3 → h3 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 18 | vless / xhttp | h3 → h3 | — | XHTTP requires h2/h2c |
| 19 | vless / xhttp | plaintext → http/1.1 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 20 | vless / xhttp | plaintext → http/1.1 | — | XHTTP requires h2/h2c |
| 21 | vless / xhttp | plaintext → h2 | VLESS Encryption, Vision | Export candidate; app runtime unverified |
| 22 | vless / xhttp | plaintext → h2 | — | Export candidate; app runtime unverified |
| 23 | vless / xhttp | plaintext → h3 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 24 | vless / xhttp | plaintext → h3 | — | XHTTP requires h2/h2c |
| 25 | vless / xhttp | http/1.1 → plaintext | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 26 | vless / xhttp | http/1.1 → plaintext | — | XHTTP requires h2/h2c |
| 27 | vless / xhttp | http/1.1 → h2 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 28 | vless / xhttp | http/1.1 → h2 | — | XHTTP requires h2/h2c |
| 29 | vless / xhttp | http/1.1 → h3 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 30 | vless / xhttp | http/1.1 → h3 | — | XHTTP requires h2/h2c |
| 31 | vless / xhttp | h2 → plaintext | VLESS Encryption, Vision | Export candidate; app runtime unverified |
| 32 | vless / xhttp | h2 → plaintext | — | Export candidate; app runtime unverified |
| 33 | vless / xhttp | h2 → http/1.1 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 34 | vless / xhttp | h2 → http/1.1 | — | XHTTP requires h2/h2c |
| 35 | vless / xhttp | h2 → h3 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 36 | vless / xhttp | h2 → h3 | — | XHTTP requires h2/h2c |
| 37 | vless / xhttp | h3 → plaintext | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 38 | vless / xhttp | h3 → plaintext | — | XHTTP requires h2/h2c |
| 39 | vless / xhttp | h3 → http/1.1 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 40 | vless / xhttp | h3 → http/1.1 | — | XHTTP requires h2/h2c |
| 41 | vless / xhttp | h3 → h2 | VLESS Encryption, Vision | XHTTP requires h2/h2c |
| 42 | vless / xhttp | h3 → h2 | — | XHTTP requires h2/h2c |
| 43 | vmess / tcp | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 44 | vmess / tcp | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 45 | vmess / tcp | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 46 | vmess / tcp | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 47 | vmess / tcp | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 48 | vmess / tcp | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 49 | vmess / tcp | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 50 | vmess / tcp | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 51 | vless / grpc | h2/http/1.1 | VLESS Encryption, Vision | Vision carrier unsupported |
| 52 | vless / grpc | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 53 | vless / grpc | plaintext | VLESS Encryption, Vision | gRPC requires TLS; Vision carrier unsupported |
| 54 | vless / grpc | plaintext | — | gRPC requires TLS |
| 55 | vmess / grpc | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 56 | vmess / grpc | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 57 | vmess / grpc | h2/http/1.1 | — | gRPC requires TLS |
| 58 | vmess / grpc | h2/http/1.1 | — | gRPC requires TLS |
| 59 | vless / tcp | plaintext | VLESS Encryption, Vision | Export candidate; app runtime unverified |
| 60 | vmess / tcp | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 61 | vless / http | http/1.1 | VLESS Encryption, Vision | Vision carrier unsupported |
| 62 | vless / http | http/1.1 | — | Export candidate; app runtime unverified |
| 63 | vless / http | plaintext | VLESS Encryption, Vision | Vision carrier unsupported |
| 64 | vless / http | plaintext | — | Export candidate; app runtime unverified |
| 65 | vmess / http | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 66 | vmess / http | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 67 | vmess / http | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 68 | vmess / http | h2/http/1.1 | — | Export candidate; app runtime unverified |
| 69 | tuic | TLS | cubic | TUIC client fixed to BBR |
| 70 | hysteria2 | TLS | Salamander | Export candidate; app runtime unverified |
| 71 | hysteria | TLS | 150/300 Mbps | Export candidate; app runtime unverified |
| 72 | wireguard | WireGuard | MTU 1380, keepalive 0, PSK | Export candidate; app runtime unverified |

The 32 incompatible rows are blocked by the Stash export adapter; 40 rows retain their parameters for later client runtime testing. The sanitized local test performs an initial connection, five further HTTP requests, and UDP public → private rejection → another public destination → original public destination on one association (AnyTLS is TCP-only). IPv4-mapped loopback is included.

Run `XINGDU_TEST_MATRIX_RUNTIME=<private runtime directory> go test ./internal/protocol -run TestCompatibility72Runtime -count=1 -v -timeout 30m`. This uses isolated Docker networking and does not change the host proxy, DNS, routes or running Stash app.


## Remote acceptance runner

After deploying the control plane and upgrading the selected Agent, run
`tools/compatibility/remote72.py --origin https://your-control-plane.example --host <server-id> --key-file <private-key-file> --state <private-state-directory> --probe-runtime-dir <pinned-runtime-directory>`.
The key file must be mode 600 and the state directory mode 700. API requests do
not follow redirects. The runner resumes nodes by their exact matrix name,
protocol and port, retains client configs privately, and stops on the first
failed deployment or forwarding check. Credentials never appear in command
arguments. Its client projection is compared against all 72 locally tested
reference configs by `TestCompatibility72PublicReferenceProjection`.

The public probe runs five HTTPS exit-IP requests per node in an isolated Docker
client and requires the observed IPv4 exit to equal the API-reported server IP.
It requires the existing `xingdu-lab-amazon:latest` test image and fixed arm64
runtime files produced by `make runtimes`. Public UDP/firewall validation remains
a separate check; local UDP acceptance does not prove the cloud security group.
The script installs no software through SSH and does not change local Stash.
