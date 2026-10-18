# Content sources

| Content | Source | Licence | Status |
|---|---|---|---|
| Bible sw — `NENO` | eBible.org `swhonen` (Biblica Open Kiswahili Contemporary Version, 2015) | CC BY-SA 4.0 | live |
| Bible en — `KJV` | eBible.org `eng-kjv2006` | public domain | live |
| Bible fr — `LSG` | eBible.org `fraLSG` (Louis Segond 1910) | public domain | live |
| Bible sw — SUV | Bible Society of Tanzania / Kenya via Digital Bible Library | licence required | requested (draft) |
| Sabbath School | Adventech API (sabbath-school.adventech.io), 29 Swahili quarterlies | © GC, permission required | requested (draft) |
| EGW writings | EGW Writings API (a.egwwritings.org, OAuth app registration) | © White Estate, permission required | requested (draft) |
| Hymnals (NZK, SDAH) | publishers (unconfirmed — see docs/permissions) | permission required | requested (draft) |
| 28 Beliefs | ECD / GC Communication | permission required | requested (draft) |

Provenance of every import is recorded in the `content_sources` table (url, sha256, licence, attribution).

## Re-importing a Bible
```sh
mkdir -p content/sources/ebible && cd content/sources/ebible
curl -LO https://ebible.org/Scriptures/swhonen_usfm.zip
cd - && go run ./cmd/ingest bible --dry-run --show JHN.3.16 --code NENO --lang sw --name "…" \
  --license "CC BY-SA 4.0" --attribution "…" --url https://ebible.org/Scriptures/swhonen_usfm.zip \
  --zip content/sources/ebible/swhonen_usfm.zip      # check counts + sample, then drop --dry-run
```
Then `make deploy` (rebuilds the offline packs; versions bump only when text changes).
