"""Check complete GIS downloads without network requests."""

import importlib.util
import json
import pathlib
import tempfile
import unittest
from unittest.mock import patch

SCRIPT = pathlib.Path(__file__).with_name("download-portland-trails.py")
SPEC = importlib.util.spec_from_file_location("portland_download", SCRIPT)
DOWNLOAD = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DOWNLOAD)


class DownloadTests(unittest.TestCase):
    def responses(self, change_ids=False, omit=False, truncated=False):
        id_calls = 0

        def response(url, **parameters):
            nonlocal id_calls
            if not url.endswith("/query"):
                return {
                    "fields": [{"name": "OBJECTID", "type": "esriFieldTypeOID"}],
                    "maxRecordCount": 2,
                }
            if parameters.get("returnIdsOnly"):
                id_calls += 1
                return {
                    "objectIds": [1, 7, 99, 100]
                    if change_ids and id_calls > 1
                    else [1, 7, 99]
                }
            if parameters.get("returnCountOnly"):
                return {"count": 3}
            identifiers = [int(value) for value in parameters["objectIds"].split(",")]
            if omit:
                identifiers = identifiers[:-1]
            return {
                "type": "FeatureCollection",
                "exceededTransferLimit": truncated,
                "features": [
                    {
                        "type": "Feature",
                        "properties": {
                            "OBJECTID": identifier,
                            "TrailName": None,
                            "TrailStatus": "closed",
                        },
                        "geometry": None,
                    }
                    for identifier in identifiers
                ],
            }

        return response

    def test_all_sparse_ids_and_null_names_remain(self):
        with tempfile.TemporaryDirectory() as directory:
            with patch.object(DOWNLOAD, "request_json", side_effect=self.responses()):
                DOWNLOAD.download_layer("portland-trails", pathlib.Path(directory), 2)
            data = json.loads(
                (pathlib.Path(directory) / "portland-trails.geojson").read_text()
            )
            self.assertEqual(
                [feature["properties"]["OBJECTID"] for feature in data["features"]],
                [1, 7, 99],
            )
            self.assertTrue(
                all(
                    feature["properties"]["TrailName"] is None
                    for feature in data["features"]
                )
            )
            self.assertTrue(
                all(
                    feature["properties"]["TrailStatus"] == "closed"
                    for feature in data["features"]
                )
            )

    def test_incomplete_or_changing_download_fails(self):
        for options in ({"omit": True}, {"truncated": True}, {"change_ids": True}):
            with (
                self.subTest(options=options),
                tempfile.TemporaryDirectory() as directory,
            ):
                with (
                    patch.object(
                        DOWNLOAD, "request_json", side_effect=self.responses(**options)
                    ),
                    self.assertRaises(ValueError),
                ):
                    DOWNLOAD.download_layer(
                        "portland-trails", pathlib.Path(directory), 2
                    )
                self.assertFalse(
                    (pathlib.Path(directory) / "portland-trails.geojson").exists()
                )

    def test_encoded_batches_respect_count_and_url_bounds(self):
        identifiers = list(range(123456789, 123458789))
        batches = list(DOWNLOAD.identifier_batches(identifiers, 500))
        self.assertEqual(
            [identifier for batch in batches for identifier in batch], identifiers
        )
        for batch in batches:
            self.assertLessEqual(len(batch), 500)
            self.assertLessEqual(len("%2C".join(map(str, batch))), 3000)


if __name__ == "__main__":
    unittest.main()
