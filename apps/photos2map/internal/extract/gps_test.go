package extract

import (
	"path/filepath"
	"testing"
)

// TestExtractGPSData ensures GPS coordinates are correctly extracted from test images.
func TestExtractGPSData(t *testing.T) {
	// Create a test directory with test images
	// images from: https://github.com/ianare/exif-samples
	testDir := filepath.Join("..", "testdata")

	// Call the function
	gpsData := ExtractGPSData(testDir)

	// Assert GPS data is non-empty for valid test images
	if len(gpsData) == 0 {
		t.Fatalf("Expected GPS data, but got none")
	}

	// Example: Check if first entry contains valid GPS data
	coords, ok := asFloatSlice(gpsData[0].Value)
	if gpsData[0].Name == "" || !ok || len(coords) != 2 {
		t.Errorf("Invalid GPS data for first image: %+v", gpsData[0])
	}
}

// asFloatSlice asserts that v holds a []float64. Taking v as an explicitly typed
// any parameter keeps go-critic's per-file analysis from mis-flagging the type
// assertion as a no-op (it cannot otherwise resolve opts.GeoData.Value's type).
func asFloatSlice(v any) ([]float64, bool) {
	f, ok := v.([]float64)
	return f, ok
}
