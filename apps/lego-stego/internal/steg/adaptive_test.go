package steg

import (
	"image"
	"image/color"
	"testing"
)

func TestVarianceDetection(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 10, 10))

	// flat region
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetGray(x, y, color.Gray{Y: 100})
		}
	}

	v := computeVariance(img, 5, 5)
	if v != 0 {
		t.Fatalf("expected zero variance, got %f", v)
	}

	// add noise
	img.SetGray(5, 5, color.Gray{Y: 255})

	v = computeVariance(img, 5, 5)
	if v == 0 {
		t.Fatalf("expected non-zero variance")
	}
}

func TestMaskSelection(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 10, 10))

	// noisy center
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			val := uint8((x * y) % 255)
			img.SetGray(x, y, color.Gray{Y: val})
		}
	}

	mask := selectEmbeddingMask(img, 5.0)

	found := false
	for y := range mask {
		for x := range mask[y] {
			if mask[y][x] {
				found = true
				break
			}
		}
	}

	if !found {
		t.Fatalf("expected some pixels selected for embedding")
	}
}
