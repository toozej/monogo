# Portland trail review

Review date: 2026-10-25. Baseline revision: `757778e5`, before the changes in this review.

The previous application was unable to produce a complete OSM trail inventory.
It read OSM only to interpret optional GPX tracks. The text list defined the checklist when the user supplied no tracks.
It ignored unnamed ways, route relations, steps, cycleways, and roads used by routes for walks.
It returned at most five matches per GPX file.

The previous score averaged distances from twenty sampled GPX points to twenty sampled OSM vertices.
Its acceptance threshold allowed average distances below 500 metres. This calculation was not a Fréchet distance.

It had the potential to reject a short trail inside a long walk or accept a nearby trail that the user did not walk.
It also had the potential to join absent OSM nodes, discard GPX errors, merge trails with equal names in different parks, and lose unlisted completions.

Text deduplication discarded different segments with equal names and parks. The example contains two such additional connector records.

The new catalog enumerates OSM candidates independently of GPX tracks.
The audit preserves each municipal segment and compares geometry independently of its name.
It reports uncertain identities without a selection through fuzzy name comparison.
The README describes candidate selection, coverage calculations, IDs, cache behavior, and input formats.

## Data sources

The primary source is Portland's [PP&R Trails Network](https://www.portlandmaps.com/arcgis/rest/services/Public/Parks_Pathways_and_Trails/MapServer?f=pjson).
Its [line layer](https://www.portlandmaps.com/arcgis/rest/services/Public/Parks_Pathways_and_Trails/MapServer/1?f=pjson) includes geometry, trail names, parks, status, type, pedestrian permission, and GlobalIDs.

The complete downloaded snapshot contains 10,591 segments and 636 distinct nonblank names. It contains 6,261 unnamed segments.
Statuses include 8,783 `existing`, 1,399 proposed, 274 informal, 112 removed, 15 closed, and eight unspecified records.

The inventory also includes sidewalks, water trails, maintenance roads, and other types.
Most records do not specify pedestrian permission. If the selection excluded unspecified permission, it would discard most of the inventory.

The second official source is the [regional Trails layer](https://www.portlandmaps.com/arcgis/rest/services/Public/Basemap_Color/MapServer/0?f=pjson).
The complete snapshot contains 26,395 segments and 3,091 distinct names.
It includes open, private, fee, on-street, decommissioned, and construction records.
Both inventories extend outside the city. The comparison uses the [official Portland boundary](https://www.portlandmaps.com/arcgis/rest/services/Public/Boundaries/MapServer/13?f=pjson).
The downloader saves complete source records in EPSG:4326 and saves the layer metadata and a download manifest.

[Portland.gov's trails page](https://www.portland.gov/parks/nature/trails) provides hiking locations and park maps.
It does not provide an inventory of every physical trail within the city.

[AllTrails Portland](https://www.alltrails.com/us/oregon/portland) lists curated walking and hiking routes.
The directory returns HTTP 403 to automated link checks. Open the directory in a browser.

Several routes reuse the same physical trails. Some listed routes lie outside Portland city limits.
AllTrails supports [individual file exports](https://support.alltrails.com/hc/en-us/articles/37230403315476-Downloading-files-from-AllTrails).

The research did not bulk scrape AllTrails. Its [terms](https://www.alltrails.com/en-gb/terms) restrict automated scraping.

[Portland category on OregonHikers](https://www.oregonhikers.org/field_guide/Category%3APortland) mixes hikes, trailheads, junctions, viewpoints, and other features.
It can help check aliases and possible omissions. It is not a city segment inventory.
[SWTrails GIS](https://swtrails.org/trails/gis-mapping/) supplies another source for routes for walks in Southwest Portland.
[Metro RLIS](https://www.oregonmetro.gov/resources/rlis-live) supplies regional GIS data.

The OSM selection follows the documented distinction between physical ways and [`hiking` route relations](https://wiki.openstreetmap.org/wiki/Tag:route%3Dhiking).
A route can contain paths, steps, cycleways, and roads. The route's name can exist only on the relation.
The catalog preserves documented route member roles. It separates optional role groups from main geometry.
The [trail guidance from OSM US](https://openstreetmap.us/our-work/trails/how-to-map/) describes trail and access tags.

No researched source establishes that it contains every physical trail inside Portland.
The official inventories supply a larger reference set. They do not prove completeness of the physical network or of OSM.

## Reproducible baseline

The baseline used a fresh [XML extract of Portland from BBBike](https://download.bbbike.org/osm/bbbike/Portland/Portland.osm.gz).
The server reported `Last-Modified: Sun, 25 Oct 2026 04:08:13 GMT`.
The gzip file contains 160,375,354 bytes. The uncompressed XML contains 1,926,071,049 bytes.
The review downloaded the municipal inventories on the same day.

The baseline command used the original binary and these arguments:

```sh
trails-completionist full \
  --osmRegionFile Portland.osm \
  --inputFile apps/trails-completionist/trails_input_file_example.txt \
  --checklistFile baseline-checklist.md \
  --htmlFile baseline.html
```

The command exited successfully. The input contains 189 records. The original checklist contains 163 entries, all incomplete.
The original OSM parser used 47.7 seconds and 5,585,084 KiB peak resident memory for the initial run.

These measurements describe one local run. They do not represent controlled performance benchmarks.
The validation run received no user activity files. GPX validation uses synthetic tracks with known geometry.

The changed text parser retains 165 records. The two additional entries have equal names and parks but different lengths.
The current example file remains unchanged.

The updated run also performs the city-boundary audit. Its initial run uses two XML passes to reduce retained data.
Its peak memory decreases by approximately 71%. Its initial run takes approximately five seconds longer than the original run.
The original binary did not perform this audit, so the runs do different work.

| Local run | Elapsed seconds | Peak resident memory, KiB |
| --- | ---: | ---: |
| Original `full`, initial XML load | 47.7 | 5,585,084 |
| Updated `full`, initial XML load and city audit | 52.9 | 1,629,140 |
| Updated `full`, cached geometry and city audit | 0.8 | 341,736 |
| Updated audit of both complete inventories, cached geometry | 3.2 | 380,520 |
| Updated `full` with text and both complete inventories, cached geometry | 3.7 | 385,516 |

The new trail cache occupies approximately 35 MiB. The original complete-map cache occupied approximately 488 MiB.

## City comparison results

The final catalog contains 24,641 candidates. These include 39 route records and 22,769 unnamed records.
The city polygon clips usable geometry. Incomplete records with unknown locations can remain visible.

The extract has no absent referenced candidate nodes. It lacks 175 referenced relation members across 11 incomplete route records.
These diagnostics describe the exported route networks. Absent members can lie outside Portland or outside the extract.

The catalog also detects 59 nonstandard role occurrences. It preserves their geometry in incomplete role groups for review.

The comparison uses a 25-metre distance threshold and 90% required source coverage.
The tables count source segments and include unnamed records. They do not count unique public hiking trails.

| Inventory | Source records | Records after city clipping | Geometry correspondence | Ambiguous | Partial | Absent |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| PP&R pathways and trails | 10,591 | 9,773 | 3,586 | 4,417 | 942 | 828 |
| Regional trails | 26,395 | 8,658 | 3,549 | 4,700 | 206 | 203 |
| Combined | 36,986 | 18,431 | 7,135 | 9,117 | 1,148 | 1,031 |

The PP&R selection contains 6,139 `existing` segments of type `Trail/Pathway` or `Firelane` without an explicit pedestrian prohibition.
Of these segments, 2,542 have geometry correspondence, 3,324 remain ambiguous, 134 have partial coverage, and 139 have no nearby counterpart.
The 139 records provide a useful initial set for manual checks. They do not prove that OSM lacks 139 trails.

The text example produces 47 unique name matches, 29 ambiguous records, and 89 unresolved names within this catalog.
These unresolved names include aliases and locations outside the selected boundary.
The combined text and complete inventories produce 18,596 checklist entries and HTML rows.
The two example files produce 172 entries after city clipping. One of the eight GeoJSON sample segments lies outside the selected city geometry.

Regression checks pass for both `trails-completionist` and `golang-starter`.
These checks cover shared packages and use the race detector.
`go vet`, the browser script test, and three downloader tests also pass.
Tests cover nested relations, absent references, malformed exports, gzip input, polygon holes, border crossings, sparse GPX points, track breaks, and repeated names.
They also cover more than five completed targets, additive inputs, metadata round trips, omitted GIS records, changed feature IDs, and transfer limits.

SHA-256 values identify the downloaded snapshots:

| File | SHA-256 |
| --- | --- |
| `Portland.osm.gz` | `d265257a8282f3a9368eca4631ea78309cd23ec6aad80360f0590a27833f018c` |
| `portland-trails.geojson` | `e3df60481f6bee7af04a8c094e231d271b3deb43f0cdfdf90a34a5bdba75d897` |
| `portland-regional-trails.geojson` | `81173c6a4baf941cc9c568b7171a6378745732446cd7df8c1e3812677fb8740c` |
| `portland-boundary.geojson` | `c75a6b7988ba042c3b70d37457d864ce81bbba80a314b4f8ed1162ace4f11c06` |

Use the README commands to download and audit newer snapshots.
The review artifacts and full JSON findings remain under `/tmp/trails-completionist-review` in this workspace.
The repository contains the small GeoJSON example, downloader, review report, and regression tests.

## Limits that remain

The report separates candidate counts, source segments, and trail identities. These quantities are not interchangeable.
OSM candidates include sidewalks and crossings. The inventories include proposed, removed, private, and non-hiking features.
Several inventories or routes can describe the same physical segment. Their lengths and counts can overlap.

A geometry match establishes proximity and coverage. It does not prove identity, public access, or legal permission to walk.
Parallel paths within the distance threshold can remain ambiguous.
The local distance projection and ten-metre sampling suit city-scale comparison. They do not provide survey precision.

Text names alone cannot resolve all aliases or repeated names.
For example, the input's `Saltzman Road` differs from OSM's `Northwest Saltzman Road`.
The application preserves this uncertainty. Municipal geometry permits comparison with no removal of name prefixes or arbitrary fuzzy comparisons.

An OSM extract can omit relation members outside its extraction area.
The catalog retains incomplete route records and reports absent references. It prevents automatic GPX completion for those routes.
An incomplete record without usable geometry can remain in the report because the application cannot establish its location.
This behavior keeps empty geometry visible as an omission.
