// Package extract provides functions for extracting GPS data from images.
package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/toozej/monogo/apps/photos2map/internal/exif"
)

// ExtractGPSData reads all the images in a given directory and returns a slice of GeoData containing GPS coordinates.
// Supported formats include JPG, PNG, RAW, DNG, and HEIF.
func ExtractGPSData(dir string) ([]opts.GeoData, error) {
	var gpsData []opts.GeoData

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		base := filepath.Base(path)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".jpg", ".jpeg", ".png":
			lat, lon, err := exif.ExtractEXIF(path)
			if err == nil {
				gpsData = append(gpsData, opts.GeoData{Name: name, Value: []float64{lon, lat}})
			}
			// TODO re-enable extracting EXIF data from raw, dng, and heif file types once those libraries work
			// case ".dng", ".raw":
			// 	lat, lon, err := exif.ExtractRawEXIF(path)
			// 	if err == nil {
			// 		gpsData = append(gpsData, opts.GeoData{Name: name, Value: []float64{lon, lat}})
			// 	}
			// case ".heif":
			// 	lat, lon, err := exif.ExtractHEIFEXIF(path)
			// 	if err == nil {
			// 		gpsData = append(gpsData, opts.GeoData{Name: name, Value: []float64{lon, lat}})
			// 	}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk image directory %q: %w", dir, err)
	}

	return gpsData, nil
}
