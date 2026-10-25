package catalog

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/toozej/monogo/apps/trails-completionist/internal/geo"
	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

type Candidate struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	OSMIDs     []string `json:"osm_ids,omitempty"`
	Status     string   `json:"status,omitempty"`
	Incomplete bool     `json:"incomplete,omitempty"`
}

type Finding struct {
	ID         string   `json:"id,omitempty"`
	Name       string   `json:"name"`
	Park       string   `json:"park,omitempty"`
	Source     string   `json:"source,omitempty"`
	Type       string   `json:"source_type,omitempty"`
	Status     string   `json:"source_status,omitempty"`
	Match      string   `json:"match"`
	Coverage   float64  `json:"coverage"`
	Candidates []string `json:"candidates,omitempty"`
}

type Report struct {
	OSMFile         string         `json:"osm_file,omitempty"`
	BoundaryFile    string         `json:"boundary_file,omitempty"`
	InputFiles      []string       `json:"input_files,omitempty"`
	DistanceMetres  float64        `json:"distance_metres"`
	MinimumCoverage float64        `json:"minimum_coverage"`
	Diagnostics     Diagnostics    `json:"diagnostics"`
	Summary         map[string]int `json:"summary"`
	Inventory       []Finding      `json:"inventory"`
	OSM             []Candidate    `json:"osm_candidates"`
}

type tile struct{ lat, lon int }

// Lookup indexes names and segment bounds once for an entire inventory.
type Lookup struct {
	trails  []types.Trail
	names   map[string][]int
	tiles   map[tile]map[int]bool
	long    map[int]bool
	indexes map[int]*geo.Index
}

const tileSize = 0.002

func tilesFor(a, b types.Point, buffer float64) (tile, tile) {
	dlat := buffer / 111000
	dlon := dlat / math.Max(0.001, math.Cos(math.Max(math.Abs(a.Lat), math.Abs(b.Lat))*math.Pi/180))
	return tile{int(math.Floor((math.Min(a.Lat, b.Lat) - dlat) / tileSize)), int(math.Floor((math.Min(a.Lon, b.Lon) - dlon) / tileSize))},
		tile{int(math.Floor((math.Max(a.Lat, b.Lat) + dlat) / tileSize)), int(math.Floor((math.Max(a.Lon, b.Lon) + dlon) / tileSize))}
}

func NewLookup(trails []types.Trail) *Lookup {
	l := &Lookup{trails: trails, names: map[string][]int{}, tiles: map[tile]map[int]bool{}, long: map[int]bool{}, indexes: map[int]*geo.Index{}}
	for i, trail := range trails {
		seen := map[string]bool{}
		for _, name := range append([]string{trail.Name}, trail.Aliases...) {
			key := Normalize(name)
			if key != "" && !seen[key] {
				l.names[key] = append(l.names[key], i)
				seen[key] = true
			}
		}
		for _, path := range trail.Geometry {
			for j := 1; j < len(path); j++ {
				a, b := tilesFor(path[j-1], path[j], 0)
				if (b.lat-a.lat+1)*(b.lon-a.lon+1) > 10000 {
					l.long[i] = true
					continue
				}
				for lat := a.lat; lat <= b.lat; lat++ {
					for lon := a.lon; lon <= b.lon; lon++ {
						key := tile{lat, lon}
						if l.tiles[key] == nil {
							l.tiles[key] = map[int]bool{}
						}
						l.tiles[key][i] = true
					}
				}
			}
		}
	}
	return l
}

func (l *Lookup) Nearby(paths [][]types.Point, distance float64) []int {
	seen := map[int]bool{}
	visited := map[tile]bool{}
	for _, path := range paths {
		for j := 1; j < len(path); j++ {
			a, b := tilesFor(path[j-1], path[j], distance)
			if (b.lat-a.lat+1)*(b.lon-a.lon+1) > 10000 {
				for i := range l.trails {
					seen[i] = true
				}
				continue
			}
			for lat := a.lat; lat <= b.lat; lat++ {
				for lon := a.lon; lon <= b.lon; lon++ {
					key := tile{lat, lon}
					if !visited[key] {
						for i := range l.tiles[key] {
							seen[i] = true
						}
						visited[key] = true
					}
				}
			}
		}
	}
	for i := range l.long {
		seen[i] = true
	}
	result := make([]int, 0, len(seen))
	for i := range seen {
		result = append(result, i)
	}
	sort.Ints(result)
	return result
}

func (l *Lookup) index(i int) *geo.Index {
	if l.indexes[i] == nil {
		l.indexes[i] = geo.NewIndex(l.trails[i].Geometry)
	}
	return l.indexes[i]
}

// Audit reports spatial correspondence separately from a unique name match.
// It preserves source records, including missing and ambiguous records.
func Audit(input, trails []types.Trail, diagnostics Diagnostics, distance, minimum float64) ([]types.Trail, Report) {
	lookup := NewLookup(trails)
	report := Report{DistanceMetres: distance, MinimumCoverage: minimum, Diagnostics: diagnostics, Summary: map[string]int{"input_records": len(input), "osm_candidates": len(trails)}}
	for _, t := range trails {
		report.OSM = append(report.OSM, Candidate{t.ID, t.Name, t.OSMIDs, t.Status, t.Incomplete})
		if strings.HasPrefix(t.Name, "Unnamed ") {
			report.Summary["unnamed_osm_candidates"]++
		}
		if strings.HasPrefix(t.ID, "osm/relation/") {
			report.Summary["osm_routes"]++
		}
	}
	resolved := append([]types.Trail(nil), input...)
	textNames := map[string]int{}
	for _, t := range input {
		if len(t.Geometry) == 0 {
			textNames[Normalize(t.Name)]++
		}
	}
	for i, t := range input {
		f := Finding{ID: t.ID, Name: t.Name, Park: t.Park, Source: t.Source, Type: t.Type, Status: t.Status, Match: "missing"}
		var candidates []int
		switch {
		case t.Incomplete:
			f.Match = "incomplete"
		case len(t.Geometry) == 0:
			seen := map[int]bool{}
			for _, name := range append([]string{t.Name}, t.Aliases...) {
				for _, c := range lookup.names[Normalize(name)] {
					if !seen[c] {
						candidates = append(candidates, c)
						seen[c] = true
					}
				}
			}
			if len(candidates) == 1 && textNames[Normalize(t.Name)] == 1 {
				f.Match = "name"
				resolved[i].Geometry = trails[candidates[0]].Geometry
				resolved[i].Incomplete = trails[candidates[0]].Incomplete
			} else if len(candidates) > 0 {
				f.Match = "ambiguous"
			}
		default:
			var union [][]types.Point
			strong := 0
			for _, c := range lookup.Nearby(t.Geometry, distance) {
				coverage := geo.Coverage(t.Geometry, lookup.index(c), distance)
				if coverage > 0 {
					candidates = append(candidates, c)
					union = append(union, trails[c].Geometry...)
					if coverage >= minimum {
						strong++
					}
				}
			}
			f.Coverage = geo.Coverage(t.Geometry, geo.NewIndex(union), distance)
			if f.Coverage >= minimum {
				f.Match = "geometry"
				if strong > 1 {
					f.Match = "ambiguous"
				}
			} else if f.Coverage > 0 {
				f.Match = "partial"
			}
		}
		sort.Ints(candidates)
		for _, c := range candidates {
			f.Candidates = append(f.Candidates, trails[c].ID)
			if f.Match == "name" || f.Match == "geometry" {
				resolved[i].OSMIDs = append(resolved[i].OSMIDs, trails[c].OSMIDs...)
			}
		}
		resolved[i].MatchStatus = f.Match
		report.Inventory = append(report.Inventory, f)
		report.Summary[f.Match]++
	}
	return resolved, report
}

func WriteReport(path string, report Report) error {
	f, err := os.Create(path) // #nosec G304
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(report)
	closeErr := f.Close()
	if encodeErr != nil {
		return fmt.Errorf("write OSM audit: %w", encodeErr)
	}
	return closeErr
}
