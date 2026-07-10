package output

import (
	"encoding/xml"
	"fmt"
	"math"

	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/twpayne/go-gpx"
)

// GenerateGPX creates a GPX file from the extracted GPS data.
// It takes a slice of GeoData and outputs a GPX file named `output.gpx`.
func GenerateGPX(gpsData []opts.GeoData) error {
	g := gpx.GPX{
		Version: "1.1",
		Creator: "photos2map",
		Wpt:     make([]*gpx.WptType, 0, len(gpsData)),
	}

	for _, data := range gpsData {
		// Type assert data.Value as []float64
		coords, ok := data.Value.([]float64)
		if !ok || len(coords) != 2 {
			return fmt.Errorf("invalid GPS coordinates for %q", data.Name)
		}

		lat, lon := coords[1], coords[0]
		if math.IsNaN(lat) || math.IsInf(lat, 0) || lat < -90 || lat > 90 ||
			math.IsNaN(lon) || math.IsInf(lon, 0) || lon < -180 || lon > 180 {
			return fmt.Errorf("GPS coordinates for %q are out of range", data.Name)
		}
		g.Wpt = append(g.Wpt, &gpx.WptType{
			Lat:  lat,
			Lon:  lon,
			Name: data.Name,
		})
	}

	// Marshal the GPX struct into indented XML
	gpxData, err := xml.MarshalIndent(g, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal GPX output: %w", err)
	}

	// Add the XML header and append the marshaled GPX data
	header := []byte(xml.Header)
	gpxData = append(header, gpxData...)

	file, commit, err := createAtomic("out/output.gpx")
	if err != nil {
		return fmt.Errorf("create GPX output: %w", err)
	}
	defer file.abort()

	if _, err := file.Write(gpxData); err != nil {
		return fmt.Errorf("write GPX output: %w", err)
	}
	if err := commit(); err != nil {
		return fmt.Errorf("commit GPX output: %w", err)
	}
	return nil
}
