package types

import "time"

// Trail represents information about a hiking trail
type Trail struct {
	Name           string
	Park           string
	Type           string
	Length         string
	URL            string
	Completed      bool
	CompletionDate string
}

// Point represents a geographical point
type Point struct {
	Lat float64
	Lon float64
}

// TrailMatch represents a potential match between GPX track and OSM trail
type TrailMatch struct {
	Name       string
	Type       string
	Length     float64
	Similarity float64
	OSMId      int64
}

// TrailResult stores the complete processing result for a GPX file
type TrailResult struct {
	Filename   string
	TravelDate time.Time
	Matches    []TrailMatch
}
