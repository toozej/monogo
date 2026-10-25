package matcher

import (
	"testing"

	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

func TestMatchTrailsAllowsRawOnlyChecklist(t *testing.T) {
	raw := []types.Trail{{Name: "Uncompleted Trail", Park: "Forest Park"}}
	combined, err := MatchTrails(nil, raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(combined) != 1 || combined[0].Name != raw[0].Name {
		t.Fatalf("combined trails = %#v", combined)
	}
}

func TestMatchTrailsPreservesIdentityAndUnlistedCompletions(t *testing.T) {
	raw := []types.Trail{{ID: "park-a/maple", Name: "Maple Trail", Park: "A", Length: "1.2"}, {ID: "park-b/maple", Name: "Maple Trail", Park: "B"}}
	completed := []types.Trail{{ID: "park-a/maple", Name: "Maple Trail", Completed: true}, {ID: "other", Name: "Other Trail", Completed: true}}
	got, err := MatchTrails(completed, raw)
	if err != nil || len(got) != 3 || !got[0].Completed || got[0].Length != "1.2" || got[1].Completed || got[2].Name != "Other Trail" {
		t.Fatalf("identity merge failed: %#v, %v", got, err)
	}
	got, err = MatchTrails(completed, nil)
	if err != nil || len(got) != 2 {
		t.Fatalf("GPX-only trails were lost: %#v, %v", got, err)
	}
}

func TestMergeKeepsInventoryMetadataAndEarliestCompletion(t *testing.T) {
	raw := []types.Trail{{ID: "city/1", Name: "Municipal name", Park: "Park", Type: "Trail/Pathway", Length: "1.2", URL: "https://example.test/trail", Completed: true, CompletionDate: "08/01/2026"}}
	completed := []types.Trail{
		{ID: "city/1", Name: "Other name", Completed: true, CompletionDate: "07/01/2026"},
		{ID: "city/1", Name: "Other name", Completed: true, CompletionDate: "09/01/2026"},
		{ID: "city/1", Completed: false},
	}
	got, err := MatchTrails(completed, raw)
	if err != nil || len(got) != 1 {
		t.Fatalf("merge failed: %#v, %v", got, err)
	}
	if got[0].Name != raw[0].Name || got[0].Park != raw[0].Park || got[0].Type != raw[0].Type || got[0].Length != raw[0].Length || got[0].URL != raw[0].URL || got[0].CompletionDate != "07/01/2026" {
		t.Fatalf("merge changed source metadata or completion date: %#v", got[0])
	}
}

func TestMergeOmitsCoveredOSMRowsWithoutPromotingPartialInventory(t *testing.T) {
	point := func(lon float64) types.Point { return types.Point{Lat: 45, Lon: lon} }
	raw := []types.Trail{{ID: "city/long", Name: "Municipal trail", Geometry: [][]types.Point{{point(-122), point(-121.99)}}}}
	completed := []types.Trail{
		{ID: "osm/way/1", Name: "Short OSM component", Completed: true, Geometry: [][]types.Point{{point(-122), point(-121.999)}}},
		{ID: "osm/way/2", Name: "Unlisted continuation", Completed: true, Geometry: [][]types.Point{{point(-121.999), point(-121.98)}}},
	}
	got, err := MatchTrails(completed, raw)
	if err != nil || len(got) != 2 || got[0].Completed || got[1].ID != "osm/way/2" {
		t.Fatalf("partial inventory became complete or a discovery was lost: %#v, %v", got, err)
	}
}

func TestMergePreservesDistinctInventoryRecordsWithEqualGeometry(t *testing.T) {
	geometry := [][]types.Point{{{Lat: 45, Lon: -122}, {Lat: 45, Lon: -121.999}}}
	raw := []types.Trail{{ID: "city/1", Name: "Trail", Geometry: geometry}, {ID: "regional/1", Name: "Trail", Geometry: geometry}}
	completed := []types.Trail{{ID: "city/1", Completed: true}, {ID: "regional/1", Completed: true}, {ID: "osm/way/1", Completed: true, Geometry: geometry}}
	got, err := MatchTrails(completed, raw)
	if err != nil || len(got) != 2 || !got[0].Completed || !got[1].Completed {
		t.Fatalf("source records were lost or an OSM duplicate remained: %#v, %v", got, err)
	}
}
