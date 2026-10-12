# Offline country database

`country.mmdb` is the unmodified DB-IP IP to Country Lite October 2026
snapshot. It is embedded into the server build; no download or external IP
lookup occurs at startup or while serving requests. It is not MIT-licensed.

- Publisher: DB-IP.com, https://db-ip.com
- Download: https://download.db-ip.com/free/dbip-country-lite-2026-10.mmdb.gz
- Release page: https://db-ip.com/db/download/ip-to-country-lite
- License: Creative Commons Attribution 4.0 International,
  https://creativecommons.org/licenses/by/4.0/
- Uncompressed SHA-256: `dbd70ccfa2a13627eaf4913d19920a1a229f1b3c5439ac01c509995b6364202a`
- Publisher's uncompressed SHA-1: `e515b63075ad48847f9c4332b724742e8ba15b0e`

Retain this attribution when redistributing the embedded database. Pages
showing its results link to DB-IP. The data is approximate and must not be
used as proof of a machine's physical location or as an authorization rule.

The publisher releases the Lite database monthly, with reduced coverage and
accuracy compared with its full database. A newer snapshot can correct some
addresses while leaving others misclassified. Do not infer physical location
from an address owner's country or hard-code customer addresses as global
corrections. The bundled DB-IP database has no automatic refresh or per-host
country override; the update procedure below requires a new server build.

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

## GeoLite Country provider

For deployments needing an alternative to DB-IP, set
`XINGDU_GEOLITE_DATABASE` on both API and worker to an operator-managed
`GeoLite2-Country.mmdb` file. The server validates its format, database type,
integrity and build time before accepting it. Missing, corrupt, future-dated
or more than 30-day-old configured files fail startup rather than silently
switching providers. A database that expires while running returns unknown
countries until updated and restarted. Without a file path or updater credentials, the server uses bundled DB-IP.

Only `country.iso_code` is used. `registered_country` represents registration,
not geolocation, and is intentionally not a fallback; records with no location
country return unknown. IPv4, IPv6 and private-address protections are unchanged.
Host responses include `country_source` (`dbip` or `maxmind`), and the frontend
credits the selected provider. Lookups remain local without DNS or API requests.

Obtain the database through your own MaxMind account and use the official
[geoipupdate tool](https://github.com/maxmind/geoipupdate) with edition
`GeoLite2-Country`. Keep the account ID and license key in a protected updater
configuration or deployment secret, outside Git and command arguments. For externally maintained files, arrange
weekly updates, replace files atomically, and restart API and worker after a
successful update. Alternatively, use the built-in official downloader below.
Mount the database read-only for runtime services and grant write access only
to the updater. Do not commit downloaded GeoLite files or retain old snapshots
in Git history, images or backups; follow MaxMind's current update and deletion
requirements.

GeoLite is governed by the [MaxMind GeoLite EULA](https://www.maxmind.com/en/geolite/eula)
and its incorporated CC BY-SA 4.0 terms, separately from Xingdu's MIT license.
This product includes GeoLite Data created by MaxMind, available from
https://www.maxmind.com. No GeoLite database is bundled or redistributed by
this source tree. Production credentials and provider activation remain deployment steps,
not established by local adapter tests.

An optional local acceptance check uses a licensed file without a tracked fixture:

```sh
XINGDU_TEST_GEOLITE_DATABASE=/var/lib/xingdu/GeoLite2-Country.mmdb \
  go test -race ./internal/geoip
```

## Managed official downloads

Set `XINGDU_MAXMIND_ACCOUNT_ID` and `XINGDU_MAXMIND_LICENSE_KEY` as runtime
secrets on both API and worker to enable managed downloads. These settings
are never browser configuration or build arguments. The service downloads
only GeoLite2-Country from MaxMind at startup, then every 24 hours, validates
it and replaces the in-memory reader under a lock. Lookups continue offline
and do not transmit host IPs. Updated results do not require a restart.

`XINGDU_GEOLITE_DATABASE` selects the writable cache file. If omitted, the
cache lives under the process temporary directory in `xingdu-geoip`. No
persistent volume is required: a new container downloads the current release.
A validated fresh cache permits startup during a download outage; without
one, failed initial download prevents startup. Subsequent failures preserve
the last valid reader until its 30-day expiry, after which countries are
unknown. Logs report refresh success/build date or a sanitized failure.

The downloader uses TLS and Basic authentication only at MaxMind's download
endpoint. Redirects are limited to MaxMind and its documented object-storage
host, and authorization is removed before following them. Credentials are
removed from the process environment after initialization so spawned machine
installation subprocesses do not inherit them. Downloaded archive and MMDB
sizes are bounded; replacement uses a private temporary file and rename.
Old readers and cache files are replaced rather than retained as snapshots.
