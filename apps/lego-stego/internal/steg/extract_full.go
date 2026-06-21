package steg

import (
	"fmt"
	"image"
)

type extractState struct {
	bits     []uint8
	needed   int
	nch      int
	complete bool
	lastErr  error
}

func Extract(img image.Image, password string) ([]byte, error) {
	coords := pixelCoords(img)
	mask := selectEmbeddingMask(img, 20.0)
	coords = filterCoords(coords, mask, 32)
	shuffleCoords(coords, password)

	nchOptions := []int{3, 1}

	states := make([]extractState, len(nchOptions))
	for i, nch := range nchOptions {
		states[i] = extractState{nch: nch}
	}

	readers := make([]*BitReader, len(nchOptions))
	for i := range nchOptions {
		readers[i] = NewBitReader(img)
	}

	for _, c := range coords {
		for i := range states {
			s := &states[i]
			if s.complete {
				continue
			}

			r := readers[i]
			r.x = c[0]
			r.y = c[1]

			for ch := 0; ch < s.nch; ch++ {
				r.ch = ch
				bit, err := r.ReadBit()
				if err != nil {
					s.complete = true
					continue
				}
				s.bits = append(s.bits, bit)
			}

			byteCount := len(s.bits) / 8
			if byteCount < 12 || len(s.bits)%8 != 0 {
				continue
			}

			data := bitsToBytes(s.bits[:byteCount*8])

			if s.needed == 0 {
				h, _, err := DecodeHeader(data)
				if err != nil {
					continue
				}
				s.needed = 12 + int(h.Length)
				if h.Channels > 0 && int(h.Channels) != s.nch {
					s.nch = int(h.Channels)
					s.bits = nil
					s.complete = true
					continue
				}
			}

			if byteCount < s.needed {
				continue
			}

			payload, err := ParsePayload(data[:s.needed], password)
			if err == nil {
				return payload, nil
			}
			s.complete = true
			s.lastErr = fmt.Errorf("payload decode failed (nch=%d): %w", s.nch, err)
		}

		allComplete := true
		for _, s := range states {
			if !s.complete {
				allComplete = false
				break
			}
		}
		if allComplete {
			break
		}
	}

	for _, s := range states {
		if s.lastErr != nil {
			return nil, s.lastErr
		}
	}

	return nil, fmt.Errorf("failed to extract payload")
}
