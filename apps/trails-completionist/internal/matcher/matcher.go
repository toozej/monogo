package matcher

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/toozej/monogo/apps/trails-completionist/internal/catalog"
	"github.com/toozej/monogo/apps/trails-completionist/internal/geo"
	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

// MatchTrails preserves source identities and completed trails absent from the input.
func MatchTrails(completed, raw []types.Trail) ([]types.Trail, error) {
	return MatchTrailsWithDistance(completed, raw, geo.DefaultDistanceMetres)
}

// MatchTrailsWithDistance merges completions by target ID and preserves source metadata.
// It omits additional OSM rows only when input geometry covers their entire sampled length.
func MatchTrailsWithDistance(completed, raw []types.Trail, distance float64) ([]types.Trail, error) {
	if math.IsNaN(distance) || math.IsInf(distance, 0) || distance <= 0 || distance > 1000 {
		return nil, fmt.Errorf("distance must be greater than zero and at most 1000 metres")
	}
	// Repeated activities for one target must not create repeated output rows.
	unique := make([]types.Trail, 0, len(completed))
	positions := map[string]int{}
	for _, trail := range completed {
		if !trail.Completed {
			continue
		}
		key := identity(trail)
		if i, ok := positions[key]; ok {
			unique[i] = withCompletion(unique[i], trail)
		} else {
			positions[key] = len(unique)
			unique = append(unique, trail)
		}
	}
	completed = unique
	byID := map[string]types.Trail{}
	byName := map[string][]types.Trail{}
	rawNames := map[string]int{}
	var inputGeometry [][]types.Point
	for _, trail := range completed {
		if trail.ID != "" {
			byID[trail.ID] = trail
		}
		key := catalog.Normalize(trail.Name)
		byName[key] = append(byName[key], trail)
	}
	for _, trail := range raw {
		rawNames[catalog.Normalize(trail.Name)]++
		if !trail.Incomplete {
			inputGeometry = append(inputGeometry, trail.Geometry...)
		}
	}
	used := map[string]bool{}
	var result []types.Trail
	for _, trail := range raw {
		match, ok := byID[trail.ID]
		// Compatibility for callers that supply trails without stable identities.
		if !ok && trail.ID == "" {
			key := catalog.Normalize(trail.Name)
			if options := byName[key]; rawNames[key] == 1 && len(options) == 1 && (trail.Park == options[0].Park || options[0].Park == "") {
				match, ok = options[0], true
			}
		}
		if ok {
			trail = withCompletion(trail, match)
			used[identity(match)] = true
		}
		result = append(result, trail)
	}
	var inputIndex *geo.Index
	for _, trail := range completed {
		if !used[identity(trail)] {
			if len(inputGeometry) > 0 && strings.HasPrefix(trail.ID, "osm/") {
				if inputIndex == nil {
					inputIndex = geo.NewIndex(inputGeometry)
				}
				// Input records receive completion from GPX coverage directly.
				// A completed OSM component must not promote a longer inventory trail.
				if geo.Coverage(trail.Geometry, inputIndex, distance)+1e-12 >= 1 {
					continue
				}
			}
			result = append(result, trail)
			used[identity(trail)] = true
		}
	}
	return result, nil
}

func withCompletion(target, completed types.Trail) types.Trail {
	if !completed.Completed {
		return target
	}
	if target.Completed {
		oldDate, oldErr := time.Parse("01/02/2006", target.CompletionDate)
		newDate, newErr := time.Parse("01/02/2006", completed.CompletionDate)
		if (oldErr == nil && (newErr != nil || !newDate.Before(oldDate))) || (oldErr != nil && newErr != nil && target.CompletionDate != "") {
			return target
		}
	}
	target.Completed = true
	target.CompletionDate = completed.CompletionDate
	return target
}

func identity(t types.Trail) string {
	if t.ID != "" {
		return t.ID
	}
	return t.Park + "/" + t.Name
}
