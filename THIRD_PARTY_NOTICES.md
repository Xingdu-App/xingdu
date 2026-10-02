# Third-party runtime notices

Xingdu's original source is licensed under [MIT](LICENSE). Protocol engines
run as separate processes and retain their own licenses; they are not affiliated
with Xingdu. No upstream component is relicensed under Xingdu's MIT license.

- New Agents use the unmodified official **sing-box 1.14.2** release
  ([GPL-3.0-or-later source](https://github.com/SagerNet/sing-box/tree/v1.14.2))
  and **Xray 26.9.9** release
  ([MPL-2.0 source](https://github.com/XTLS/Xray-core/tree/v26.9.9)).
  Archive and executable hashes are pinned; original licenses are retained.
- Historical **sing-box 1.14.2-xingdu.1** is built from upstream commit
  `af6e64c3b69e6132ebaee0e1a3d24e93903f6709` with the per-datagram public-egress
  guard in `tools/runtime-patches/sing-box-udp-guard.patch`.
  [Source and GPL-3.0-or-later license](https://github.com/SagerNet/sing-box/tree/af6e64c3b69e6132ebaee0e1a3d24e93903f6709).
- Historical **Xray 26.3.27-xingdu.1** is built from upstream commit
  `d2758a023cd7f4174a5a5fa4ff66e487d4342ba0`, with the per-datagram egress guard
  and the explicit HTTP/1.1 ALPN correction listed in `tools/build-runtime/build.sh`.
  [Source and MPL-2.0 license](https://github.com/XTLS/Xray-core/tree/d2758a023cd7f4174a5a5fa4ff66e487d4342ba0).
- TrustTunnel remains a separately executed, unmodified pinned upstream endpoint.

`make runtimes` fetches verified official assets and retains legacy patched assets
for old Agents. Legacy builds verify fixed source/archive hashes, use Go 1.26.1,
verifies the resulting executables against `internal/protocol/runtime_manifest.go`,
and preserves upstream notices. The build recipe and patches are distributed
with this repository. Docker artifacts include the modified source trees and
module checksums in `/opt/xingdu/agents/{sing-box,xray}-source.tar.gz`.
They are publicly downloadable from the deployment's
`/api/v1/agent/runtime-source/sing-box` and `/api/v1/agent/runtime-source/xray`
endpoints alongside the binaries. Upstream dependency versions and licenses remain
in those source archives. Do not strip the sources or notices when redistributing.

The selected protocols do not use or bundle Naive/Chromium. Client applications
such as Stash, Surge, Loon and Shadowrocket are separate products; their names
do not imply endorsement or verified compatibility with every protocol.

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

## Subscription policy-group icons

The original category-only v1 set derives from
[Lucide](https://github.com/lucide-icons/lucide) at revision
`5a92b9ba262de5bf10e864219883267672c05db8`, under ISC with inherited Feather
MIT notices. Its sources and full license remain available under
`apps/web/public/subscription-icons/v1/`.

The v2 set adds service marks from [Lobe Icons](https://github.com/lobehub/lobe-icons)
at revision `329f378cbd1a88f45b60cd096b9111ce16f3ea39` (MIT) and
[Simple Icons](https://github.com/simple-icons/simple-icons) at revision
`1089fb7d2bf0e323f834c205ab76265005a6d5e8` (CC0-1.0). Category symbols retain
the above Lucide sources. Full licenses, source SVGs and the composition manifest
are distributed under `apps/web/public/subscription-icons/v2/`.

Service marks identify third-party routing targets; their respective owners
retain trademark rights, and no endorsement or partnership is implied.
Xingdu supplies the backgrounds and PNG/SVG composition. Qure and Semporia
personal-use resources are not bundled.
