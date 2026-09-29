# Offline country database

`country.mmdb` is the unmodified DB-IP IP to Country Lite September 2026
snapshot. It is embedded into the server build; no download or external IP
lookup occurs at startup or while serving requests. It is not MIT-licensed.

- Publisher: DB-IP.com, https://db-ip.com
- Download: https://download.db-ip.com/free/dbip-country-lite-2026-09.mmdb.gz
- Release page: https://db-ip.com/db/download/ip-to-country-lite
- License: Creative Commons Attribution 4.0 International,
  https://creativecommons.org/licenses/by/4.0/
- Uncompressed SHA-256: `d284ae2e7427fe33d83465e1506b2b21aae47eb8a9b099f8f4dac6a98c99f041`
- Publisher's uncompressed SHA-1: `385d4ab1e08417634a0a64921ac0e9c15c4c5e8a`

Retain this attribution when redistributing the embedded database. Pages
showing its results link to DB-IP. The data is approximate and must not be
used as proof of a machine's physical location or as an authorization rule.

The API enriches host responses after tenant-scoped reads, without adding
stored columns or changing RLS. Literal IPv4 and IPv6 addresses are supported;
private/special addresses and hostnames return no country. No DNS resolution
is performed. The frontend localizes country names and renders Unicode flags
with a country-name fallback for platforms without flag glyphs.

To update, download a dated release from the publisher, decompress it, verify
its published checksum, and replace `country.mmdb`. Update this provenance,
the pinned SHA-256 in `geoip_test.go` and the third-party notice; then run
`make check` and container validation. Updates are reviewed source changes,
not runtime network downloads. Country changes appear after the next server
build without migrating or rewriting host records.
