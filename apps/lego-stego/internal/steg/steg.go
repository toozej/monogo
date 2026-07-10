package steg

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	goqrcode "github.com/skip2/go-qrcode"
)

func EmbedQRCode(url, inputPath, outputPath string) error {
	f, err := os.Open(inputPath) // #nosec G304 -- path provided by user via CLI flag
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	qrImg, err := GenerateQRCode(url)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, qrImg); err != nil {
		return err
	}

	stego, err := Embed(img, buf.Bytes(), "")
	if err != nil {
		return err
	}

	return writePNGAtomic(outputPath, stego)
}

func ExtractAndDecode(inputPath, outputPath string) (string, error) {
	f, err := os.Open(inputPath) // #nosec G304 -- path provided by user via CLI flag
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}

	data, err := Extract(img, "")
	if err != nil {
		return "", err
	}

	decoded, err := DecodeQRCode(data)
	if err != nil {
		return "", err
	}

	if outputPath != "" {
		qrImg, err := GenerateQRCode(decoded)
		if err != nil {
			return "", err
		}

		if err := writePNGAtomic(outputPath, qrImg); err != nil {
			return "", err
		}
	}

	return decoded, nil
}

func writePNGAtomic(outputPath string, img image.Image) error {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return err
	}
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(absPath), "."+filepath.Base(absPath)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if _, err := temp.Write(encoded.Bytes()); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, absPath)
}

func GenerateQRCode(data string) (image.Image, error) {
	qr, err := goqrcode.New(data, goqrcode.Medium)
	if err != nil {
		return nil, err
	}
	return qr.Image(256), nil
}

func DecodeQRCode(pngData []byte) (string, error) {
	reader := bytes.NewReader(pngData)
	img, _, err := image.Decode(reader)
	if err != nil {
		return "", err
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}

	qrReader := qrcode.NewQRCodeReader()
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}
