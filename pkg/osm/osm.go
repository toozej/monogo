package osm

import (
	"compress/gzip"
	"encoding/gob"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// OSMData contains the objects in one OSM XML export.
type OSMData struct {
	Nodes     map[int64]OSMNode
	Ways      map[int64]OSMWay
	Relations map[int64]OSMRelation
}

type OSMNode struct {
	ID   int64
	Lat  float64
	Lon  float64
	Tags map[string]string
}

type OSMWay struct {
	ID    int64
	Nodes []int64
	Tags  map[string]string
	BBox  [4]float64 // south, west, north, east
}

type OSMRelation struct {
	ID      int64
	Members []Member
	Tags    map[string]string
}

type Member struct {
	Type string `xml:"type,attr"`
	Ref  int64  `xml:"ref,attr"`
	Role string `xml:"role,attr"`
}

type Tag struct {
	Key   string `xml:"k,attr"`
	Value string `xml:"v,attr"`
}

// These XML structures remain available to callers of the shared package.
type OSM struct {
	XMLName xml.Name `xml:"osm"`
	Nodes   []Node   `xml:"node"`
	Ways    []Way    `xml:"way"`
}

type Node struct {
	ID   string  `xml:"id,attr"`
	Lat  float64 `xml:"lat,attr"`
	Lon  float64 `xml:"lon,attr"`
	Tags []Tag   `xml:"tag"`
}

type Way struct {
	ID       string    `xml:"id,attr"`
	NodeRefs []NodeRef `xml:"nd"`
	Tags     []Tag     `xml:"tag"`
}

type NodeRef struct {
	Ref string `xml:"ref,attr"`
}

// Trail stores a way candidate. WayIDs contains node references for compatibility.
type Trail struct {
	ID     int64
	Name   string
	Type   string
	WayIDs []int64
}

// Change this version when the parser or stored schema changes.
const cacheVersion = 4

type cache struct {
	Version int
	Size    int64
	MTime   int64
	Data    *OSMData
}

// LoadOSMData reads XML or gzip XML. It rejects caches from an older parser.
func LoadOSMData(path string, forceReload bool) (*OSMData, error) {
	return loadCached(path, ".bin", forceReload, loadOSMFile)
}

func loadCached(path, suffix string, forceReload bool, load func(string) (*OSMData, error)) (*OSMData, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !forceReload {
		if data, err := readCache(path+suffix, info); err == nil {
			return data, nil
		}
	}
	data, err := load(path)
	if err != nil {
		return nil, err
	}
	// A read-only input directory must not prevent use of the export.
	_ = writeCache(path+suffix, cache{cacheVersion, info.Size(), info.ModTime().UnixNano(), data})
	return data, nil
}

// IsTrailCandidate includes pedestrian geometry and explicitly marked hiking ways.
// A candidate can be a sidewalk, private path, or other feature requiring review.
func IsTrailCandidate(tags map[string]string) bool {
	switch tags["highway"] {
	case "path", "footway", "track", "steps", "cycleway", "pedestrian", "bridleway", "trail":
		return tags["area"] != "yes"
	}
	return tags["highway"] != "" && (tags["sac_scale"] != "" || tags["trail_visibility"] != "" ||
		tags["foot"] == "designated" || tags["RLIS:systemname"] != "" || tags["route"] == "hiking" || tags["route"] == "foot")
}

func IsWalkingRoute(tags map[string]string) bool {
	return (tags["type"] == "route" || tags["type"] == "superroute" || tags["type"] == "route_master") &&
		(tags["route"] == "hiking" || tags["route"] == "foot" || tags["route"] == "walking")
}

// LoadTrailData uses two streaming passes to retain only candidate geometry.
// It also retains every way referenced by a walking route, including road sections.
func LoadTrailData(path string, forceReload bool) (*OSMData, error) {
	return loadCached(path, ".trails.bin", forceReload, func(path string) (*OSMData, error) {
		data, err := decodeOSM(path, 1, nil)
		if err != nil {
			return nil, err
		}
		ways := map[int64]bool{}
		for id, way := range data.Ways {
			if IsTrailCandidate(way.Tags) {
				ways[id] = true
			}
		}
		visited := map[int64]bool{}
		var visit func(int64)
		visit = func(id int64) {
			if visited[id] {
				return
			}
			visited[id] = true
			for _, member := range data.Relations[id].Members {
				if member.Type == "way" {
					ways[member.Ref] = true
				}
				if member.Type == "relation" {
					visit(member.Ref)
				}
			}
		}
		for id, relation := range data.Relations {
			if IsWalkingRoute(relation.Tags) {
				visit(id)
			}
		}
		required := map[int64]bool{}
		for id, way := range data.Ways {
			if !ways[id] {
				delete(data.Ways, id)
				continue
			}
			for _, node := range way.Nodes {
				required[node] = true
			}
		}
		for id := range data.Relations {
			if !visited[id] {
				delete(data.Relations, id)
			}
		}
		nodes, err := decodeOSM(path, 2, required)
		if err != nil {
			return nil, err
		}
		data.Nodes = nodes.Nodes
		for id, way := range data.Ways {
			way.BBox = CalculateWayBBox(way, data.Nodes)
			data.Ways[id] = way
		}
		return data, nil
	})
}

func readCache(path string, info os.FileInfo) (*OSMData, error) {
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var c cache
	if err := gob.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	if c.Version != cacheVersion || c.Size != info.Size() || c.MTime != info.ModTime().UnixNano() || c.Data == nil {
		return nil, fmt.Errorf("OSM cache does not match the source or parser version")
	}
	return c.Data, nil
}

func writeCache(path string, c cache) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".osm-cache-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	encodeErr := gob.NewEncoder(f).Encode(c)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

func tagMap(tags []Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	result := make(map[string]string, len(tags))
	for _, tag := range tags {
		result[tag.Key] = tag.Value
	}
	return result
}

// loadOSMFile decodes one object at a time. Ways can precede their nodes.
func loadOSMFile(path string) (*OSMData, error) {
	return decodeOSM(path, 0, nil)
}

// mode 0 reads all objects. Mode 1 reads ways and relations. Mode 2 reads required nodes.
func decodeOSM(path string, mode int, required map[int64]bool) (*OSMData, error) {
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var reader io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gz.Close() }()
		reader = gz
	}
	d := xml.NewDecoder(reader)
	data := &OSMData{Nodes: map[int64]OSMNode{}, Ways: map[int64]OSMWay{}, Relations: map[int64]OSMRelation{}}
	rootSeen, rootClosed := false, false
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse OSM XML: %w", err)
		}
		if characters, ok := token.(xml.CharData); ok && (!rootSeen || rootClosed) && strings.TrimSpace(string(characters)) != "" {
			return nil, fmt.Errorf("unexpected text outside the osm document")
		}
		if end, ok := token.(xml.EndElement); ok && end.Name.Local == "osm" {
			rootClosed = true
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if !rootSeen {
			if start.Name.Local != "osm" {
				return nil, fmt.Errorf("expected an osm XML document, got %q", start.Name.Local)
			}
			rootSeen = true
			continue
		}
		if rootClosed {
			return nil, fmt.Errorf("unexpected element after the osm document")
		}
		switch start.Name.Local {
		case "node":
			if mode == 1 {
				if err := d.Skip(); err != nil {
					return nil, err
				}
				continue
			}
			if mode == 2 {
				var id int64
				for _, attribute := range start.Attr {
					if attribute.Name.Local == "id" {
						id, err = strconv.ParseInt(attribute.Value, 10, 64)
						if err != nil {
							return nil, err
						}
					}
				}
				if !required[id] {
					if err := d.Skip(); err != nil {
						return nil, err
					}
					continue
				}
			}
			var n struct {
				ID   int64    `xml:"id,attr"`
				Lat  *float64 `xml:"lat,attr"`
				Lon  *float64 `xml:"lon,attr"`
				Tags []Tag    `xml:"tag"`
			}
			if err := d.DecodeElement(&n, &start); err != nil {
				return nil, err
			}
			if n.ID == 0 || n.Lat == nil || n.Lon == nil || math.IsNaN(*n.Lat) || math.IsNaN(*n.Lon) || math.Abs(*n.Lat) > 90 || math.Abs(*n.Lon) > 180 {
				return nil, fmt.Errorf("invalid OSM node %d", n.ID)
			}
			if mode == 0 || required[n.ID] {
				data.Nodes[n.ID] = OSMNode{n.ID, *n.Lat, *n.Lon, tagMap(n.Tags)}
			}
		case "way":
			if mode == 2 {
				if err := d.Skip(); err != nil {
					return nil, err
				}
				continue
			}
			var w struct {
				ID   int64 `xml:"id,attr"`
				Refs []struct {
					Ref int64 `xml:"ref,attr"`
				} `xml:"nd"`
				Tags []Tag `xml:"tag"`
			}
			if err := d.DecodeElement(&w, &start); err != nil {
				return nil, err
			}
			if w.ID == 0 {
				return nil, fmt.Errorf("OSM way has no valid ID")
			}
			way := OSMWay{ID: w.ID, Tags: tagMap(w.Tags)}
			for _, ref := range w.Refs {
				if ref.Ref == 0 {
					return nil, fmt.Errorf("OSM way %d has an invalid node reference", w.ID)
				}
				way.Nodes = append(way.Nodes, ref.Ref)
			}
			data.Ways[w.ID] = way
		case "relation":
			if mode == 2 {
				if err := d.Skip(); err != nil {
					return nil, err
				}
				continue
			}
			var r struct {
				ID      int64    `xml:"id,attr"`
				Members []Member `xml:"member"`
				Tags    []Tag    `xml:"tag"`
			}
			if err := d.DecodeElement(&r, &start); err != nil {
				return nil, err
			}
			if r.ID == 0 {
				return nil, fmt.Errorf("OSM relation has no valid ID")
			}
			for _, member := range r.Members {
				if member.Ref == 0 || (member.Type != "node" && member.Type != "way" && member.Type != "relation") {
					return nil, fmt.Errorf("OSM relation %d has an invalid member", r.ID)
				}
			}
			data.Relations[r.ID] = OSMRelation{r.ID, r.Members, tagMap(r.Tags)}
		case "remark":
			var message string
			if err := d.DecodeElement(&message, &start); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("OSM server returned an incomplete export: %s", strings.TrimSpace(message))
		default:
			if err := d.Skip(); err != nil {
				return nil, err
			}
		}
	}
	if !rootSeen || !rootClosed {
		return nil, fmt.Errorf("OSM XML document is empty or incomplete")
	}
	if mode == 0 {
		for id, way := range data.Ways {
			way.BBox = CalculateWayBBox(way, data.Nodes)
			data.Ways[id] = way
		}
	}
	return data, nil
}

func CalculateWayBBox(way OSMWay, nodes map[int64]OSMNode) [4]float64 {
	box := [4]float64{90, 180, -90, -180}
	for _, id := range way.Nodes {
		if n, ok := nodes[id]; ok {
			box[0] = math.Min(box[0], n.Lat)
			box[1] = math.Min(box[1], n.Lon)
			box[2] = math.Max(box[2], n.Lat)
			box[3] = math.Max(box[3], n.Lon)
		}
	}
	return box
}

func Overlaps(a, b [4]float64) bool {
	return a[0] <= b[2] && a[2] >= b[0] && a[1] <= b[3] && a[3] >= b[1]
}
