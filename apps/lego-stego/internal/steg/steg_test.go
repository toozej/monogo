package steg_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/toozej/lego-stego/internal/steg"
)

func createNoisyGrayCarrier(w, h int) image.Image {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*y + y) % 255)
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return img
}

func createNoisyRGBCarrier(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*y + y) % 255)
			img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestEmbedExtractGray(t *testing.T) {
	tmpDir := t.TempDir()
	carrierPath := filepath.Join(tmpDir, "carrier.png")
	stegoPath := filepath.Join(tmpDir, "stego.png")
	qrPath := filepath.Join(tmpDir, "qr.png")

	carrier, err := encodePNG(createNoisyGrayCarrier(400, 400))
	if err != nil {
		t.Fatalf("encode carrier failed: %v", err)
	}
	if err := os.WriteFile(carrierPath, carrier, 0644); err != nil {
		t.Fatalf("write carrier failed: %v", err)
	}

	err = steg.EmbedQRCode("https://example.com", carrierPath, stegoPath)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	_, err = steg.ExtractAndDecode(stegoPath, qrPath)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if _, err := os.Stat(qrPath); os.IsNotExist(err) {
		t.Fatalf("missing output qr.png")
	}
}

func TestEmbedExtractRGB(t *testing.T) {
	tmpDir := t.TempDir()
	carrierPath := filepath.Join(tmpDir, "carrier.png")
	stegoPath := filepath.Join(tmpDir, "stego.png")
	qrPath := filepath.Join(tmpDir, "qr.png")

	carrier, err := encodePNG(createNoisyRGBCarrier(400, 400))
	if err != nil {
		t.Fatalf("encode carrier failed: %v", err)
	}
	if err := os.WriteFile(carrierPath, carrier, 0644); err != nil {
		t.Fatalf("write carrier failed: %v", err)
	}

	err = steg.EmbedQRCode("https://example.com", carrierPath, stegoPath)
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	_, err = steg.ExtractAndDecode(stegoPath, qrPath)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
}

func TestEmbedExtractGrayDirect(t *testing.T) {
	carrier := createNoisyGrayCarrier(400, 400)
	data := []byte("test secret data for gray image")

	stego, err := steg.Embed(carrier, data, "")
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, stego); err != nil {
		t.Fatalf("encode stego failed: %v", err)
	}

	decoded, _, err := image.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode stego failed: %v", err)
	}

	out, err := steg.Extract(decoded, "")
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if string(out) != string(data) {
		t.Fatalf("mismatch: got %s, want %s", out, data)
	}
}

func TestEmbedExtractRGBDirect(t *testing.T) {
	carrier := createNoisyRGBCarrier(400, 400)
	data := []byte("test secret data for rgb image")

	stego, err := steg.Embed(carrier, data, "password123")
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, stego); err != nil {
		t.Fatalf("encode stego failed: %v", err)
	}

	decoded, _, err := image.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode stego failed: %v", err)
	}

	out, err := steg.Extract(decoded, "password123")
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if string(out) != string(data) {
		t.Fatalf("mismatch: got %s, want %s", out, data)
	}
}
