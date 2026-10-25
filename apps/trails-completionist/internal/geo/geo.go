// Package geo provides distance, coverage, and polygon operations for local trail data.
package geo

import (
	"math"
	"sort"

	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

type Point = types.Point

const earthRadius = 6371008.8
const cellSize = 0.001 // degrees, approximately 78 by 111 metres in Portland

const DefaultDistanceMetres = 25.0

func Distance(a, b Point) float64 {
	lat1, lat2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dlat, dlon := lat2-lat1, (b.Lon-a.Lon)*math.Pi/180
	h := math.Pow(math.Sin(dlat/2), 2) + math.Cos(lat1)*math.Cos(lat2)*math.Pow(math.Sin(dlon/2), 2)
	return 2 * earthRadius * math.Asin(math.Sqrt(math.Min(1, h)))
}

func Length(paths [][]Point) float64 {
	var length float64
	for _, path := range paths {
		for i := 1; i < len(path); i++ {
			length += Distance(path[i-1], path[i])
		}
	}
	return length
}

type segment struct{ a, b Point }
type cell struct{ lat, lon int }

// Index stores segment bounds. It does not replace a line with isolated vertices.
type Index struct {
	cells map[cell][]segment
	long  []segment
}

func key(p Point) cell {
	return cell{int(math.Floor(p.Lat / cellSize)), int(math.Floor(p.Lon / cellSize))}
}

func NewIndex(paths [][]Point) *Index {
	idx := &Index{cells: map[cell][]segment{}}
	for _, path := range paths {
		for i := 1; i < len(path); i++ {
			s := segment{path[i-1], path[i]}
			a, b := key(s.a), key(s.b)
			minLat, maxLat := min(a.lat, b.lat), max(a.lat, b.lat)
			minLon, maxLon := min(a.lon, b.lon), max(a.lon, b.lon)
			if (maxLat-minLat+1)*(maxLon-minLon+1) > 10000 {
				idx.long = append(idx.long, s)
				continue
			}
			for lat := minLat; lat <= maxLat; lat++ {
				for lon := minLon; lon <= maxLon; lon++ {
					c := cell{lat, lon}
					idx.cells[c] = append(idx.cells[c], s)
				}
			}
		}
	}
	return idx
}

func segmentDistance(p Point, s segment) float64 {
	// A local equirectangular projection preserves metre-scale distances in this city.
	xscale := math.Cos(p.Lat*math.Pi/180) * math.Pi / 180 * earthRadius
	yscale := math.Pi / 180 * earthRadius
	ax, ay := (s.a.Lon-p.Lon)*xscale, (s.a.Lat-p.Lat)*yscale
	bx, by := (s.b.Lon-p.Lon)*xscale, (s.b.Lat-p.Lat)*yscale
	dx, dy := bx-ax, by-ay
	t := 0.0
	if denominator := dx*dx + dy*dy; denominator > 0 {
		t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/denominator))
	}
	return math.Hypot(ax+t*dx, ay+t*dy)
}

func (idx *Index) Near(p Point, metres float64) bool {
	dlat := metres / earthRadius * 180 / math.Pi
	dlon := dlat / math.Max(0.001, math.Cos(p.Lat*math.Pi/180))
	a, b := key(Point{Lat: p.Lat - dlat, Lon: p.Lon - dlon}), key(Point{Lat: p.Lat + dlat, Lon: p.Lon + dlon})
	for lat := a.lat; lat <= b.lat; lat++ {
		for lon := a.lon; lon <= b.lon; lon++ {
			for _, s := range idx.cells[cell{lat, lon}] {
				if segmentDistance(p, s) <= metres {
					return true
				}
			}
		}
	}
	for _, s := range idx.long {
		if segmentDistance(p, s) <= metres {
			return true
		}
	}
	return false
}

// Coverage samples every target segment at intervals of at most ten metres.
// The result is a length-weighted fraction within the specified distance.
func Coverage(paths [][]Point, reference *Index, metres float64) float64 {
	var covered, total float64
	for _, path := range paths {
		for i := 1; i < len(path); i++ {
			a, b := path[i-1], path[i]
			length := Distance(a, b)
			n := max(1, int(math.Ceil(length/10)))
			for j := range n {
				t := (float64(j) + 0.5) / float64(n)
				p := Point{Lat: a.Lat + t*(b.Lat-a.Lat), Lon: a.Lon + t*(b.Lon-a.Lon)}
				if reference.Near(p, metres) {
					covered += length / float64(n)
				}
			}
			total += length
		}
	}
	if total == 0 {
		return 0
	}
	return covered / total
}

// Boundary stores polygon rings and indexes their edges for repeated line clipping.
type Boundary struct {
	polygons [][]ring
	edges    *Index
	box      [4]float64
}

type ring struct {
	bands map[int][]segment
	box   [4]float64
}

func NewBoundary(polygons [][][]Point) Boundary {
	b := Boundary{box: [4]float64{90, 180, -90, -180}}
	var paths [][]Point
	for _, polygon := range polygons {
		var rings []ring
		for _, points := range polygon {
			r := ring{bands: map[int][]segment{}, box: [4]float64{90, 180, -90, -180}}
			var closed []Point
			closed = append(closed, points...)
			if len(points) > 0 && points[0] != points[len(points)-1] {
				closed = append(closed, points[0])
			}
			for i := 1; i < len(closed); i++ {
				a, c := closed[i-1], closed[i]
				r.box[0] = math.Min(r.box[0], a.Lat)
				r.box[1] = math.Min(r.box[1], a.Lon)
				r.box[2] = math.Max(r.box[2], a.Lat)
				r.box[3] = math.Max(r.box[3], a.Lon)
				for band := key(Point{Lat: math.Min(a.Lat, c.Lat)}).lat; band <= key(Point{Lat: math.Max(a.Lat, c.Lat)}).lat; band++ {
					r.bands[band] = append(r.bands[band], segment{a, c})
				}
			}
			b.box[0] = math.Min(b.box[0], r.box[0])
			b.box[1] = math.Min(b.box[1], r.box[1])
			b.box[2] = math.Max(b.box[2], r.box[2])
			b.box[3] = math.Max(b.box[3], r.box[3])
			paths = append(paths, closed)
			rings = append(rings, r)
		}
		b.polygons = append(b.polygons, rings)
	}
	b.edges = NewIndex(paths)
	return b
}

func (b Boundary) Empty() bool { return len(b.polygons) == 0 }

func cross(a, b Point) float64  { return a.Lon*b.Lat - a.Lat*b.Lon }
func subtract(a, b Point) Point { return Point{Lat: a.Lat - b.Lat, Lon: a.Lon - b.Lon} }

func (r ring) contains(p Point) bool {
	if p.Lat < r.box[0] || p.Lat > r.box[2] || p.Lon < r.box[1] || p.Lon > r.box[3] {
		return false
	}
	inside := false
	for _, s := range r.bands[key(p).lat] {
		a, b := s.a, s.b
		if math.Abs(cross(subtract(b, a), subtract(p, a))) < 1e-12 && p.Lat >= math.Min(a.Lat, b.Lat)-1e-12 && p.Lat <= math.Max(a.Lat, b.Lat)+1e-12 && p.Lon >= math.Min(a.Lon, b.Lon)-1e-12 && p.Lon <= math.Max(a.Lon, b.Lon)+1e-12 {
			return true
		}
		if (a.Lat > p.Lat) != (b.Lat > p.Lat) && p.Lon < (b.Lon-a.Lon)*(p.Lat-a.Lat)/(b.Lat-a.Lat)+a.Lon {
			inside = !inside
		}
	}
	return inside
}

func (boundary Boundary) Contains(p Point) bool {
	for _, polygon := range boundary.polygons {
		if len(polygon) == 0 || !polygon[0].contains(p) {
			continue
		}
		inHole := false
		for _, hole := range polygon[1:] {
			inHole = inHole || hole.contains(p)
		}
		if !inHole {
			return true
		}
	}
	return false
}

func (boundary Boundary) intersectionEdges(a, b Point) []segment {
	first := key(Point{Lat: math.Max(math.Min(a.Lat, b.Lat), boundary.box[0]), Lon: math.Max(math.Min(a.Lon, b.Lon), boundary.box[1])})
	last := key(Point{Lat: math.Min(math.Max(a.Lat, b.Lat), boundary.box[2]), Lon: math.Min(math.Max(a.Lon, b.Lon), boundary.box[3])})
	seen := map[segment]bool{}
	for lat := min(first.lat, last.lat); lat <= max(first.lat, last.lat); lat++ {
		for lon := min(first.lon, last.lon); lon <= max(first.lon, last.lon); lon++ {
			for _, s := range boundary.edges.cells[cell{lat, lon}] {
				seen[s] = true
			}
		}
	}
	for _, s := range boundary.edges.long {
		seen[s] = true
	}
	edges := make([]segment, 0, len(seen))
	for s := range seen {
		edges = append(edges, s)
	}
	return edges
}

// Clip retains line portions inside a polygon, including lines that cross its border.
func (boundary Boundary) Clip(paths [][]Point) [][]Point {
	if boundary.Empty() {
		return paths
	}
	var result [][]Point
	for _, path := range paths {
		var current []Point
		flush := func() {
			if len(current) >= 2 {
				result = append(result, current)
			}
			current = nil
		}
		for i := 1; i < len(path); i++ {
			a, b := path[i-1], path[i]
			if math.Max(a.Lat, b.Lat) < boundary.box[0] || math.Min(a.Lat, b.Lat) > boundary.box[2] || math.Max(a.Lon, b.Lon) < boundary.box[1] || math.Min(a.Lon, b.Lon) > boundary.box[3] {
				flush()
				continue
			}
			r := subtract(b, a)
			ts := []float64{0, 1}
			for _, edge := range boundary.intersectionEdges(a, b) {
				c, d := edge.a, edge.b
				s := subtract(d, c)
				denom := cross(r, s)
				if math.Abs(denom) < 1e-15 {
					continue
				}
				t, u := cross(subtract(c, a), s)/denom, cross(subtract(c, a), r)/denom
				if t > 0 && t < 1 && u >= 0 && u <= 1 {
					ts = append(ts, t)
				}
			}
			sort.Float64s(ts)
			at := func(t float64) Point { return Point{Lat: a.Lat + t*r.Lat, Lon: a.Lon + t*r.Lon} }
			for j := 1; j < len(ts); j++ {
				if ts[j]-ts[j-1] < 1e-12 {
					continue
				}
				if boundary.Contains(at((ts[j] + ts[j-1]) / 2)) {
					start, end := at(ts[j-1]), at(ts[j])
					if len(current) == 0 {
						current = append(current, start)
					} else if Distance(current[len(current)-1], start) > 0.01 {
						flush()
						current = append(current, start)
					}
					current = append(current, end)
				} else {
					flush()
				}
			}
		}
		flush()
	}
	return result
}
