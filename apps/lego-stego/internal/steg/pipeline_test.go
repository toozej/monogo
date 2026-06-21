package steg

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func noisyImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*y + y) % 255)
			img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func TestEmbedExtractPipeline(t *testing.T) {
	img := noisyImage(300, 300)

	data := []byte("super secret payload")
	password := "hunter2"

	stego, err := Embed(img, data, password)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	out, err := Extract(stego, password)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if !bytes.Equal(out, data) {
		t.Fatalf("mismatch: %s", out)
	}
}

func TestWrongPasswordPipeline(t *testing.T) {
	img := noisyImage(300, 300)

	data := []byte("secret")
	stego, _ := Embed(img, data, "correct")

	_, err := Extract(stego, "wrong")
	if err == nil {
		t.Fatalf("expected failure with wrong password")
	}
}
