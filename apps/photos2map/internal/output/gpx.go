package output

import (
	"encoding/xml"
	"fmt"

	"github.com/go-echarts/go-echarts/v2/opts"
	log "github.com/sirupsen/logrus"
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
		lon, lat, err := coordinates(data)
		if err != nil {
			return err
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
	log.Println("GPX file generated successfully.")
	return nil
}
