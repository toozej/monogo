package steg

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/spf13/afero"
)

func createCarrier(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{200, 200, 200, 255})
		}
	}
	return img
}

func TestBitRoundTrip(t *testing.T) {
	img := createCarrier(100, 100)

	writer := NewBitWriter(img)

	data := []byte("hello world")
	bits := bytesToBits(data)

	for _, bit := range bits {
		if err := writer.WriteBit(bit); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	reader := NewBitReader(writer.Image())

	var outBits []uint8
	for range bits {
		b, _ := reader.ReadBit()
		outBits = append(outBits, b)
	}

	out := bitsToBytes(outBits)

	if !bytes.Equal(out, data) {
		t.Fatalf("mismatch: %s", out)
	}
}

func TestAferoImageRoundTrip(t *testing.T) {
	fs := afero.NewMemMapFs()

	img := createCarrier(200, 200)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	if err := afero.WriteFile(fs, "/carrier.png", buf.Bytes(), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// load again
	f, _ := fs.Open("/carrier.png")
	decoded, _, _ := image.Decode(f)

	writer := NewBitWriter(decoded)
	data := []byte("secret")

	for _, bit := range bytesToBits(data) {
		if err := writer.WriteBit(bit); err != nil {
			t.Fatalf("write bit failed: %v", err)
		}
	}

	var outBuf bytes.Buffer
	if err := png.Encode(&outBuf, writer.Image()); err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	if err := afero.WriteFile(fs, "/stego.png", outBuf.Bytes(), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// read back
	f2, _ := fs.Open("/stego.png")
	img2, _, _ := image.Decode(f2)

	reader := NewBitReader(img2)

	var bits []uint8
	for range bytesToBits(data) {
		b, _ := reader.ReadBit()
		bits = append(bits, b)
	}

	out := bitsToBytes(bits)

	if string(out) != "secret" {
		t.Fatalf("failed: %s", out)
	}
}
