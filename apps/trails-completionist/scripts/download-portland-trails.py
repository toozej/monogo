#!/usr/bin/env python3
"""Download complete Portland GIS inventories as WGS84 GeoJSON.

The downloads retain all source segments, names, and statuses. Use the city
boundary to select features within Portland. A city inventory does not prove
that all trails exist in the source data.
"""

import argparse
import datetime
import json
import pathlib
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

BASE_URL = "https://www.portlandmaps.com/arcgis/rest/services/Public/"
LAYERS = {
    "portland-boundary": ("Boundaries/MapServer/13", "CITYNAME='Portland'"),
    "portland-trails": ("Parks_Pathways_and_Trails/MapServer/1", "1=1"),
    "portland-regional-trails": ("Basemap_Color/MapServer/0", "1=1"),
}


def request_json(url, **parameters):
    """Read one public ArcGIS response. Retry temporary network errors."""
    request_url = url + "?" + urllib.parse.urlencode(parameters)
    request = urllib.request.Request(
        request_url, headers={"User-Agent": "trails-completionist/portland-inventory"}
    )
    for attempt in range(4):
        try:
            # Callers use the fixed HTTPS BASE_URL and LAYERS constants.
            # noopengrep: python.lang.security.audit.dynamic-urllib-use-detected.dynamic-urllib-use-detected
            with urllib.request.urlopen(request, timeout=120) as response:
                data = json.load(response)
            if "error" in data:
                raise ValueError(f"ArcGIS returned an error: {data['error']}")
            return data
        except urllib.error.HTTPError as error:
            if error.code not in (408, 429, 500, 502, 503, 504) or attempt == 3:
                raise
        except (urllib.error.URLError, TimeoutError):
            if attempt == 3:
                raise
        time.sleep(2**attempt)
    raise RuntimeError("The GIS request did not complete.")


def read_ids(url, where):
    """Read the complete identifier list. ArcGIS does not cap this list."""
    data = request_json(url + "/query", where=where, returnIdsOnly="true", f="json")
    identifiers = data.get("objectIds")
    if not isinstance(identifiers, list):
        raise TypeError("ArcGIS did not return the feature identifier list.")
    if len(identifiers) != len(set(identifiers)):
        raise ValueError("ArcGIS returned duplicate feature identifiers.")
    return sorted(identifiers)


def write_json(path, data):
    """Replace an output file only after JSON serialization succeeds."""
    temporary = path.with_suffix(path.suffix + ".tmp")
    try:
        with temporary.open("w", encoding="utf-8") as output:
            json.dump(data, output, ensure_ascii=False, separators=(",", ":"))
            output.write("\n")
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)


def identifier_batches(identifiers, maximum_count):
    """Keep GET queries below the GIS server's query-string length limit."""
    batch = []
    encoded_length = 0
    for identifier in identifiers:
        extra = len(str(identifier)) + (3 if batch else 0)
        if batch and (len(batch) >= maximum_count or encoded_length + extra > 3000):
            yield batch
            batch = []
            encoded_length = 0
            extra = len(str(identifier))
        batch.append(identifier)
        encoded_length += extra
    if batch:
        yield batch


def download_layer(name, output_dir, batch_size):
    layer, where = LAYERS[name]
    url = BASE_URL + layer
    metadata = request_json(url, f="json")
    identifiers = read_ids(url, where)
    count = request_json(url + "/query", where=where, returnCountOnly="true", f="json")
    if count.get("count") != len(identifiers):
        raise ValueError(f"{name}: the feature count changed. Repeat the download.")
    oid_fields = [
        field["name"]
        for field in metadata["fields"]
        if field["type"] == "esriFieldTypeOID"
    ]
    if len(oid_fields) != 1:
        raise ValueError(
            f"{name}: the source does not have one object identifier field."
        )
    oid_field = oid_fields[0]
    page_size = min(batch_size, metadata["maxRecordCount"])
    features = []
    for batch in identifier_batches(identifiers, page_size):
        page = request_json(
            url + "/query",
            objectIds=",".join(str(identifier) for identifier in batch),
            outFields="*",
            returnGeometry="true",
            outSR=4326,
            orderByFields=oid_field,
            f="geojson",
        )
        if page.get("type") != "FeatureCollection":
            raise ValueError(
                f"{name}: ArcGIS did not return a GeoJSON FeatureCollection."
            )
        if page.get("exceededTransferLimit") or page.get("properties", {}).get(
            "exceededTransferLimit"
        ):
            raise ValueError(
                f"{name}: ArcGIS truncated a response. Reduce --batch-size."
            )
        page_features = page.get("features", [])
        received = [feature["properties"][oid_field] for feature in page_features]
        if len(received) != len(batch) or set(received) != set(batch):
            raise ValueError(
                f"{name}: a response omitted or duplicated features. Repeat the download."
            )
        features.extend(page_features)
        print(
            f"{name}: received {len(features)}/{len(identifiers)} features",
            file=sys.stderr,
            flush=True,
        )
    if read_ids(url, where) != identifiers:
        raise ValueError(
            f"{name}: the feature identifiers changed. Repeat the download."
        )
    source = {
        "url": url,
        "where": where,
        "downloaded_at": datetime.datetime.now(datetime.UTC).isoformat(),
        "feature_count": len(features),
        "coordinate_reference_system": "EPSG:4326",
        "attribution": "City of Portland, Oregon",
        "selection": "All source features. No status, name, or geographic filter applies, except the boundary name query.",
    }
    write_json(
        output_dir / (name + ".geojson"),
        {
            "type": "FeatureCollection",
            "name": name,
            "source": source,
            "features": features,
        },
    )
    write_json(output_dir / (name + ".metadata.json"), metadata)
    return source


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output-dir",
        type=pathlib.Path,
        required=True,
        help="Directory for GeoJSON files and source metadata.",
    )
    parser.add_argument(
        "--include-regional",
        action="store_true",
        help="Also download the regional trails inventory.",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=500,
        help="Maximum features per request. Default: 500.",
    )
    args = parser.parse_args()
    if args.batch_size < 1:
        parser.error("--batch-size must be greater than zero.")
    args.output_dir.mkdir(parents=True, exist_ok=True)
    names = ["portland-boundary", "portland-trails"]
    if args.include_regional:
        names.append("portland-regional-trails")
    sources = []
    try:
        for name in names:
            sources.append(download_layer(name, args.output_dir, args.batch_size))
        write_json(
            args.output_dir / "portland-download-manifest.json", {"sources": sources}
        )
    except (OSError, ValueError, KeyError) as error:
        print(f"The Portland download failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
