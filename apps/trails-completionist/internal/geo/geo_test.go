package geo

import (
	"math"
	"testing"
)

func TestCoverageUsesSegmentsAndTargetLength(t *testing.T) {
	trail := [][]Point{{{Lat: 45, Lon: -122}, {Lat: 45, Lon: -121.99}}}
	if got := Coverage(trail, NewIndex(trail), 25); math.Abs(got-1) > 1e-9 {
		t.Fatalf("sparse matching line coverage=%v", got)
	}
	nearby := [][]Point{{{Lat: 45.003, Lon: -122}, {Lat: 45.003, Lon: -121.99}}}
	if got := Coverage(trail, NewIndex(nearby), 25); got != 0 {
		t.Fatalf("parallel trail 330 metres away matched: %v", got)
	}
	crossing := [][]Point{{{Lat: 44.999, Lon: -121.995}, {Lat: 45.001, Lon: -121.995}}}
	if got := Coverage(trail, NewIndex(crossing), 25); got >= 0.1 {
		t.Fatalf("crossing completed a trail: %v", got)
	}
}

func TestClipCrossingLineAndHole(t *testing.T) {
	b := NewBoundary([][][]Point{{
		{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 1}, {Lat: 1, Lon: 1}, {Lat: 1, Lon: 0}, {Lat: 0, Lon: 0}},
		{{Lat: .4, Lon: .4}, {Lat: .4, Lon: .6}, {Lat: .6, Lon: .6}, {Lat: .6, Lon: .4}, {Lat: .4, Lon: .4}},
	}})
	paths := b.Clip([][]Point{{{Lat: .5, Lon: -.1}, {Lat: .5, Lon: 1.1}}})
	if len(paths) != 2 || math.Abs(paths[0][0].Lon) > 1e-9 || math.Abs(paths[0][1].Lon-.4) > 1e-9 || math.Abs(paths[1][0].Lon-.6) > 1e-9 || math.Abs(paths[1][1].Lon-1) > 1e-9 {
		t.Fatalf("clip did not preserve hole or crossing: %#v", paths)
	}
	if b.Contains(Point{Lat: .5, Lon: .5}) || !b.Contains(Point{Lat: .2, Lon: .2}) {
		t.Fatal("polygon hole containment failed")
	}
}

func TestSegmentsDoNotBridgeTrackBreaks(t *testing.T) {
	index := NewIndex([][]Point{{{Lat: 45, Lon: -122}, {Lat: 45, Lon: -121.999}}, {{Lat: 45, Lon: -121.991}, {Lat: 45, Lon: -121.99}}})
	if index.Near(Point{Lat: 45, Lon: -121.995}, 25) {
		t.Fatal("index connected separate GPX segments")
	}
}
