package parser

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/trails-completionist/internal/types"
	"github.com/toozej/trails-completionist/pkg/osm"

	"github.com/tkrajina/gpxgo/gpx"
)

// outputResults outputs the processing results in the specified format
// TODO trash OutputResults function
func OutputResults(results []types.TrailResult, format string) {
	switch format {
	case "csv":
		fmt.Println("Filename,TravelDate,TrailName,TrailType,Similarity,OSMID")
		for _, result := range results {
			for _, match := range result.Matches {
				fmt.Printf("%s,%s,%s,%s,%.1f,%.2f,%d\n",
					result.Filename,
					result.TravelDate.Format("2006-01-02"),
					match.Name,
					match.Type,
					match.Length,
					match.Similarity*100,
					match.OSMId)
			}
		}
	default: // text format
		for _, result := range results {
			fmt.Printf("\nFile: %s\n", result.Filename)
			fmt.Printf("Travel Date: %s\n", result.TravelDate.Format("January 2, 2006"))

			if len(result.Matches) == 0 {
				fmt.Println("No matches found")
			} else {
				fmt.Println("Trail matches:")
				for i, match := range result.Matches {
					fmt.Printf("  %d. %s (%.1f%% match, Type: %s, Length: %.1f, OSM ID: %d)\n",
						i+1, match.Name, match.Similarity*100, match.Type, match.Length, match.OSMId)
				}
			}
			fmt.Println(strings.Repeat("-", 40))
		}
	}
}

// ParseTrailsFromTrackFiles processes the provided track files and returns the found trails
func ParseTrailsFromTrackFiles(trackFiles string, recursive bool, osmData *osm.OSMData) ([]types.Trail, error) {
	foundTrailResults, err := processDirectory(trackFiles, recursive, osmData)
	if err != nil {
		return nil, fmt.Errorf("error processing track files: %w", err)
	}
	if len(foundTrailResults) == 0 {
		return nil, fmt.Errorf("no GPX files found in %s", trackFiles)
	}

	// convert from []types.TrailResult to []types.Trail
	trails, err := convertTrailResultsToTrails(foundTrailResults)
	if err != nil {
		return nil, fmt.Errorf("error converting trail results to trails: %w", err)
	}
	return trails, nil
}

// convertTrailResultsToTrails converts a slice of TrailResult to a slice of Trail
// This function assumes that the Trail is completed and sets the completion date.
func convertTrailResultsToTrails(results []types.TrailResult) ([]types.Trail, error) {
	var trails []types.Trail
	fmt.Printf("convertTrailResultsToTrails converting %d results into Trails\n", len(results))
	for _, result := range results {
		switch len(result.Matches) {
		case 0:
			fmt.Printf("No matches found for file %s\n", result.Filename)
		case 1:
			// Only one match, use it
			trail := types.Trail{
				Name:           result.Matches[0].Name,
				Park:           "",
				Type:           convertOSMTrailTypeToTrailType(result.Matches[0].Type),
				Length:         fmt.Sprintf("%.1f", result.Matches[0].Length),
				URL:            "",
				Completed:      true,
				CompletionDate: result.TravelDate.Format("01/02/2006"),
			}
			if log.GetLevel() == log.DebugLevel {
				fmt.Printf("Added trail %s from filename %s\n", trail.Name, result.Filename)
			}
			trails = append(trails, trail)
		default:
			// Multiple matches, use all of them
			for _, match := range result.Matches {
				// Check if the trail already exists in the list, and if so, skip it
				exists := false
				for _, t := range trails {
					if strings.EqualFold(t.Name, match.Name) {
						exists = true
						break
					}
				}
				if exists {
					continue
				}

				// Create a new trail from the match
				trail := types.Trail{
					Name:           match.Name,
					Park:           "",
					Type:           convertOSMTrailTypeToTrailType(match.Type),
					Length:         fmt.Sprintf("%.1f", match.Length),
					URL:            "",
					Completed:      true,
					CompletionDate: result.TravelDate.Format("01/02/2006"),
				}
				if log.GetLevel() == log.DebugLevel {
					fmt.Printf("Added trail %s from filename %s\n", trail.Name, result.Filename)
				}
				trails = append(trails, trail)

			}
		}
	}
	return trails, nil
}

// processDirectory processes all GPX files in a directory
func processDirectory(dirPath string, recursive bool, osmData *osm.OSMData) ([]types.TrailResult, error) {
	var results []types.TrailResult

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories if not recursive
		if info.IsDir() && !recursive && path != dirPath {
			return filepath.SkipDir
		}

		// Process only .gpx files
		if !info.IsDir() && filepath.Ext(path) == ".gpx" {
			fmt.Printf("Processing %s...\n", path)
			result, err := processGPXFile(path, osmData)
			if err != nil {
				fmt.Printf("  Warning: Could not process %s: %v\n", path, err)
				return nil // Continue with other files
			}
			results = append(results, result)
		}
		return nil
	}

	if err := filepath.Walk(dirPath, walkFn); err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return results, nil
}

// convertOSMTrailTypeToTrailType converts OSM trail types to a more user-friendly format
func convertOSMTrailTypeToTrailType(osmType string) string {
	switch osmType {
	case "path":
		return "Trail"
	case "footway":
		return "Trail"
	case "track":
		return "Trail"
	case "trail":
		return "Trail"
	default:
		return "Unknown"
	}
}

// calculateTrailLength calculates the total length of a trail in miles
func calculateTrailLength(points []types.Point) float64 {
	const earthRadiusMiles = 3958.8 // Earth's radius in miles

	totalDistance := 0.0
	for i := 1; i < len(points); i++ {
		lat1, lon1 := points[i-1].Lat, points[i-1].Lon
		lat2, lon2 := points[i].Lat, points[i].Lon

		// Convert degrees to radians
		lat1Rad := lat1 * math.Pi / 180
		lat2Rad := lat2 * math.Pi / 180
		lon1Rad := lon1 * math.Pi / 180
		lon2Rad := lon2 * math.Pi / 180

		dLat := lat2Rad - lat1Rad
		dLon := lon2Rad - lon1Rad

		a := math.Sin(dLat/2)*math.Sin(dLat/2) +
			math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
		c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

		// Distance between two points
		distance := earthRadiusMiles * c
		totalDistance += distance
	}

	// Round to the nearest tenth of a mile
	return math.Round(totalDistance*10) / 10
}

// processGPXFile processes a single GPX file
func processGPXFile(filePath string, osmData *osm.OSMData) (types.TrailResult, error) {
	result := types.TrailResult{
		Filename: filePath,
	}

	// Parse the GPX file
	gpxData, err := gpx.ParseFile(filePath)
	if err != nil {
		return result, fmt.Errorf("error parsing GPX file: %w", err)
	}

	// Extract track points
	var trackPoints []types.Point
	var trackTime time.Time

	// Try to get the travel date from the GPX data
	if len(gpxData.Tracks) > 0 && len(gpxData.Tracks[0].Segments) > 0 &&
		len(gpxData.Tracks[0].Segments[0].Points) > 0 {
		// Use the timestamp from the first point if available
		trackTime = gpxData.Tracks[0].Segments[0].Points[0].Timestamp
	}

	// If no timestamp in GPX, use file modification time
	if trackTime.IsZero() {
		fileInfo, err := os.Stat(filePath)
		if err == nil {
			trackTime = fileInfo.ModTime()
		} else {
			trackTime = time.Now() // Fallback to current time if all else fails
		}
	}

	result.TravelDate = trackTime

	// Collect all track points
	for _, track := range gpxData.Tracks {
		for _, segment := range track.Segments {
			for _, point := range segment.Points {
				trackPoints = append(trackPoints, types.Point{
					Lat: point.Latitude,
					Lon: point.Longitude,
				})
			}
		}
	}

	// If no points found, try waypoints
	if len(trackPoints) == 0 {
		for _, wpt := range gpxData.Waypoints {
			trackPoints = append(trackPoints, types.Point{
				Lat: wpt.Latitude,
				Lon: wpt.Longitude,
			})
		}
	}

	// If still no points, try routes
	if len(trackPoints) == 0 {
		for _, route := range gpxData.Routes {
			for _, point := range route.Points {
				trackPoints = append(trackPoints, types.Point{
					Lat: point.Latitude,
					Lon: point.Longitude,
				})
			}
		}
	}

	if len(trackPoints) == 0 {
		return result, fmt.Errorf("no GPS points found in file")
	}

	// Calculate bounding box with buffer
	bbox := calculateBoundingBox(trackPoints, 0.005) // ~500m buffer

	// Query for trails in the area
	trails, err := queryTrailsFromOSM(osmData, bbox)
	if err != nil {
		return result, fmt.Errorf("error querying OSM data: %w", err)
	}

	// Match trails
	matches, err := matchTrailsWithPoints(osmData, trackPoints, trails)
	if err != nil {
		return result, fmt.Errorf("error matching trails: %w", err)
	}

	result.Matches = matches
	return result, nil
}

// calculateBoundingBox determines the geographical bounds of the GPX track
func calculateBoundingBox(points []types.Point, buffer float64) [4]float64 {
	minLat, maxLat := 90.0, -90.0
	minLon, maxLon := 180.0, -180.0

	for _, point := range points {
		minLat = math.Min(minLat, point.Lat)
		maxLat = math.Max(maxLat, point.Lat)
		minLon = math.Min(minLon, point.Lon)
		maxLon = math.Max(maxLon, point.Lon)
	}

	return [4]float64{
		minLat - buffer,
		minLon - buffer,
		maxLat + buffer,
		maxLon + buffer,
	}
}

// queryTrailsFromOSM queries the OSM data for trails within the bounding box
func queryTrailsFromOSM(osmData *osm.OSMData, bbox [4]float64) ([]osm.Trail, error) {
	var trails []osm.Trail

	for id, way := range osmData.Ways {
		// Skip ways that don't overlap with our bounding box
		if !osm.Overlaps(way.BBox, bbox) {
			continue
		}

		// Check if this is a trail/path based on tags
		isTrail := false
		trailType := ""
		name := ""

		// Check highway tag for trail types
		if val, ok := way.Tags["highway"]; ok {
			if val == "path" || val == "footway" || val == "track" || val == "trail" {
				isTrail = true
				trailType = val
			}
		}

		// Get the name if available
		if val, ok := way.Tags["name"]; ok {
			name = val
		}

		// Only include named trails
		if isTrail && name != "" {
			trail := osm.Trail{
				ID:     id,
				Name:   name,
				Type:   trailType,
				WayIDs: way.Nodes,
			}
			trails = append(trails, trail)
		}
	}

	return trails, nil
}

// matchTrailsWithPoints matches GPX track points against OSM trails
func matchTrailsWithPoints(osmData *osm.OSMData, trackPoints []types.Point, trails []osm.Trail) ([]types.TrailMatch, error) {
	var matches []types.TrailMatch

	for _, trail := range trails {
		// Get all points for this trail's way
		var trailPoints []types.Point

		// For each node ID in the way, get its coordinates
		for _, nodeID := range trail.WayIDs {
			if node, exists := osmData.Nodes[nodeID]; exists {
				trailPoints = append(trailPoints, types.Point{
					Lat: node.Lat,
					Lon: node.Lon,
				})
			} else {
				// Skip nodes that aren't in our data
				continue
			}
		}

		if len(trailPoints) == 0 {
			// Skip trails with no points
			continue
		}

		// Calculate similarity between track and trail
		similarity := calculateSimilarity(trackPoints, trailPoints)

		// Only include trails with reasonable similarity
		if similarity > 0.5 {
			// Calculate trail length in miles
			trailLength := calculateTrailLength(trailPoints)

			matches = append(matches, types.TrailMatch{
				Name:       trail.Name,
				Type:       trail.Type,
				Length:     trailLength,
				Similarity: similarity,
				OSMId:      trail.ID,
			})
		}
	}

	// Sort matches by similarity (highest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Similarity > matches[j].Similarity
	})

	// Return top matches (up to 5)
	maxResults := 5
	if len(matches) < maxResults {
		maxResults = len(matches)
	}

	if maxResults == 0 {
		return []types.TrailMatch{}, nil
	}

	return matches[:maxResults], nil
}

// calculateSimilarity measures the similarity between two sets of points
func calculateSimilarity(track1, track2 []types.Point) float64 {
	// Sample points for efficient processing
	sampleSize := 20
	track1Sample := samplePoints(track1, sampleSize)
	track2Sample := samplePoints(track2, sampleSize)

	// Calculate Frechet distance (simplified implementation)
	totalDistance := 0.0
	matchCount := 0

	for _, p1 := range track1Sample {
		minDist := math.MaxFloat64
		for _, p2 := range track2Sample {
			dist := haversineDistance(p1.Lat, p1.Lon, p2.Lat, p2.Lon)
			if dist < minDist {
				minDist = dist
			}
		}
		totalDistance += minDist
		matchCount++
	}

	avgDistance := totalDistance / float64(matchCount)

	// Convert to similarity score (closer to 1 is better)
	// 100m is excellent match, 1000m is poor match
	similarity := math.Max(0, 1.0-(avgDistance/1.0)) // 1.0 km threshold

	return similarity
}

// samplePoints selects representative points from a track
func samplePoints(track []types.Point, count int) []types.Point {
	if len(track) <= count {
		return track
	}

	result := make([]types.Point, count)
	step := float64(len(track)) / float64(count)

	for i := 0; i < count; i++ {
		idx := int(math.Floor(float64(i) * step))
		if idx >= len(track) {
			idx = len(track) - 1
		}
		result[i] = track[idx]
	}

	return result
}

// haversineDistance calculates the distance between two coordinates in kilometers
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371.0 // km

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	lon1Rad := lon1 * math.Pi / 180
	lon2Rad := lon2 * math.Pi / 180

	dLat := lat2Rad - lat1Rad
	dLon := lon2Rad - lon1Rad

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	return distance
}
