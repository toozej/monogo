package steg

import (
	"image"
)

// generate all pixel coordinates
func pixelCoords(img image.Image) [][2]int {
	b := img.Bounds()
	var coords [][2]int

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			coords = append(coords, [2]int{x, y})
		}
	}
	return coords
}

// filter coords by adaptive mask (skip header region)
func filterCoords(coords [][2]int, mask [][]bool, skip int) [][2]int {
	var noisy, flat [][2]int

	for i, c := range coords {
		if i < skip {
			noisy = append(noisy, c)
			continue
		}

		if c[1] < len(mask) && c[0] < len(mask[c[1]]) && mask[c[1]][c[0]] {
			noisy = append(noisy, c)
		} else {
			flat = append(flat, c)
		}
	}

	return append(noisy, flat...)
}

// shuffle using password-seeded PRNG
func shuffleCoords(coords [][2]int, password string) {
	rng := seededPRNG(password)
	rng.Shuffle(len(coords), func(i, j int) {
		coords[i], coords[j] = coords[j], coords[i]
	})
}
