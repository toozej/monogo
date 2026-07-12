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

		if len(mask) > 0 {
			minX, minY := coords[0][0], coords[0][1]
			maskX, maskY := c[0]-minX, c[1]-minY
			if maskY >= 0 && maskY < len(mask) && maskX >= 0 && maskX < len(mask[maskY]) && mask[maskY][maskX] {
				noisy = append(noisy, c)
				continue
			}
		}
		flat = append(flat, c)
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
