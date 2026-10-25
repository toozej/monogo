// Package catalog enumerates trail candidates without a GPX or name requirement.
package catalog

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"github.com/toozej/monogo/apps/trails-completionist/internal/geo"
	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
	"github.com/toozej/monogo/pkg/osm"
)

type Diagnostics struct {
	MissingNodes     int `json:"missing_nodes"`
	MissingMembers   int `json:"missing_members"`
	RelationCycles   int `json:"relation_cycles"`
	IncompleteTrails int `json:"incomplete_trails"`
	UnsupportedRoles int `json:"unsupported_roles"`
}

// Normalize changes case and punctuation. It does not apply fuzzy name matching.
func Normalize(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

func names(tags map[string]string) (string, []string) {
	name := tags["name"]
	var aliases []string
	for _, key := range []string{"name", "official_name", "alt_name", "short_name", "loc_name", "name:en", "ref"} {
		for _, value := range strings.Split(tags[key], ";") {
			if value = strings.TrimSpace(value); value != "" {
				if name == "" {
					name = value
				}
				aliases = append(aliases, value)
			}
		}
	}
	return name, aliases
}

func IsCandidate(tags map[string]string) bool {
	return osm.IsTrailCandidate(tags)
}

func route(tags map[string]string) bool {
	return osm.IsWalkingRoute(tags)
}

// Build preserves route identities and separates disconnected ways with equal names.
// Missing references split geometry. They never create a shortcut across the gap.
func Build(data *osm.OSMData, boundary geo.Boundary) ([]types.Trail, Diagnostics) {
	var diagnostics Diagnostics
	missingNodes, missingMembers := map[int64]bool{}, map[string]bool{}
	wayGeometry := func(way osm.OSMWay) ([][]types.Point, bool) {
		var paths [][]types.Point
		var path []types.Point
		incomplete := false
		flush := func() {
			if len(path) >= 2 {
				paths = append(paths, path)
			}
			path = nil
		}
		for _, id := range way.Nodes {
			if node, ok := data.Nodes[id]; ok {
				path = append(path, types.Point{Lat: node.Lat, Lon: node.Lon})
			} else {
				missingNodes[id] = true
				incomplete = true
				flush()
			}
		}
		flush()
		return paths, incomplete || len(paths) == 0
	}
	var trails []types.Trail
	covered := map[int64]map[string]bool{}
	relationIDs := make([]int64, 0, len(data.Relations))
	for id, r := range data.Relations {
		if route(r.Tags) {
			relationIDs = append(relationIDs, id)
		}
	}
	sort.Slice(relationIDs, func(i, j int) bool { return relationIDs[i] < relationIDs[j] })
	for _, id := range relationIDs {
		r := data.Relations[id]
		name, aliases := names(r.Tags)
		if name == "" {
			name = fmt.Sprintf("Unnamed route (OSM relation %d)", id)
		}
		type part struct {
			trail types.Trail
			ways  map[int64]bool
		}
		parts := map[string]*part{}
		getPart := func(role string) *part {
			if parts[role] == nil {
				t := types.Trail{ID: fmt.Sprintf("osm/relation/%d", id), Name: name, Aliases: aliases, Type: "Trail", Source: "OpenStreetMap", Status: accessStatus(r.Tags)}
				if role != "" {
					t.ID += "/" + url.PathEscape(role)
					t.Name += " (" + role + ")"
					t.Aliases = nil
					for _, alias := range aliases {
						t.Aliases = append(t.Aliases, alias+" ("+role+")")
					}
					t.Status += ", role=" + role
				}
				parts[role] = &part{trail: t, ways: map[int64]bool{}}
			}
			return parts[role]
		}
		getPart("")
		type relationPart struct {
			id   int64
			role string
		}
		seenRelations := map[relationPart]bool{}
		active := map[int64]bool{}
		var visit func(int64, string)
		visit = func(ref int64, role string) {
			current := getPart(role)
			if active[ref] {
				diagnostics.RelationCycles++
				current.trail.Incomplete = true
				return
			}
			key := relationPart{ref, role}
			if seenRelations[key] {
				return
			}
			seenRelations[key], active[ref] = true, true
			defer delete(active, ref)
			relation, ok := data.Relations[ref]
			if !ok {
				missingMembers[fmt.Sprintf("relation/%d", ref)] = true
				current.trail.Incomplete = true
				return
			}
			for _, member := range relation.Members {
				if member.Type != "way" && member.Type != "relation" {
					continue
				}
				memberRole := role
				supported := true
				switch member.Role {
				case "", "main", "forward", "backward":
				case "guidepost", "route_marker", "parking":
					continue
				case "alternative", "approach", "excursion", "connection":
					if memberRole == "" {
						memberRole = member.Role
					} else if memberRole != member.Role {
						memberRole += "/" + member.Role
					}
				default:
					supported = false
					diagnostics.UnsupportedRoles++
					if memberRole != "" {
						memberRole += "/"
					}
					memberRole += "unknown:" + member.Role
				}
				target := getPart(memberRole)
				if !supported {
					target.trail.Incomplete = true
				}
				if member.Type == "relation" {
					visit(member.Ref, memberRole)
					continue
				}
				if target.ways[member.Ref] {
					continue
				}
				target.ways[member.Ref] = true
				way, ok := data.Ways[member.Ref]
				if !ok {
					missingMembers[fmt.Sprintf("way/%d", member.Ref)] = true
					target.trail.Incomplete = true
					continue
				}
				paths, incomplete := wayGeometry(way)
				target.trail.Incomplete = target.trail.Incomplete || incomplete
				target.trail.Geometry = append(target.trail.Geometry, paths...)
				target.trail.OSMIDs = append(target.trail.OSMIDs, fmt.Sprintf("way/%d", member.Ref))
			}
		}
		visit(id, "")
		for _, part := range parts {
			t := part.trail
			if len(part.ways) == 0 {
				t.Incomplete = true
			}
			t.Geometry = boundary.Clip(t.Geometry)
			if len(t.Geometry) == 0 && !t.Incomplete {
				continue
			}
			for wayID := range part.ways {
				if covered[wayID] == nil {
					covered[wayID] = map[string]bool{}
				}
				covered[wayID][Normalize(t.Name)] = true
			}
			t.OSMIDs = append(t.OSMIDs, fmt.Sprintf("relation/%d", id))
			sort.Strings(t.OSMIDs)
			t.Length = fmt.Sprintf("%.3f", geo.Length(t.Geometry)/1609.344)
			trails = append(trails, t)
		}
	}

	// Union ways by shared nodes and primary name in near-linear time.
	wayIDs := make([]int64, 0)
	for id, way := range data.Ways {
		name, _ := names(way.Tags)
		if IsCandidate(way.Tags) && !covered[id][Normalize(name)] {
			wayIDs = append(wayIDs, id)
		}
	}
	sort.Slice(wayIDs, func(i, j int) bool { return wayIDs[i] < wayIDs[j] })
	parent := map[int64]int64{}
	var root func(int64) int64
	root = func(id int64) int64 {
		if parent[id] != id {
			parent[id] = root(parent[id])
		}
		return parent[id]
	}
	type nodeName struct {
		node int64
		name string
	}
	owners := map[nodeName]int64{}
	for _, id := range wayIDs {
		parent[id] = id
		way := data.Ways[id]
		name, _ := names(way.Tags)
		if name == "" {
			continue
		}
		for _, node := range way.Nodes {
			key := nodeName{node, Normalize(name)}
			if owner, ok := owners[key]; ok {
				a, b := root(id), root(owner)
				parent[max(a, b)] = min(a, b)
			} else {
				owners[key] = id
			}
		}
	}
	groups := map[int64]*types.Trail{}
	for _, id := range wayIDs {
		way := data.Ways[id]
		name, aliases := names(way.Tags)
		if name == "" {
			name = fmt.Sprintf("Unnamed %s (OSM way %d)", way.Tags["highway"], id)
		}
		groupID := root(id)
		t := groups[groupID]
		if t == nil {
			t = &types.Trail{ID: fmt.Sprintf("osm/way/%d", groupID), Name: name, Type: "Trail", Source: "OpenStreetMap", Status: accessStatus(way.Tags)}
			groups[groupID] = t
		}
		paths, incomplete := wayGeometry(way)
		t.Incomplete = t.Incomplete || incomplete
		t.Geometry = append(t.Geometry, paths...)
		t.Aliases = append(t.Aliases, aliases...)
		t.OSMIDs = append(t.OSMIDs, fmt.Sprintf("way/%d", id))
	}
	for _, t := range groups {
		t.Geometry = boundary.Clip(t.Geometry)
		if len(t.Geometry) == 0 && !t.Incomplete {
			continue
		}
		t.Length = fmt.Sprintf("%.3f", geo.Length(t.Geometry)/1609.344)
		trails = append(trails, *t)
	}
	for _, t := range trails {
		if t.Incomplete {
			diagnostics.IncompleteTrails++
		}
	}
	diagnostics.MissingNodes, diagnostics.MissingMembers = len(missingNodes), len(missingMembers)
	sort.Slice(trails, func(i, j int) bool { return trails[i].ID < trails[j].ID })
	return trails, diagnostics
}

func accessStatus(tags map[string]string) string {
	var values []string
	for _, key := range []string{"highway", "access", "foot", "informal", "construction", "proposed"} {
		if tags[key] != "" {
			values = append(values, key+"="+tags[key])
		}
	}
	return strings.Join(values, ", ")
}
