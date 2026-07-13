package output

import (
	"encoding/xml"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/twpayne/go-gpx"
)

// TestGenerateGPX checks if a valid GPX file is generated.
func TestGenerateGPX(t *testing.T) {
	target := "out/output.gpx"
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(target) })

	gpsData := []opts.GeoData{
		{Name: "Image1", Value: []float64{-0.1276, 51.5074}},
		{Name: "Image2", Value: []float64{2.3522, 48.8566}},
		{Name: "PositiveBoundary", Value: []float64{180, 90}},
		{Name: "NegativeBoundary", Value: []float64{-180, -90}},
	}

	if err := GenerateGPX(gpsData); err != nil {
		t.Fatalf("GenerateGPX() error = %v", err)
	}

	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(contents), xml.Header) {
		t.Fatal("GPX output is missing the XML header")
	}
	var document gpx.GPX
	if err := xml.Unmarshal(contents, &document); err != nil {
		t.Fatalf("unmarshal generated GPX: %v", err)
	}
	if got, want := len(document.Wpt), len(gpsData); got != want {
		t.Fatalf("waypoint count = %d, want %d", got, want)
	}
	for i, want := range gpsData {
		coords := want.Value.([]float64)
		got := document.Wpt[i]
		if got.Name != want.Name || got.Lon != coords[0] || got.Lat != coords[1] {
			t.Errorf("waypoint %d = %+v, want name=%q lon=%v lat=%v", i, got, want.Name, coords[0], coords[1])
		}
	}
}

func TestGenerateGPXRejectsInvalidCoordinates(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{name: "wrong type", value: "0,0"},
		{name: "too few values", value: []float64{0}},
		{name: "too many values", value: []float64{0, 0, 0}},
		{name: "longitude too low", value: []float64{-181, 0}},
		{name: "longitude too high", value: []float64{181, 0}},
		{name: "latitude too low", value: []float64{0, -91}},
		{name: "latitude too high", value: []float64{0, 91}},
		{name: "longitude NaN", value: []float64{math.NaN(), 0}},
		{name: "latitude infinity", value: []float64{0, math.Inf(1)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := GenerateGPX([]opts.GeoData{{Name: "Invalid", Value: tt.value}}); err == nil {
				t.Fatal("GenerateGPX() error = nil, want invalid-coordinate error")
			}
		})
	}
}

func TestGenerateGPXValidationFailurePreservesExistingOutput(t *testing.T) {
	target := "out/output.gpx"
	original := []byte("existing GPX")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(target) })

	err := GenerateGPX([]opts.GeoData{{Name: "Invalid", Value: []float64{181, 0}}})
	if err == nil {
		t.Fatal("GenerateGPX() error = nil, want invalid-coordinate error")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("output = %q, want preserved content %q", got, original)
	}
}
