package steg

import (
	"image"
)

// computeVariance calculates variance in a 3x3 window
func computeVariance(img image.Image, x, y int) float64 {
	var sum, sumSq float64
	count := 0

	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			px := x + dx
			py := y + dy

			r, g, b, _ := img.At(px, py).RGBA()
			gray := float64(((r>>8)&0xFE + (g>>8)&0xFE + (b>>8)&0xFE) / 3)

			sum += gray
			sumSq += gray * gray
			count++
		}
	}

	mean := sum / float64(count)
	return (sumSq / float64(count)) - (mean * mean)
}

// selectEmbeddingMask returns a boolean mask of “safe” pixels
func selectEmbeddingMask(img image.Image, threshold float64) [][]bool {
	b := img.Bounds()
	mask := make([][]bool, b.Dy())

	for y := 0; y < b.Dy(); y++ {
		mask[y] = make([]bool, b.Dx())
		for x := 0; x < b.Dx(); x++ {

			// skip borders
			if x == 0 || y == 0 || x == b.Dx()-1 || y == b.Dy()-1 {
				mask[y][x] = false
				continue
			}

			variance := computeVariance(img, x, y)
			if variance > threshold {
				mask[y][x] = true
			}
		}
	}
	return mask
}
