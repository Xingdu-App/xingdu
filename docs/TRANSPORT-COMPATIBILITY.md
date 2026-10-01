# Managed protocol parameter compatibility

The 0.16.0-dev contract adds bounded VLESS/VMess transports, XHTTP split upload
and download settings, VLESS Encryption, SOCKS5 UDP, WireGuard and explicit QUIC
parameters. Server runtime capability is independent of client export support.
See [the 72-case matrix](PROTOCOL-MATRIX-72.md) for the acceptance boundary.

## API

Creation, preflight and configuration revision APIs accept these optional objects:

- `v2ray`: `engine` (`sing-box` default, or `xray`), `network`, `tls`, `path`,
  `host`, `service_name`, `alpn`, `flow`, `packet_encoding`, `fingerprint`,
  `encryption`, `mode`, `headers`, `download`.
- `quic`: `alpn`, TUIC `congestion` (`bbr`, `cubic`, `new_reno`), Hysteria2
  `salamander`, and Hysteria1 `up_mbps` / `down_mbps`.
- `wireguard`: `mtu`, `keepalive`, three integer `reserved` bytes, `preshared`.
- `udp_enabled`: opt-in UDP for SOCKS5.

The sing-box transport adapter accepts TCP, WebSocket, gRPC and HTTP; its TLS
HTTP transport is HTTP/2. The independent Xray adapter also accepts `httpupgrade`
and `xhttp`, and preserves HTTP/1.1 camouflage over TLS as a distinct transport.
Vision with VLESS Encryption is supported by the Xray adapter on the tested
framed transports; this does not imply that every client implements it.

XHTTP uses one public TCP/UDP port with a private Unix-socket backend. An optional
`download: {"tls": true, "alpn": ["h3"]}` controls the downlink without changing
the upload's TLS/ALPN. Upload and download share the same host, port, path and
session. `mode` accepts `auto`, `packet-up`, `stream-up` or `stream-one`; split
`stream-one` is rejected. Headers are bounded to `User-Agent`. Tenant input cannot
select a backend address, Unix socket, binary URL, or arbitrary engine JSON.

Example creation body:

```json
{
  "name": "VLESS split lab",
  "protocol": "vless",
  "port": 18444,
  "server_name": "node.example.com",
  "certificate": "<matching PEM certificate>",
  "private_key": "<matching PEM private key>",
  "v2ray": {
    "engine": "xray",
    "network": "xhttp",
    "tls": true,
    "alpn": ["h2"],
    "path": "/lab",
    "mode": "auto",
    "encryption": true,
    "flow": "xtls-rprx-vision",
    "download": {"tls": true, "alpn": ["h3"]}
  },
  "confirm_install": true
}
```

A certificate is required when either direction uses TLS. Primary TLS=false
therefore does not imply that certificate fields should be empty when the
split download uses TLS. Inert ALPN on non-TLS Xray TCP inputs is retained.
TLS material must match the configured server name and be valid PEM.

Keys are generated server-side and stored in the encrypted deployment spec and
revision snapshots. Connection reveal returns only client material: public
VLESS Encryption settings, the WireGuard client private key and server public
key, optional PSK, and client obfuscation password. It never returns server TLS,
VLESS decryption, or WireGuard private keys. Ordinary lists omit credentials.

On revision updates an omitted object preserves the current one; a provided
object replaces it. Credential rotation preserves parameters. WireGuard PSK
can be toggled without changing the peer identity. Historical restoration
replaces the entire decoded spec, avoiding stale nested fields. Runtime engine
changes require a new node. Advanced parameter nodes cannot be relay exits.

These added capabilities require Agent 0.16.0-dev or newer. The control plane
and Agent enforce the requirement before queueing/claiming work. Migration 046
adds WireGuard to the database protocol constraint.

## Runtime isolation and reproducibility

Both server engines have a per-packet public-destination guard. Routing once at
UDP association creation was insufficient: a later datagram could switch to a
private or metadata address. The patched engines validate each resolved address,
including IPv4-mapped IPv6, and drop only the denied datagram. Existing route
rules still protect initial TCP/UDP destinations.

Older Agents retain their original runtime download endpoint and digest. New
Agents fetch hardened sing-box from `/api/v1/agent/runtime/sing-box/{arch}`.
Existing service processes are not silently replaced by a control-plane update.

The fixed-source build recipe, patches, checksums and downloadable corresponding
sources are described in [third-party notices](../THIRD_PARTY_NOTICES.md).
The reference Xray client also corrects explicit HTTP/1.1 ALPN when a browser
fingerprint would otherwise advertise HTTP/2. Tests retain the requested ALPN.

## Client boundary

Stash's current Rust Core uses HTTP/2 or h2c for XHTTP; HTTP/1.1 and HTTP/3 are
outside that implementation. Plaintext gRPC and Vision on gRPC/HTTP are also
rejected. TUIC uses BBR, so a requested cubic controller cannot be preserved.
Exports reject those combinations explicitly. The remaining matrix rows are
export candidates, not proof that an installed Stash release passed runtime
acceptance. No Stash core changes are part of this work.

Mihomo XHTTP export remains unverified and fails explicitly. Surge/Loon custom
transport exports remain bounded to their existing supported forms.
