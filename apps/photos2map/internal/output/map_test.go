package output

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/go-echarts/go-echarts/v2/opts"
)

// TestGenerateMap checks that the HTML map file is created correctly.
func TestGenerateMap(t *testing.T) {
	target := "out/map.html"
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(target) })

	gpsData := []opts.GeoData{
		{Name: "Image1", Value: []float64{-0.1276, 51.5074}},
		{Name: "Image2", Value: []float64{2.3522, 48.8566}},
	}

	if err := GenerateMap(gpsData); err != nil {
		t.Fatalf("GenerateMap() error = %v", err)
	}

	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"photos2map: GPS Image Map", "Image1", "-0.1276", "51.5074", "Image2", "2.3522", "48.8566"} {
		if !strings.Contains(string(contents), want) {
			t.Errorf("generated map does not contain %q", want)
		}
	}
}

func TestGenerateMapRejectsInvalidCoordinatesAndPreservesOutput(t *testing.T) {
	target := "out/map.html"
	original := []byte("existing map")
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(target) })

	err := GenerateMap([]opts.GeoData{{Name: "Invalid", Value: []float64{0, math.NaN()}}})
	if err == nil {
		t.Fatal("GenerateMap() error = nil, want invalid-coordinate error")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("output = %q, want preserved content %q", got, original)
	}
}
