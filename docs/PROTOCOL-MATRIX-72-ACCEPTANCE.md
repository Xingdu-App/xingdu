# AWS and Android matrix acceptance — 2026-10-02

## Scope and versions

All 72 nodes were deployed through the Xingdu API to the designated AWS HK host.
The Agent reports `0.16.0-dev`; all 72 deployment states are `succeeded` and all
72 managed services are `active`. Real HTTPS requests verified the AWS exit
address five times per reference-client case.

Reference runtimes: `sing-box 1.14.2-xingdu.1` and `Xray 26.3.27-xingdu.1`.
Android: `1.1.7-debug`, repository/remote `dev` commit `ff92cbd`, Rust Core pin
`19721c5c`; the native artifact stamp matches that pin. Tests ran on the arm64
API 37 emulator. A Pixel 8 received the same APK (SHA-256 matched), but direct
TCP connections to the AWS test and baseline ports timed out before protocol
handshake. Its results are a reachability limitation, not protocol failures.

## Results and limits

- Reference clients: **51 cases passed on the original network path**. The other
  **21 passed with UDP carried through the working VLESS/WS/TCP node** to AWS.
  Each path listed as passing completed five HTTPS requests with matching exit IP.
- Android native core: **34 cases passed on the original path**, and **6 passed
  through the same TCP carrier** (three matching-exit HTTPS requests each).
- **32 cases remain incompatible with Stash parameters** and were rejected by
  its export adapter; they are not counted as successful Android runtime tests.
- Full Android App VPN acceptance passed for case 01: service startup, matching
  AWS exit address, and an observed connection sourced from the TUN interface.
  The test restored the previous config and stopped its VPN afterward. This is
  one representative VPN test, not 40 separate full-App/TUN acceptances.
- These results do not establish direct UDP reachability, physical Android
  acceptance, iOS/macOS acceptance, throughput, roaming, or long-duration soak
  stability. The local original-parameter 72-case TCP/UDP isolation suite is
  documented separately in [the parameter matrix](PROTOCOL-MATRIX-72.md).

## Repairs found by real tests

1. Stash WireGuard reads `preshared-key`, while Mihomo uses `pre-shared-key`.
   Xingdu emitted the Mihomo spelling for both, so Stash omitted the PSK and
   reported `InvalidAeadTag`. Fix `d9ce1d2` makes the adapters emit their respective fields;
   both WireGuard cases passed on Android after correcting this field.
2. The public reference WireGuard probe inherited the Mac's fake-IP DNS answer.
   The server correctly rejected that reserved destination. Fix `8a14837` makes its reference
   client resolve via DoH inside WireGuard. The original UDP path still
   times out after that correction; the TCP-carried path passes.

The carrier comparison preserves each node's protocol, encryption, TLS, ALPN,
PSK and transport parameters. It changes how packets reach the endpoint and
must not be described as direct network acceptance. Secrets, live addresses,
subscription tokens and raw configurations remain in ignored private lab state.

## Per-case evidence

“Carrier” means the additional VLESS/WS/TCP path described above. “Blocked” is
an export compatibility rejection; “unreachable” is the original path's failure.
WireGuard Android results below include the PSK export correction.

| Case | Protocol | AWS reference original | AWS reference carrier | Android original | Android carrier |
| --- | --- | --- | --- | --- | --- |
| 01 | vless | PASS | — | PASS | — |
| 02 | vmess | PASS | — | PASS | — |
| 03 | vless | PASS | — | PASS | — |
| 04 | vmess | PASS | — | PASS | — |
| 05 | vmess | PASS | — | PASS | — |
| 06 | socks | PASS | — | PASS | — |
| 07 | wireguard | unreachable | PASS | unreachable | PASS |
| 08 | tuic | unreachable | PASS | unreachable | PASS |
| 09 | hysteria2 | unreachable | PASS | unreachable | PASS |
| 10 | anytls | PASS | — | PASS | — |
| 11 | vless | PASS | — | PASS | — |
| 12 | vless | PASS | — | PASS | — |
| 13 | vless | PASS | — | blocked | — |
| 14 | vless | PASS | — | blocked | — |
| 15 | vless | PASS | — | PASS | — |
| 16 | vless | PASS | — | PASS | — |
| 17 | vless | unreachable | PASS | blocked | — |
| 18 | vless | unreachable | PASS | blocked | — |
| 19 | vless | PASS | — | blocked | — |
| 20 | vless | PASS | — | blocked | — |
| 21 | vless | PASS | — | PASS | — |
| 22 | vless | PASS | — | PASS | — |
| 23 | vless | unreachable | PASS | blocked | — |
| 24 | vless | unreachable | PASS | blocked | — |
| 25 | vless | PASS | — | blocked | — |
| 26 | vless | PASS | — | blocked | — |
| 27 | vless | PASS | — | blocked | — |
| 28 | vless | PASS | — | blocked | — |
| 29 | vless | unreachable | PASS | blocked | — |
| 30 | vless | unreachable | PASS | blocked | — |
| 31 | vless | PASS | — | PASS | — |
| 32 | vless | PASS | — | PASS | — |
| 33 | vless | PASS | — | blocked | — |
| 34 | vless | PASS | — | blocked | — |
| 35 | vless | unreachable | PASS | blocked | — |
| 36 | vless | unreachable | PASS | blocked | — |
| 37 | vless | unreachable | PASS | blocked | — |
| 38 | vless | unreachable | PASS | blocked | — |
| 39 | vless | unreachable | PASS | blocked | — |
| 40 | vless | unreachable | PASS | blocked | — |
| 41 | vless | unreachable | PASS | blocked | — |
| 42 | vless | unreachable | PASS | blocked | — |
| 43 | vmess | PASS | — | PASS | — |
| 44 | vmess | PASS | — | PASS | — |
| 45 | vmess | PASS | — | PASS | — |
| 46 | vmess | PASS | — | PASS | — |
| 47 | vmess | PASS | — | PASS | — |
| 48 | vmess | PASS | — | PASS | — |
| 49 | vmess | PASS | — | PASS | — |
| 50 | vmess | PASS | — | PASS | — |
| 51 | vless | PASS | — | blocked | — |
| 52 | vless | PASS | — | PASS | — |
| 53 | vless | PASS | — | blocked | — |
| 54 | vless | PASS | — | blocked | — |
| 55 | vmess | PASS | — | PASS | — |
| 56 | vmess | PASS | — | PASS | — |
| 57 | vmess | PASS | — | blocked | — |
| 58 | vmess | PASS | — | blocked | — |
| 59 | vless | PASS | — | PASS | — |
| 60 | vmess | PASS | — | PASS | — |
| 61 | vless | PASS | — | blocked | — |
| 62 | vless | PASS | — | PASS | — |
| 63 | vless | PASS | — | blocked | — |
| 64 | vless | PASS | — | PASS | — |
| 65 | vmess | PASS | — | PASS | — |
| 66 | vmess | PASS | — | PASS | — |
| 67 | vmess | PASS | — | PASS | — |
| 68 | vmess | PASS | — | PASS | — |
| 69 | tuic | unreachable | PASS | blocked | — |
| 70 | hysteria2 | unreachable | PASS | unreachable | PASS |
| 71 | hysteria | unreachable | PASS | unreachable | PASS |
| 72 | wireguard | unreachable | PASS | unreachable | PASS |

## Validation boundaries

`make check` passed after the repairs. Full and API-only Docker Compose rebuilds
were attempted, but Docker Hub base-image metadata requests hit TLS handshake
timeouts; existing local API/web health remained good. Cloud deployment status
and a fresh production export must be verified separately from these tests.
