# Private offline IP geolocation

IP location lookups use an authenticated `POST /api/v1/user/ip-geo` and a local
MMDB database. The application does not send queried customer IP addresses to
GeoJS, iplocation, or any other remote lookup service. There is no external
fallback. A missing or unreadable database makes only the optional location
lookup unavailable; usage records and other application functions continue.

The request body is `{ "ips": ["8.8.8.8", "2001:4860:4860::8888"] }`, with
1–50 literal IP addresses and a 4 KiB body limit. The route uses the existing
JWT authentication and heavy panel rate limiter. Results have
`Cache-Control: private, no-store`. Read-only lookup requests are excluded from
operation auditing so the queried customer-IP lists are not copied into audit
records. The browser removes its old persistent IP cache and keeps results only
in memory for the current signed-in user, with a 24-hour expiry.

## Install the database before deploying this change

1. Obtain a compatible city MMDB from its official publisher. DB-IP City Lite
   is available at <https://db-ip.com/db/download/ip-to-city-lite>. GeoLite2 City
   and licensed GeoIP2/DB-IP city databases are also compatible. Do not install
   the synthetic test fixture as a production database.
2. Check the publisher's license and published checksum. Downloading the whole
   database does not send customer IP addresses to the publisher.
3. Decompress the file outside Git, then install it as `geoip.mmdb` in the
   application's existing persistent data directory. Keep `config.yaml`,
   credentials, and all other existing data files unchanged.
4. Ensure the application can read the file and restart the application after
   an atomic file replacement. The read-only reader is shared across requests
   and loaded once per application process. Do not truncate or overwrite an
   MMDB file while a running application has it memory mapped.
5. From an authenticated local session, query a known public resolver address
   using the panel's “Fetch region” action. Batch lookup and local detail
   expansion should also work. Do not add production credentials to this file.

The path is selected in this order:

- `GEOIP_DATABASE_PATH`, if explicitly set;
- `${DATA_DIR}/geoip.mmdb`, if `DATA_DIR` is set;
- `./data/geoip.mmdb`, relative to the application working directory.

The normal Docker image runs under `/app`, so an existing `/app/data` volume
can hold `/app/data/geoip.mmdb` without a new mount. A non-default path needs an
appropriate read-only mount and the environment variable on **each** application
instance. Keep a rollback copy outside the live file; atomically rename the new
file into place and restart instances one at a time.

For Passion's documented persistent volume this means placing the file under
`/opt/sub2api-custom-prod/deploy/data/geoip.mmdb` before switching application
instances, after verifying their actual mounts. This document is not a record
that the production file has already been installed.

## Data license and location limitations

DB-IP Lite is licensed under CC BY 4.0. The UI includes a no-referrer attribution
link to DB-IP on results identified as DB-IP data; MaxMind results have the
corresponding provider link. It contains no queried IP or
lookup parameter. Source and license: <https://db-ip.com/db/lite.php>.

IP locations are approximate and can be stale or unknown; private/link-local
addresses are marked private without opening the database. City-only databases
may omit ISP, timezone, or accuracy data. The UI displays only fields actually
available in the installed database.

## Verification

The committed 18 KiB MaxMind public test fixture is synthetic and MIT licensed;
its fixed commit, SHA-256, and license are recorded under
`backend/internal/pkg/geoip/testdata/`. It is read by the default unit tests
without network requests or secrets:

```sh
cd backend
go test -tags=unit ./internal/pkg/geoip
go test -tags=unit ./internal/handler -run '^TestIPGeo'
```

The fixture tests cover real MMDB decoding, IPv4-mapped IPv6, unmatched addresses,
input limits, private addresses, and generic errors for missing/corrupt files.
Handler tests cover authentication, response cache headers, body limits, and
internal-path redaction. Frontend tests cover authenticated POST bodies, local
batching, session isolation, old-cache deletion, and removal of external IP
lookup navigation.
