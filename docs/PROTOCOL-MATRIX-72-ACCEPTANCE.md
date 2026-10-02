# Historical custom-runtime matrix results — 2026-10-02

> These results used modified sing-box/Xray binaries and an Xingdu XHTTP
> frontend. They do **not** establish compatibility with official upstream
> releases. Official-runtime acceptance must be recorded separately, using
> unmodified release artifacts and native listeners. The historical results
> below are retained only as diagnostic evidence.

The replacement investigation is documented in [official runtime interoperability](PROTOCOL-MATRIX-72-OFFICIAL.md).

## Scope and versions

All 72 nodes were deployed through the Xingdu API to the designated AWS HK host.
The Agent reports `0.16.0-dev`; all 72 deployment states are `succeeded` and all
72 managed services are `active`. Real HTTPS requests verified the AWS exit
address five times per reference-client case.

Reference runtimes: `sing-box 1.14.2-xingdu.1` and `Xray 26.3.27-xingdu.1`.

## Results and limits

- Reference clients: **51 cases passed on the original network path**. The other
  **21 passed with UDP carried through the working VLESS/WS/TCP node** to AWS.
  Each path listed as passing completed five HTTPS requests with matching exit IP.
- These results do not establish direct UDP reachability, third-party client
  compatibility, throughput, roaming, or long-duration soak stability. The local
  original-parameter TCP/UDP isolation suite is documented separately in
  [the parameter matrix](PROTOCOL-MATRIX-72.md).

## Repairs found by real tests

1. WireGuard export adapters used one PSK field spelling across different
   client formats. Fix `d9ce1d2` emits the format-specific field so the key is
   preserved. Export correctness is separate from runtime deployment acceptance.
2. The public reference WireGuard probe inherited the Mac's fake-IP DNS answer.
   The server correctly rejected that reserved destination. Fix `8a14837` makes its reference
   client resolve via DoH inside WireGuard. The original UDP path still
   times out after that correction; the TCP-carried path passes.

The carrier comparison preserves each node's protocol, encryption, TLS, ALPN,
PSK and transport parameters. It changes how packets reach the endpoint and
must not be described as direct network acceptance. Secrets, live addresses,
subscription tokens and raw configurations remain in ignored private lab state.

## Per-case evidence

“Carrier” means the additional VLESS/WS/TCP path described above. “Unreachable” is
the original path's failure.

| Case | Protocol | AWS reference original | AWS reference carrier |
| --- | --- | --- | --- |
| 01 | vless | PASS | — |
| 02 | vmess | PASS | — |
| 03 | vless | PASS | — |
| 04 | vmess | PASS | — |
| 05 | vmess | PASS | — |
| 06 | socks | PASS | — |
| 07 | wireguard | unreachable | PASS |
| 08 | tuic | unreachable | PASS |
| 09 | hysteria2 | unreachable | PASS |
| 10 | anytls | PASS | — |
| 11 | vless | PASS | — |
| 12 | vless | PASS | — |
| 13 | vless | PASS | — |
| 14 | vless | PASS | — |
| 15 | vless | PASS | — |
| 16 | vless | PASS | — |
| 17 | vless | unreachable | PASS |
| 18 | vless | unreachable | PASS |
| 19 | vless | PASS | — |
| 20 | vless | PASS | — |
| 21 | vless | PASS | — |
| 22 | vless | PASS | — |
| 23 | vless | unreachable | PASS |
| 24 | vless | unreachable | PASS |
| 25 | vless | PASS | — |
| 26 | vless | PASS | — |
| 27 | vless | PASS | — |
| 28 | vless | PASS | — |
| 29 | vless | unreachable | PASS |
| 30 | vless | unreachable | PASS |
| 31 | vless | PASS | — |
| 32 | vless | PASS | — |
| 33 | vless | PASS | — |
| 34 | vless | PASS | — |
| 35 | vless | unreachable | PASS |
| 36 | vless | unreachable | PASS |
| 37 | vless | unreachable | PASS |
| 38 | vless | unreachable | PASS |
| 39 | vless | unreachable | PASS |
| 40 | vless | unreachable | PASS |
| 41 | vless | unreachable | PASS |
| 42 | vless | unreachable | PASS |
| 43 | vmess | PASS | — |
| 44 | vmess | PASS | — |
| 45 | vmess | PASS | — |
| 46 | vmess | PASS | — |
| 47 | vmess | PASS | — |
| 48 | vmess | PASS | — |
| 49 | vmess | PASS | — |
| 50 | vmess | PASS | — |
| 51 | vless | PASS | — |
| 52 | vless | PASS | — |
| 53 | vless | PASS | — |
| 54 | vless | PASS | — |
| 55 | vmess | PASS | — |
| 56 | vmess | PASS | — |
| 57 | vmess | PASS | — |
| 58 | vmess | PASS | — |
| 59 | vless | PASS | — |
| 60 | vmess | PASS | — |
| 61 | vless | PASS | — |
| 62 | vless | PASS | — |
| 63 | vless | PASS | — |
| 64 | vless | PASS | — |
| 65 | vmess | PASS | — |
| 66 | vmess | PASS | — |
| 67 | vmess | PASS | — |
| 68 | vmess | PASS | — |
| 69 | tuic | unreachable | PASS |
| 70 | hysteria2 | unreachable | PASS |
| 71 | hysteria | unreachable | PASS |
| 72 | wireguard | unreachable | PASS |

## Validation boundaries

`make check` passed after the repairs. Full and API-only Docker Compose rebuilds
were attempted, but Docker Hub base-image metadata requests hit TLS handshake
timeouts; existing local API/web health remained good. Cloud deployment status
and a fresh production export must be verified separately from these tests.
