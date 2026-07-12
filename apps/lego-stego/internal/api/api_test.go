package api_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/toozej/monogo/apps/lego-stego/internal/api"
)

func TestEmbedFileSupportsInPlaceOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "carrier.png")
	img := image.NewRGBA(image.Rect(0, 0, 120, 120))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * y), G: uint8(x + y), B: uint8(x*3 + y), A: 255})
		}
	}
	var carrier bytes.Buffer
	if err := png.Encode(&carrier, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, carrier.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	want := []byte("in-place payload")
	if err := api.EmbedFile(path, path, want, "password"); err != nil {
		t.Fatalf("EmbedFile() in place failed: %v", err)
	}
	got, err := api.ExtractFile(path, "password")
	if err != nil {
		t.Fatalf("ExtractFile() failed: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ExtractFile() = %q, want %q", got, want)
	}
}
