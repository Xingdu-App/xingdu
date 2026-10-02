# Official runtime reference-client results — 2026-10-02

## Scope

This report records the 72-case deployment parameter matrix against unmodified
upstream release executables in a local Linux arm64 Docker lab. It uses native
listeners and, for 20 mixed upload/download combinations, standard configuration
on official Nginx. It does not use the custom launcher, XHTTP frontend, UDP guard
patches, or reference-client ALPN patch from the historical runtime suite.

Published lab ports bind only to loopback. Each passing reference-client case
returned the expected HTTP marker three times. This establishes TCP payload
forwarding, including over UDP-based proxy transports; it does not establish
UDP destination relay, Internet throughput, third-party client compatibility,
or production AWS acceptance.

## Runtime provenance

| Artifact | Version | SHA-256 |
| --- | --- | --- |
| sing-box Linux arm64 executable | 1.14.2 | `b8610f45abb7e967e195264383f5cbd20fba7821a3c37e3a8c4c5ab6cad28eac` |
| Xray Linux arm64 executable | 26.9.9 | `c1defe42b6db958a97c5e049a02a00a4baaedaca7b51c1c229f0830e288acef5` |
| Xray Linux arm64 release archive | 26.9.9 | `3e38d72dfc5eb65c91df0e5583e9b6676c32232041da47de6ae73946b526d66c` |
| Official Nginx image | 1.30.5 | `0985e772fb9f729e6fa0980da05fca5d9c468e870eed43071545afa9d2e27d94` |

Upstream executable hashes were rechecked unchanged after testing. Private
fixtures, credentials and raw logs remain outside version control.

## Results and remaining gap

Official reference clients passed **68 of 72 cases**. Cases **27, 28, 33 and 34**
remain unsuccessful with their original mixed HTTP/1.1 / HTTP/2 and browser
fingerprint settings. The observed mismatch concerns the configured HTTP
version and the ALPN offered by the selected fingerprint; it is not evidence
that these combinations are inherently unsupported upstream.

The [upstream XHTTP documentation](https://github.com/XTLS/Xray-core/discussions/4113)
distinguishes HTTP version selection from changing the uTLS browser fingerprint.
These four cases must remain unresolved until a reference-client run validates
the original parameters. No patched binary or substituted parameter is counted
as an official-runtime pass.

The initial native-listener baseline covered 52 combinations, with 48 reference
passes and the same four failures. The remaining 20 combinations were initially
excluded pending a valid frontend topology; standard Nginx TLS, HTTP/2, HTTP/3
and gRPC reverse-proxy configuration enabled their later reference tests.

## Deployment boundary

The AWS nodes from the [historical custom-runtime run](PROTOCOL-MATRIX-72-ACCEPTANCE.md)
were not replaced by this local test. Their results must not be relabeled as
unmodified upstream acceptance. Production runtime deployment and subscription
export restrictions remain unchanged. No release or production upgrade is
established by this report.
