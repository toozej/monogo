package parser

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/toozej/monogo/apps/trails-completionist/internal/geo"
	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

type geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

type feature struct {
	Type       string         `json:"type"`
	ID         any            `json:"id"`
	Properties map[string]any `json:"properties"`
	Geometry   *geometry      `json:"geometry"`
}

type collection struct {
	Type                  string          `json:"type"`
	Name                  string          `json:"name"`
	Features              []feature       `json:"features"`
	Source                map[string]any  `json:"source"`
	CRS                   json.RawMessage `json:"crs"`
	ExceededTransferLimit bool            `json:"exceededTransferLimit"`
	Properties            struct {
		ExceededTransferLimit bool `json:"exceededTransferLimit"`
	} `json:"properties"`
}

func readCollection(path string) (collection, error) {
	var c collection
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.Type != "FeatureCollection" || c.Features == nil {
		return c, fmt.Errorf("%s must contain a GeoJSON FeatureCollection", path)
	}
	if c.ExceededTransferLimit || c.Properties.ExceededTransferLimit {
		return c, fmt.Errorf("%s contains a truncated GIS response", path)
	}
	if len(c.CRS) > 0 && string(c.CRS) != "null" && !strings.Contains(string(c.CRS), "4326") && !strings.Contains(string(c.CRS), "CRS84") {
		return c, fmt.Errorf("%s must use WGS84 longitude and latitude (EPSG:4326)", path)
	}
	if count, ok := c.Source["feature_count"].(float64); ok && int(count) != len(c.Features) {
		return c, fmt.Errorf("%s has %d features but declares %.0f", path, len(c.Features), count)
	}
	return c, nil
}

func property(properties map[string]any, keys ...string) string {
	for _, key := range keys {
		for name, value := range properties {
			if strings.EqualFold(name, key) && value != nil {
				s := strings.TrimSpace(fmt.Sprint(value))
				if s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func points(coordinates [][]float64) ([]types.Point, error) {
	var result []types.Point
	if len(coordinates) < 2 {
		return nil, fmt.Errorf("a line or ring needs at least two coordinates")
	}
	for _, p := range coordinates {
		if len(p) < 2 || math.Abs(p[0]) > 180 || math.Abs(p[1]) > 90 {
			return nil, fmt.Errorf("invalid WGS84 coordinate %v", p)
		}
		result = append(result, types.Point{Lat: p[1], Lon: p[0]})
	}
	return result, nil
}

// ParseInventory reads PP&R, regional, or generic line GeoJSON without name deduplication.
func ParseInventory(path string, boundary geo.Boundary) ([]types.Trail, error) {
	c, err := readCollection(path)
	if err != nil {
		return nil, err
	}
	source := c.Name
	if url, ok := c.Source["url"].(string); ok {
		source = url
	}
	if source == "" {
		source = "GeoJSON inventory"
	}
	var trails []types.Trail
	seen := map[string]bool{}
	for i, f := range c.Features {
		if f.Type != "Feature" {
			return nil, fmt.Errorf("feature %d has no Feature type", i+1)
		}
		id := property(f.Properties, "GlobalID", "OBJECTID", "FID", "id")
		if id == "" && f.ID != nil {
			id = fmt.Sprint(f.ID)
		}
		if id == "" {
			return nil, fmt.Errorf("feature %d has no stable ID", i+1)
		}
		id = source + "/" + id
		if seen[id] {
			return nil, fmt.Errorf("duplicate inventory ID %q", id)
		}
		seen[id] = true
		name := property(f.Properties, "TrailName", "name")
		if name == "" {
			name = "Unnamed segment (" + strings.TrimPrefix(id, source+"/") + ")"
		}
		t := types.Trail{ID: id, Name: name, Park: property(f.Properties, "ParksProperty", "park", "SYSTEMNAME"), Type: "Trail", Source: source,
			Status: property(f.Properties, "TrailStatus", "status"), URL: property(f.Properties, "url", "website")}
		if sourceType := property(f.Properties, "TrailType", "type"); sourceType != "" {
			t.Type = sourceType
		}
		for _, key := range []string{"UserPedestrian", "HIKE"} {
			if value := property(f.Properties, key); value != "" {
				t.Status += ", " + key + "=" + value
			}
		}
		for _, key := range []string{"SHAREDNAME", "alt_name", "official_name"} {
			if alias := property(f.Properties, key); alias != "" {
				t.Aliases = append(t.Aliases, strings.Split(alias, ";")...)
			}
		}
		if f.Geometry == nil {
			t.Incomplete = true
		} else {
			var lines [][][]float64
			switch f.Geometry.Type {
			case "LineString":
				var line [][]float64
				err = json.Unmarshal(f.Geometry.Coordinates, &line)
				lines = append(lines, line)
			case "MultiLineString":
				err = json.Unmarshal(f.Geometry.Coordinates, &lines)
			default:
				return nil, fmt.Errorf("feature %s has unsupported geometry %q", id, f.Geometry.Type)
			}
			if err != nil || len(lines) == 0 {
				return nil, fmt.Errorf("feature %s has invalid line coordinates", id)
			}
			for _, line := range lines {
				p, err := points(line)
				if err != nil {
					return nil, fmt.Errorf("feature %s: %w", id, err)
				}
				t.Geometry = append(t.Geometry, p)
			}
			t.Geometry = boundary.Clip(t.Geometry)
			if len(t.Geometry) == 0 {
				continue
			}
		}
		t.Length = fmt.Sprintf("%.3f", geo.Length(t.Geometry)/1609.344)
		trails = append(trails, t)
	}
	return trails, nil
}

func ParseBoundary(path string) (geo.Boundary, error) {
	if path == "" {
		return geo.Boundary{}, nil
	}
	c, err := readCollection(path)
	if err != nil {
		return geo.Boundary{}, err
	}
	var polygons [][][]types.Point
	for _, f := range c.Features {
		if f.Geometry == nil {
			return geo.Boundary{}, fmt.Errorf("boundary feature has no geometry")
		}
		var coordinates [][][][]float64
		switch f.Geometry.Type {
		case "Polygon":
			var polygon [][][]float64
			err = json.Unmarshal(f.Geometry.Coordinates, &polygon)
			coordinates = append(coordinates, polygon)
		case "MultiPolygon":
			err = json.Unmarshal(f.Geometry.Coordinates, &coordinates)
		default:
			return geo.Boundary{}, fmt.Errorf("boundary geometry must be Polygon or MultiPolygon")
		}
		if err != nil {
			return geo.Boundary{}, err
		}
		for _, polygon := range coordinates {
			var rings [][]types.Point
			for _, ring := range polygon {
				p, err := points(ring)
				if err != nil || len(p) < 4 || p[0] != p[len(p)-1] {
					return geo.Boundary{}, fmt.Errorf("boundary ring must contain at least four coordinates and be closed")
				}
				rings = append(rings, p)
			}
			if len(rings) == 0 {
				return geo.Boundary{}, fmt.Errorf("boundary polygon has no rings")
			}
			polygons = append(polygons, rings)
		}
	}
	if len(polygons) == 0 {
		return geo.Boundary{}, fmt.Errorf("boundary contains no polygons")
	}
	return geo.NewBoundary(polygons), nil
}
