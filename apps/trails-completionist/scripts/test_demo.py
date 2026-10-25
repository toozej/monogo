"""Check the demo launcher without network requests or a Go build."""

import gzip
import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest

APP = pathlib.Path(__file__).resolve().parents[1]


class DemoTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = pathlib.Path(self.temporary.name) / "demo repository"
        self.app = self.root / "apps" / "trails-completionist"
        self.app.mkdir(parents=True)
        self.mock = self.root / "mock"
        self.mock.mkdir()
        self.output = self.root / "demo output"
        self.download_log = self.root / "downloads.jsonl"
        self.run_log = self.root / "runs.jsonl"
        self.build_log = self.root / "builds.jsonl"
        shutil.copy2(APP / "demo.sh", self.app / "demo.sh")
        for name in [
            "trails_input_file_example.txt",
            "trails_portland_inventory_example.geojson",
        ]:
            shutil.copy2(APP / name, self.app / name)
        self.archive = self.root / "fixture.osm.gz"
        self.archive.write_bytes(
            gzip.compress(
                b'<osm version="0.6"><node id="1" lat="45" lon="-122"/></osm>'
            )
        )
        self.env = {
            **os.environ,
            "PATH": str(self.mock) + os.pathsep + os.environ["PATH"],
            "DEMO_TEST_ARCHIVE": str(self.archive),
            "DEMO_TEST_DOWNLOAD_LOG": str(self.download_log),
            "DEMO_TEST_RUN_LOG": str(self.run_log),
            "DEMO_TEST_BUILD_LOG": str(self.build_log),
        }
        self.env.pop("BIN", None)
        self.env.pop("SERVE_ADDRESS", None)
        self.write_executable(
            self.root / "mock-app",
            """#!/usr/bin/env python3
import json,os,pathlib,sys
with open(os.environ['DEMO_TEST_RUN_LOG'],'a') as log: log.write(json.dumps(sys.argv[1:])+'\\n')
for flag in ['--checklistFile','--htmlFile','--reportFile']:
 path=pathlib.Path(sys.argv[sys.argv.index(flag)+1])
 path.write_text('demo output\\n')
""",
        )
        self.write_executable(
            self.mock / "make",
            """#!/usr/bin/env python3
import json,os,pathlib,shutil,sys
with open(os.environ['DEMO_TEST_BUILD_LOG'],'a') as log: log.write(json.dumps(sys.argv[1:])+'\\n')
root=pathlib.Path(sys.argv[sys.argv.index('-C')+1])
(root/'out').mkdir(exist_ok=True)
shutil.copy2(root/'mock-app',root/'out'/'trails-completionist')
""",
        )
        self.write_executable(self.mock / "go", "#!/usr/bin/env bash\nexit 0\n")
        self.write_executable(
            self.mock / "curl",
            """#!/usr/bin/env python3
import json,os,pathlib,shutil,sys
with open(os.environ['DEMO_TEST_DOWNLOAD_LOG'],'a') as log: log.write(json.dumps(sys.argv[1:])+'\\n')
output=pathlib.Path(sys.argv[sys.argv.index('--output')+1])
if os.environ.get('DEMO_TEST_DOWNLOAD_FAIL'):
 output.write_text('partial download')
 sys.exit(22)
if sys.argv[-1].endswith('Portland.osm.gz'):
 if os.environ.get('DEMO_TEST_BAD_GZIP'): output.write_text('not a gzip file')
 else: shutil.copyfile(os.environ['DEMO_TEST_ARCHIVE'],output)
else:
 data={'type':'FeatureCollection','features':[{'type':'Feature','properties':{'CITYNAME':'Portland'},'geometry':{'type':'Polygon','coordinates':[[[-123,44],[-121,44],[-121,46],[-123,46],[-123,44]]]}}]}
 if os.environ.get('DEMO_TEST_BAD_BOUNDARY'): data={'error':{'message':'GIS error'}}
 output.write_text(json.dumps(data))
""",
        )

    def write_executable(self, path, contents):
        path.write_text(contents)
        path.chmod(0o755)

    def launch(self, *options, check=True, env=None):
        return subprocess.run(
            [str(self.app / "demo.sh"), "--output-dir", str(self.output), *options],
            cwd=self.temporary.name,
            env=env or self.env,
            capture_output=True,
            text=True,
            check=check,
            timeout=30,
        )

    @staticmethod
    def entries(path):
        return (
            [json.loads(line) for line in path.read_text().splitlines()]
            if path.exists()
            else []
        )

    def test_build_download_inputs_and_cache_with_empty_tracks(self):
        self.launch("--no-serve")
        args = self.entries(self.run_log)[0]
        self.assertEqual(args[0], "full")
        self.assertEqual(
            args[args.index("--inputFile") + 1],
            str(self.app / "trails_input_file_example.txt"),
        )
        self.assertEqual(
            args[args.index("--inventoryFile") + 1],
            str(self.app / "trails_portland_inventory_example.geojson"),
        )
        self.assertEqual(args[args.index("--trackFiles") + 1], "")
        self.assertIn("--serve=false", args)
        self.assertEqual(args[args.index("--serveAddress") + 1], "0.0.0.0:3000")
        self.assertTrue((self.app / "tracks").is_dir())
        self.assertEqual(len(self.entries(self.build_log)), 1)
        self.assertEqual(len(self.entries(self.download_log)), 2)
        self.assertFalse(list((self.output / "data").glob(".demo-download.*")))
        self.launch("--no-serve")
        self.assertEqual(len(self.entries(self.download_log)), 2)
        self.launch("--no-serve", "--refresh")
        self.assertEqual(len(self.entries(self.download_log)), 4)

    def test_nested_uppercase_gpx_and_default_server(self):
        tracks = self.root / "custom tracks"
        (tracks / "nested").mkdir(parents=True)
        (tracks / "nested" / "walk.GPX").write_text("fixture")
        self.launch("--tracks-dir", str(tracks))
        args = self.entries(self.run_log)[0]
        self.assertEqual(args[args.index("--trackFiles") + 1], str(tracks))
        self.assertIn("--serve=true", args)

    def test_make_supplied_binary_and_tcx_detection(self):
        tracks = self.app / "tracks"
        tracks.mkdir()
        (tracks / "walk.TCX").write_text("fixture")
        self.launch("--no-serve", env={**self.env, "BIN": str(self.root / "mock-app")})
        self.assertFalse(self.build_log.exists())
        args = self.entries(self.run_log)[0]
        self.assertEqual(args[args.index("--trackFiles") + 1], str(tracks))

    def test_listen_address_environment_and_flag_override(self):
        self.launch("--no-serve", env={**self.env, "SERVE_ADDRESS": "127.0.0.1:3001"})
        args = self.entries(self.run_log)[-1]
        self.assertEqual(args[args.index("--serveAddress") + 1], "127.0.0.1:3001")
        self.launch(
            "--no-serve",
            "--listen-address",
            "192.168.1.20:3002",
            env={**self.env, "SERVE_ADDRESS": "127.0.0.1:3001"},
        )
        args = self.entries(self.run_log)[-1]
        self.assertEqual(args[args.index("--serveAddress") + 1], "192.168.1.20:3002")

    def test_failed_refresh_retains_cache_and_removes_partial_downloads(self):
        self.launch("--no-serve")
        original = (self.output / "data" / "Portland.osm.gz").read_bytes()
        result = self.launch(
            "--refresh", check=False, env={**self.env, "DEMO_TEST_DOWNLOAD_FAIL": "1"}
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(
            (self.output / "data" / "Portland.osm.gz").read_bytes(), original
        )
        self.assertEqual(len(self.entries(self.run_log)), 1)
        self.assertFalse(list((self.output / "data").glob(".demo-download.*")))

    def test_invalid_downloads_do_not_start_the_app(self):
        for failure in ["DEMO_TEST_BAD_GZIP", "DEMO_TEST_BAD_BOUNDARY"]:
            with self.subTest(failure=failure):
                result = self.launch(
                    "--no-serve",
                    "--refresh",
                    check=False,
                    env={**self.env, failure: "1"},
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(self.run_log.exists())
                self.assertFalse(list((self.output / "data").glob(".demo-download.*")))

    def test_bad_options_fail_before_build_or_download(self):
        for options in [
            ("--unknown",),
            ("--tracks-dir",),
            ("--listen-address",),
            ("--output-dir", "--no-serve"),
        ]:
            with self.subTest(options=options):
                result = self.launch(*options, check=False)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(self.build_log.exists())
                self.assertFalse(self.download_log.exists())


if __name__ == "__main__":
    unittest.main()
