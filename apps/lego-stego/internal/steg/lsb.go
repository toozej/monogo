package steg

import (
	"errors"
	"image"
	"image/color"
)

type BitWriter struct {
	img    *image.RGBA
	bounds image.Rectangle
	x, y   int
	ch     int
	nch    int
}

func NewBitWriter(src image.Image) *BitWriter {
	b := src.Bounds()
	out := image.NewRGBA(b)

	// copy
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x, y, src.At(x, y))
		}
	}

	nch := 3
	if _, ok := src.(*image.Gray); ok {
		nch = 1
	}

	return &BitWriter{
		img:    out,
		bounds: b,
		x:      b.Min.X,
		y:      b.Min.Y,
		ch:     0,
		nch:    nch,
	}
}

func (w *BitWriter) Capacity() int {
	pixels := w.bounds.Dx() * w.bounds.Dy()
	return pixels * w.nch
}

func (w *BitWriter) WriteBit(bit uint8) error {
	if w.y >= w.bounds.Max.Y {
		return errors.New("out of capacity")
	}

	r, g, b, a := w.img.At(w.x, w.y).RGBA()
	r8 := color32to8(r)
	g8 := color32to8(g)
	b8 := color32to8(b)

	if w.nch == 1 {
		v := r8
		v = (v & 0xFE) | bit
		w.img.SetRGBA(w.x, w.y, color.RGBA{v, v, v, color32to8(a)})
	} else {
		switch w.ch {
		case 0:
			r8 = (r8 & 0xFE) | bit
		case 1:
			g8 = (g8 & 0xFE) | bit
		case 2:
			b8 = (b8 & 0xFE) | bit
		}
		w.img.SetRGBA(w.x, w.y, color.RGBA{r8, g8, b8, color32to8(a)})
	}

	w.advance()
	return nil
}

func (w *BitWriter) advance() {
	w.ch++
	if w.ch >= w.nch {
		w.ch = 0
		w.x++
		if w.x >= w.bounds.Max.X {
			w.x = w.bounds.Min.X
			w.y++
		}
	}
}

func (w *BitWriter) Image() *image.RGBA {
	return w.img
}
