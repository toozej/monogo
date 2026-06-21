package steg

import (
	"errors"
	"image"
)

type BitReader struct {
	img    image.Image
	bounds image.Rectangle
	x, y   int
	ch     int
	nch    int
}

func NewBitReader(img image.Image) *BitReader {
	b := img.Bounds()

	nch := 3
	if _, ok := img.(*image.Gray); ok {
		nch = 1
	}

	return &BitReader{
		img:    img,
		bounds: b,
		x:      b.Min.X,
		y:      b.Min.Y,
		ch:     0,
		nch:    nch,
	}
}

func color32to8(v uint32) uint8 {
	return uint8(v >> 8) // #nosec G115 -- shift guarantees value fits in uint8
}

func (r *BitReader) ReadBit() (uint8, error) {
	if r.y >= r.bounds.Max.Y {
		return 0, errors.New("out of data")
	}

	rr, gg, bb, _ := r.img.At(r.x, r.y).RGBA()
	r8 := color32to8(rr)
	g8 := color32to8(gg)
	b8 := color32to8(bb)

	var bit uint8

	if r.nch == 1 {
		bit = r8 & 1
	} else {
		switch r.ch {
		case 0:
			bit = r8 & 1
		case 1:
			bit = g8 & 1
		case 2:
			bit = b8 & 1
		}
	}

	r.advance()
	return bit, nil
}

func (r *BitReader) advance() {
	r.ch++
	if r.ch >= r.nch {
		r.ch = 0
		r.x++
		if r.x >= r.bounds.Max.X {
			r.x = r.bounds.Min.X
			r.y++
		}
	}
}
