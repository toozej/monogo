package catalog

import (
	"testing"

	"github.com/toozej/monogo/apps/trails-completionist/internal/geo"
	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
	"github.com/toozej/monogo/pkg/osm"
)

func sampleData() *osm.OSMData {
	return &osm.OSMData{Nodes: map[int64]osm.OSMNode{
		1: {ID: 1, Lat: 45, Lon: -122}, 2: {ID: 2, Lat: 45, Lon: -121.999}, 3: {ID: 3, Lat: 45, Lon: -121.998},
		4: {ID: 4, Lat: 45.01, Lon: -122}, 5: {ID: 5, Lat: 45.01, Lon: -121.999},
	}, Ways: map[int64]osm.OSMWay{
		10: {ID: 10, Nodes: []int64{1, 2}, Tags: map[string]string{"highway": "path", "name": "Maple Trail"}},
		11: {ID: 11, Nodes: []int64{2, 3}, Tags: map[string]string{"highway": "path", "name": "Maple Trail"}},
		12: {ID: 12, Nodes: []int64{4, 5}, Tags: map[string]string{"highway": "steps", "name": "Maple Trail"}},
		13: {ID: 13, Nodes: []int64{1, 3}, Tags: map[string]string{"highway": "cycleway"}},
		14: {ID: 14, Nodes: []int64{2, 5}, Tags: map[string]string{"highway": "residential"}},
	}, Relations: map[int64]osm.OSMRelation{}}
}

func TestCatalogConnectedNamesAndUnnamedWays(t *testing.T) {
	trails, d := Build(sampleData(), geo.Boundary{})
	if len(trails) != 3 || d.IncompleteTrails != 0 {
		t.Fatalf("catalog=%#v, diagnostics=%+v", trails, d)
	}
	if len(trails[0].OSMIDs) != 2 || trails[1].Name != "Maple Trail" || trails[2].Name == "" {
		t.Fatalf("lost connected or unnamed ways: %#v", trails)
	}
	resolved, r := Audit([]types.Trail{{Name: "Maple Trail"}}, trails, d, 25, .9)
	if r.Inventory[0].Match != "ambiguous" || len(resolved[0].Geometry) != 0 {
		t.Fatal("same name in different places was treated as unique")
	}
}

func TestCatalogNestedRoutesMissingReferencesAndCycles(t *testing.T) {
	data := sampleData()
	data.Relations[20] = osm.OSMRelation{ID: 20, Tags: map[string]string{"type": "route", "route": "hiking", "name": "City Loop", "alt_name": "Local Loop"}, Members: []osm.Member{{Type: "relation", Ref: 21}, {Type: "way", Ref: 14}, {Type: "way", Ref: 14}}}
	data.Relations[21] = osm.OSMRelation{ID: 21, Members: []osm.Member{{Type: "way", Ref: 13}, {Type: "relation", Ref: 20}, {Type: "way", Ref: 99}}}
	trails, d := Build(data, geo.Boundary{})
	if d.MissingMembers != 1 || d.RelationCycles != 1 || d.IncompleteTrails != 1 {
		t.Fatalf("diagnostics=%+v", d)
	}
	for _, trail := range trails {
		if trail.ID == "osm/relation/20" && len(trail.Geometry) == 2 && len(trail.OSMIDs) == 3 && trail.Incomplete {
			return
		}
	}
	t.Fatalf("route members were lost or counted twice: %#v", trails)
}

func TestMissingNodeDoesNotCreateShortcut(t *testing.T) {
	data := sampleData()
	data.Ways = map[int64]osm.OSMWay{1: {ID: 1, Nodes: []int64{1, 99, 3}, Tags: map[string]string{"highway": "path"}}}
	trails, d := Build(data, geo.Boundary{})
	if len(trails) != 1 || len(trails[0].Geometry) != 0 || d.MissingNodes != 1 {
		t.Fatalf("gap became a line: %#v, %+v", trails, d)
	}
}

func TestAuditGeometryAcrossSplitWays(t *testing.T) {
	data := sampleData()
	delete(data.Ways, 13)
	trails, d := Build(data, geo.Boundary{})
	input := []types.Trail{{ID: "source/1", Name: "Another name", Geometry: [][]types.Point{{{Lat: 45, Lon: -122}, {Lat: 45, Lon: -121.998}}}}}
	_, r := Audit(input, trails, d, 25, .9)
	if r.Inventory[0].Match != "geometry" || r.Inventory[0].Coverage < .99 {
		t.Fatalf("split ways did not match: %+v", r.Inventory[0])
	}
}

func TestRouteRolesPreserveMainAndOptionalGeometry(t *testing.T) {
	data := sampleData()
	data.Relations[20] = osm.OSMRelation{ID: 20, Tags: map[string]string{"type": "route", "route": "hiking", "name": "City Loop"}, Members: []osm.Member{{Type: "way", Ref: 10, Role: "main"}, {Type: "way", Ref: 14, Role: "alternative"}, {Type: "relation", Ref: 21, Role: "approach"}, {Type: "way", Ref: 11, Role: "parking"}, {Type: "way", Ref: 12, Role: "unrecognized"}}}
	data.Relations[21] = osm.OSMRelation{ID: 21, Members: []osm.Member{{Type: "way", Ref: 13, Role: "main"}}}
	trails, d := Build(data, geo.Boundary{})
	if d.UnsupportedRoles != 1 {
		t.Fatalf("unknown roles did not produce a diagnostic: %+v", d)
	}
	seen := map[string]types.Trail{}
	for _, trail := range trails {
		seen[trail.ID] = trail
	}
	for _, id := range []string{"osm/relation/20", "osm/relation/20/alternative", "osm/relation/20/approach"} {
		trail, ok := seen[id]
		if !ok || len(trail.Geometry) != 1 || trail.Incomplete {
			t.Fatalf("lost main or optional geometry for %s: %#v", id, trail)
		}
	}
	if len(seen["osm/relation/20"].OSMIDs) != 2 {
		t.Fatal("optional or parking geometry became part of the main route")
	}
	if !seen["osm/relation/20/unknown:unrecognized"].Incomplete {
		t.Fatal("unknown role received normal completion eligibility")
	}
}
