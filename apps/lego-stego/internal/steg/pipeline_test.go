package steg

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"
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

	start := time.Now()
	_, err := Extract(stego, "wrong")
	if err == nil {
		t.Fatalf("expected failure with wrong password")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("wrong-password rejection took %v; malformed headers must not trigger a full-image quadratic scan", elapsed)
	}
}

func TestEmbedExtractWithNonZeroImageBounds(t *testing.T) {
	img := image.NewRGBA(image.Rect(10, 20, 210, 220))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	data := []byte("bounded image payload")
	encoded, err := Embed(img, data, "password")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Extract(encoded, "password")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatalf("decoded %q, want %q", decoded, data)
	}
}

func TestExtractRejectsDeclaredLengthBeyondCapacity(t *testing.T) {
	img := noisyImage(100, 100)
	writer := NewBitWriter(img)
	header, err := EncodeHeader(Header{Version: 1, Flags: 0, Channels: 3, Length: 1})
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(header[8:12], ^uint32(0))
	bits := bytesToBits(header)
	coords := filterCoords(pixelCoords(img), selectEmbeddingMask(img, 20.0), 32)
	shuffleCoords(coords, "password")
	bitIndex := 0
	for _, coord := range coords {
		writer.x, writer.y = coord[0], coord[1]
		for ch := 0; ch < writer.nch && bitIndex < len(bits); ch++ {
			writer.ch = ch
			if err := writer.WriteBit(bits[bitIndex]); err != nil {
				t.Fatal(err)
			}
			bitIndex++
		}
		if bitIndex == len(bits) {
			break
		}
	}

	_, err = Extract(writer.Image(), "password")
	if err == nil || !strings.Contains(err.Error(), "exceeds image capacity") {
		t.Fatalf("Extract() error = %v, want capacity error", err)
	}
}
