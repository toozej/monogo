package steg

import (
	"fmt"
	"image"
)

func Embed(img image.Image, data []byte, password string) (*image.RGBA, error) {
	writer := NewBitWriter(img)

	payload, err := BuildPayload(data, password, uint8(writer.nch)) // #nosec G115 -- nch is 1 or 3, always fits in uint8
	if err != nil {
		return nil, err
	}

	bits := bytesToBits(payload)

	bitCap := writer.Capacity()

	if bitCap < len(bits) {
		return nil, fmt.Errorf("insufficient capacity: need %d bits, have %d", len(bits), bitCap)
	}

	coords := pixelCoords(img)
	mask := selectEmbeddingMask(img, 20.0)

	coords = filterCoords(coords, mask, 32)
	shuffleCoords(coords, password)

	bitIndex := 0

	for _, c := range coords {
		if bitIndex >= len(bits) {
			break
		}

		writer.x = c[0]
		writer.y = c[1]

		for ch := 0; ch < writer.nch && bitIndex < len(bits); ch++ {
			writer.ch = ch
			if err := writer.WriteBit(bits[bitIndex]); err != nil {
				return nil, err
			}
			bitIndex++
		}
	}

	if bitIndex < len(bits) {
		return nil, fmt.Errorf("not enough masked capacity: embedded %d of %d bits", bitIndex, len(bits))
	}

	return writer.Image(), nil
}
