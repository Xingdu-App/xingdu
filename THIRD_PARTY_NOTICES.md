# Third-party runtime notices

Xingdu's original source code is licensed under [MIT](LICENSE). Protocol deployments run the unmodified **sing-box 1.14.2** executable as a separate systemd process. sing-box is a third-party component, is not relicensed under Xingdu's MIT license, and is not affiliated with Xingdu.

- [Upstream release and binary assets](https://github.com/SagerNet/sing-box/releases/tag/v1.14.2)
- [Versioned source code and build configuration](https://github.com/SagerNet/sing-box/tree/v1.14.2)
- [Upstream license](https://github.com/SagerNet/sing-box/blob/v1.14.2/LICENSE): GNU GPL version 3 or later, with the upstream naming/association notice.
- [GNU GPL version 3 full text](https://www.gnu.org/licenses/gpl-3.0.html)

`make runtimes` downloads the official archives, verifies pinned archive and executable SHA-256 hashes, and preserves the upstream `LICENSE` as `bin/agents/sing-box-LICENSE`. Docker builds include the same license at `/opt/xingdu/agents/sing-box-LICENSE`; managed machines receive the embedded notice at `/usr/local/lib/xingdu/sing-box-LICENSE`. The pinned versions and hashes are maintained in `internal/protocol/runtime_manifest.go`; no tenant can select a binary URL or a runtime version.

The selected protocols do not use the optional Naive/Chromium library included in the upstream archives; Xingdu does not copy or distribute that library. Client applications such as Stash, Surge, Loon and Shadowrocket are separate products; their names do not imply endorsement or verified compatibility with every protocol.

When redistributing runtime binaries, retain the upstream notices and meet their source-distribution obligations. This repository does not claim that an upstream download link alone fulfills every redistribution scenario. Release packaging and corresponding-source delivery must be reviewed before publishing distributable binary releases.

## Browser QR code rendering

Subscription QR codes are generated locally with qrcode.react 4.2.0.

ISC License

Copyright (c) 2015, Paul O’Shannessy

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND
FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
PERFORMANCE OF THIS SOFTWARE.

This product bundles QR Code Generator, which is available under a
"MIT" license. For details, see src/third-party/qrcodegen.

## Offline IP country data

IP Geolocation by [DB-IP](https://db-ip.com). The unmodified September 2026
IP to Country Lite database is bundled in `internal/geoip/country.mmdb`
and embedded in the server. This data is licensed under
[Creative Commons Attribution 4.0 International](https://creativecommons.org/licenses/by/4.0/),
not Xingdu's MIT license. Retain attribution and the license link when
redistributing it. See `internal/geoip/README.md` for provenance and checksums.
The server performs lookups locally; it does not transmit host IPs to DB-IP.

The MaxMind DB reader is `github.com/oschwald/maxminddb-golang/v2`, licensed
under ISC. Its license is included in `internal/geoip/READER-LICENSE`.
