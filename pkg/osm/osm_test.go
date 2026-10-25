package osm

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRelationsAndUnorderedNodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "region.osm")
	contents := `<osm version="0.6"><way id="3"><nd ref="1"/><nd ref="2"/><tag k="highway" v="steps"/></way><relation id="4"><member type="way" ref="3" role=""/><tag k="type" v="route"/><tag k="route" v="hiking"/></relation><node id="1" lat="45" lon="-123"/><node id="2" lat="46" lon="-122"/></osm>`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		data, err := LoadOSMData(path, false)
		if err != nil {
			t.Fatal(err)
		}
		if data.Ways[3].BBox != [4]float64{45, -123, 46, -122} || len(data.Relations[4].Members) != 1 {
			t.Fatalf("lost relation or bounds: %#v", data)
		}
	}
	// A replacement with an older mtime must not use a cache for another size.
	if err := os.WriteFile(path, []byte(`<osm><node id="9" lat="0" lon="0"/></osm>`), 0o600); err != nil {
		t.Fatal(err)
	}
	older := time.Unix(1, 0)
	if err := os.Chtimes(path, older, older); err != nil {
		t.Fatal(err)
	}
	data, err := LoadOSMData(path, false)
	if err != nil || len(data.Relations) != 0 || len(data.Nodes) != 1 {
		t.Fatalf("stale cache: %#v, %v", data, err)
	}
}

func TestRejectIncompleteAndInvalidExports(t *testing.T) {
	for _, contents := range []string{"", `<html>error</html>`, `<osm><remark>runtime error: timeout</remark></osm>`, `<osm><node id="abc"/></osm>`, `<osm><node id="1" lat="NaN"/></osm>`, `<osm><way id="3">`, `<osm><node id="1"/></osm>`, `<osm/>unexpected`, `<osm><way id="1"><nd/></way></osm>`, `<osm><relation id="1"><member type="invalid" ref="3"/></relation></osm>`} {
		path := filepath.Join(t.TempDir(), "invalid.osm")
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadOSMData(path, false); err == nil {
			t.Fatalf("accepted invalid export %q", contents)
		}
	}
}

func TestLoadGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "region.osm.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := gzip.NewWriter(f)
	if _, err := w.Write([]byte(`<osm><node id="1" lat="45" lon="-122"/></osm>`)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := LoadOSMData(path, false)
	if err != nil || len(data.Nodes) != 1 {
		t.Fatalf("gzip: %#v, %v", data, err)
	}
}

func TestTrailLoaderRetainsRoadMembersAndMissingReferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "region.osm")
	contents := `<osm><node id="1" lat="45" lon="-122"/><node id="2" lat="45" lon="-121.99"/><node id="3" lat="46" lon="-123"/>
<way id="10"><nd ref="1"/><nd ref="2"/><tag k="highway" v="path"/></way>
<way id="11"><nd ref="2"/><nd ref="99"/><tag k="highway" v="residential"/></way>
<way id="12"><nd ref="3"/><tag k="building" v="yes"/></way>
<relation id="20"><member type="way" ref="11" role=""/><member type="way" ref="98" role=""/></relation>
<relation id="21"><member type="relation" ref="20" role=""/><tag k="type" v="route"/><tag k="route" v="foot"/></relation></osm>`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		data, err := LoadTrailData(path, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(data.Nodes) != 2 || len(data.Ways) != 2 || len(data.Relations) != 2 || data.Ways[11].Nodes[1] != 99 || data.Relations[20].Members[1].Ref != 98 {
			t.Fatalf("filtered loader lost required objects or retained unrelated objects: %#v", data)
		}
	}
}

func TestExplicitPedestrianAndImportedTrailRoadsRemainCandidates(t *testing.T) {
	for _, tags := range []map[string]string{
		{"highway": "service", "foot": "designated"},
		{"highway": "residential", "RLIS:systemname": "Forest Park Trails"},
		{"highway": "service", "route": "hiking"},
	} {
		if !IsTrailCandidate(tags) {
			t.Fatalf("explicit trail road was lost: %v", tags)
		}
	}
}
