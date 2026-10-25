package parser

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tkrajina/gpxgo/gpx"
	"github.com/toozej/monogo/apps/trails-completionist/internal/catalog"
	"github.com/toozej/monogo/apps/trails-completionist/internal/geo"
	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
	"github.com/toozej/monogo/pkg/osm"
)

const DefaultDistanceMetres = geo.DefaultDistanceMetres
const DefaultMinimumCoverage = 0.9

func OutputResults(results []types.TrailResult, format string) {
	if format == "csv" {
		writer := csv.NewWriter(os.Stdout)
		_ = writer.Write([]string{"Filename", "TravelDate", "TrailName", "TrailType", "LengthMiles", "CoveragePercent", "OSMID"})
		for _, result := range results {
			for _, match := range result.Matches {
				_ = writer.Write([]string{result.Filename, result.TravelDate.Format("2006-01-02"), match.Name, match.Type, fmt.Sprint(match.Length), fmt.Sprintf("%.2f", match.Similarity*100), fmt.Sprint(match.OSMId)})
			}
		}
		writer.Flush()
		return
	}
	for _, result := range results {
		fmt.Printf("%s (%s): %d matches\n", result.Filename, result.TravelDate.Format("2006-01-02"), len(result.Matches))
	}
}

func ParseTrailsFromTrackFiles(directory string, recursive bool, data *osm.OSMData) ([]types.Trail, error) {
	if data == nil {
		return nil, fmt.Errorf("OSM region data is required to match GPX tracks. Provide --osmRegionFile or OSM_REGION_FILE")
	}
	targets, _ := catalog.Build(data, geo.Boundary{})
	return ParseTracksWithTargets(directory, recursive, targets, DefaultDistanceMetres, DefaultMinimumCoverage)
}

// ParseTracksWithTargets requires coverage of each complete target, not the whole GPX track.
// It preserves segment breaks and returns every qualifying target.
func ParseTracksWithTargets(directory string, recursive bool, targets []types.Trail, distance, minimum float64) ([]types.Trail, error) {
	if math.IsNaN(distance) || math.IsInf(distance, 0) || distance <= 0 || distance > 1000 || math.IsNaN(minimum) || minimum <= 0 || minimum > 1 {
		return nil, fmt.Errorf("distance must be positive and coverage must be greater than zero and at most one")
	}
	lookup := catalog.NewLookup(targets)
	completed := map[string]types.Trail{}
	files := 0
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && !recursive && path != directory {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".gpx") {
			return nil
		}
		files++
		paths, date, err := readGPX(path)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		index := geo.NewIndex(paths)
		for _, i := range lookup.Nearby(paths, distance) {
			target := targets[i]
			status := strings.ToLower(target.Status)
			if target.Incomplete || strings.HasPrefix(status, "proposed") || strings.HasPrefix(status, "removed") || strings.HasPrefix(status, "closed") || strings.HasPrefix(status, "decommissioned") {
				continue
			}
			if geo.Coverage(target.Geometry, index, distance)+1e-12 < minimum {
				continue
			}
			target.Completed = true
			target.CompletionDate = date.Format("01/02/2006")
			id := target.ID
			if id == "" {
				id = target.Park + "/" + target.Name
			}
			if previous, ok := completed[id]; ok {
				previousDate, _ := time.Parse("01/02/2006", previous.CompletionDate)
				if !date.Before(previousDate) {
					continue
				}
			}
			completed[id] = target
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if files == 0 {
		return nil, fmt.Errorf("no GPX files found in %s", directory)
	}
	trails := make([]types.Trail, 0, len(completed))
	for _, trail := range completed {
		trails = append(trails, trail)
	}
	sort.Slice(trails, func(i, j int) bool { return trails[i].ID < trails[j].ID })
	return trails, nil
}

func readGPX(path string) ([][]types.Point, time.Time, error) {
	data, err := gpx.ParseFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	var paths [][]types.Point
	var date time.Time
	add := func(points []gpx.GPXPoint) error {
		var line []types.Point
		for _, p := range points {
			if math.IsNaN(p.Latitude) || math.IsNaN(p.Longitude) || math.Abs(p.Latitude) > 90 || math.Abs(p.Longitude) > 180 {
				return fmt.Errorf("GPX contains invalid coordinates")
			}
			line = append(line, types.Point{Lat: p.Latitude, Lon: p.Longitude})
			if date.IsZero() && !p.Timestamp.IsZero() {
				date = p.Timestamp
			}
		}
		if len(line) >= 2 {
			paths = append(paths, line)
		}
		return nil
	}
	for _, track := range data.Tracks {
		for _, segment := range track.Segments {
			if err := add(segment.Points); err != nil {
				return nil, time.Time{}, err
			}
		}
	}
	for _, route := range data.Routes {
		if err := add(route.Points); err != nil {
			return nil, time.Time{}, err
		}
	}
	if len(paths) == 0 {
		return nil, time.Time{}, fmt.Errorf("GPX contains no track or route segments. Isolated waypoints cannot prove trail completion")
	}
	if date.IsZero() {
		info, err := os.Stat(path)
		if err != nil {
			return nil, time.Time{}, err
		}
		date = info.ModTime()
	}
	return paths, date, nil
}
