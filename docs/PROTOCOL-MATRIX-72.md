# 72-case protocol matrix

Source parameters are preserved in `internal/protocol/testdata/compatibility72.json`; endpoint addresses, names, paths and credentials are lab replacements. Every case has its own listener. No Hiddify installation is required.

This matrix describes protocol deployment parameters independently of client products.
Acceptance is tracked separately for local reference-client forwarding and public
AWS forwarding. The historical custom-runtime suite passed all 72 local cases
(929 seconds), plus four HTTPUpgrade combinations; it does not establish official
upstream compatibility. See [historical deployment results](PROTOCOL-MATRIX-72-ACCEPTANCE.md)
and [official-runtime results](PROTOCOL-MATRIX-72-OFFICIAL.md) for their respective
versions, topology and limitations.

| Case | Protocol / transport | Upload / download | Additional parameter |
| --- | --- | --- | --- |
| 1 | vless / ws | http/1.1 | — |
| 2 | vmess / ws | http/1.1 | — |
| 3 | vless / grpc | h2 | — |
| 4 | vmess / grpc | h2 | — |
| 5 | vmess / ws | plaintext | — |
| 6 | socks | plaintext | — |
| 7 | wireguard | WireGuard | MTU 1380, keepalive 25, PSK |
| 8 | tuic | TLS | — |
| 9 | hysteria2 | TLS | Salamander |
| 10 | anytls | TLS | — |
| 11 | vless / xhttp | plaintext → plaintext | VLESS Encryption, Vision |
| 12 | vless / xhttp | plaintext → plaintext | — |
| 13 | vless / xhttp | http/1.1 → http/1.1 | VLESS Encryption, Vision |
| 14 | vless / xhttp | http/1.1 → http/1.1 | — |
| 15 | vless / xhttp | h2 → h2 | VLESS Encryption, Vision |
| 16 | vless / xhttp | h2 → h2 | — |
| 17 | vless / xhttp | h3 → h3 | VLESS Encryption, Vision |
| 18 | vless / xhttp | h3 → h3 | — |
| 19 | vless / xhttp | plaintext → http/1.1 | VLESS Encryption, Vision |
| 20 | vless / xhttp | plaintext → http/1.1 | — |
| 21 | vless / xhttp | plaintext → h2 | VLESS Encryption, Vision |
| 22 | vless / xhttp | plaintext → h2 | — |
| 23 | vless / xhttp | plaintext → h3 | VLESS Encryption, Vision |
| 24 | vless / xhttp | plaintext → h3 | — |
| 25 | vless / xhttp | http/1.1 → plaintext | VLESS Encryption, Vision |
| 26 | vless / xhttp | http/1.1 → plaintext | — |
| 27 | vless / xhttp | http/1.1 → h2 | VLESS Encryption, Vision |
| 28 | vless / xhttp | http/1.1 → h2 | — |
| 29 | vless / xhttp | http/1.1 → h3 | VLESS Encryption, Vision |
| 30 | vless / xhttp | http/1.1 → h3 | — |
| 31 | vless / xhttp | h2 → plaintext | VLESS Encryption, Vision |
| 32 | vless / xhttp | h2 → plaintext | — |
| 33 | vless / xhttp | h2 → http/1.1 | VLESS Encryption, Vision |
| 34 | vless / xhttp | h2 → http/1.1 | — |
| 35 | vless / xhttp | h2 → h3 | VLESS Encryption, Vision |
| 36 | vless / xhttp | h2 → h3 | — |
| 37 | vless / xhttp | h3 → plaintext | VLESS Encryption, Vision |
| 38 | vless / xhttp | h3 → plaintext | — |
| 39 | vless / xhttp | h3 → http/1.1 | VLESS Encryption, Vision |
| 40 | vless / xhttp | h3 → http/1.1 | — |
| 41 | vless / xhttp | h3 → h2 | VLESS Encryption, Vision |
| 42 | vless / xhttp | h3 → h2 | — |
| 43 | vmess / tcp | h2/http/1.1 | — |
| 44 | vmess / tcp | h2/http/1.1 | — |
| 45 | vmess / tcp | h2/http/1.1 | — |
| 46 | vmess / tcp | h2/http/1.1 | — |
| 47 | vmess / tcp | h2/http/1.1 | — |
| 48 | vmess / tcp | h2/http/1.1 | — |
| 49 | vmess / tcp | h2/http/1.1 | — |
| 50 | vmess / tcp | h2/http/1.1 | — |
| 51 | vless / grpc | h2/http/1.1 | VLESS Encryption, Vision |
| 52 | vless / grpc | h2/http/1.1 | — |
| 53 | vless / grpc | plaintext | VLESS Encryption, Vision |
| 54 | vless / grpc | plaintext | — |
| 55 | vmess / grpc | h2/http/1.1 | — |
| 56 | vmess / grpc | h2/http/1.1 | — |
| 57 | vmess / grpc | h2/http/1.1 | — |
| 58 | vmess / grpc | h2/http/1.1 | — |
| 59 | vless / tcp | plaintext | VLESS Encryption, Vision |
| 60 | vmess / tcp | h2/http/1.1 | — |
| 61 | vless / http | http/1.1 | VLESS Encryption, Vision |
| 62 | vless / http | http/1.1 | — |
| 63 | vless / http | plaintext | VLESS Encryption, Vision |
| 64 | vless / http | plaintext | — |
| 65 | vmess / http | h2/http/1.1 | — |
| 66 | vmess / http | h2/http/1.1 | — |
| 67 | vmess / http | h2/http/1.1 | — |
| 68 | vmess / http | h2/http/1.1 | — |
| 69 | tuic | TLS | cubic |
| 70 | hysteria2 | TLS | Salamander |
| 71 | hysteria | TLS | 150/300 Mbps |
| 72 | wireguard | WireGuard | MTU 1380, keepalive 0, PSK |

The historical sanitized local test performs an initial connection, five further
HTTP requests, and UDP public → private rejection → another public destination →
original public destination on one association (AnyTLS is TCP-only).
IPv4-mapped loopback is included. Client export capabilities are maintained
separately from this deployment matrix.

Run `XINGDU_TEST_MATRIX_RUNTIME=<private runtime directory> go test ./internal/protocol -run TestCompatibility72Runtime -count=1 -v -timeout 30m`. This uses isolated Docker networking and does not change the host proxy, DNS or routes.


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
The script installs no software through SSH and does not change the host proxy configuration.
