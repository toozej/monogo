package steg

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	goqrcode "github.com/skip2/go-qrcode"
	"github.com/toozej/monogo/apps/lego-stego/internal/atomicfile"
)

func EmbedQRCode(url, inputPath, outputPath string) error {
	f, err := os.Open(inputPath) // #nosec G304 -- path provided by user via CLI flag
	if err != nil {
		return err
	}

	img, _, decodeErr := image.Decode(f)
	closeErr := f.Close()
	if decodeErr != nil {
		return decodeErr
	}
	if closeErr != nil {
		return closeErr
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

	img, _, decodeErr := image.Decode(f)
	closeErr := f.Close()
	if decodeErr != nil {
		return "", decodeErr
	}
	if closeErr != nil {
		return "", closeErr
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
	return atomicfile.Write(outputPath, 0600, func(w io.Writer) error {
		return png.Encode(w, img)
	})
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
