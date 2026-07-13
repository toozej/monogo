package api

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/toozej/monogo/apps/lego-stego/internal/atomicfile"
	"github.com/toozej/monogo/apps/lego-stego/internal/steg"
)

func EmbedQR(in, out, url string, password string) error {
	qrImg, err := steg.GenerateQRCode(url)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, qrImg); err != nil {
		return err
	}
	qrcodeData := buf.Bytes()

	return EmbedFile(in, out, qrcodeData, password)
}

func ExtractQR(in, out string, password string) (string, error) {
	data, err := ExtractFile(in, password)
	if err != nil {
		return "", err
	}

	decodedURL, err := steg.DecodeQRCode(data)
	if err != nil {
		return "", err
	}

	if out != "" {
		qrImg, err := steg.GenerateQRCode(decodedURL)
		if err != nil {
			return "", err
		}

		if err := writePNGAtomic(out, qrImg); err != nil {
			return "", err
		}
	}

	return decodedURL, nil
}

func EmbedFile(in, out string, data []byte, password string) error {
	// #nosec G304 -- path provided by caller
	f, err := os.Open(in)
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

	stego, err := steg.Embed(img, data, password)
	if err != nil {
		return err
	}

	return writePNGAtomic(out, stego)
}

func writePNGAtomic(path string, img image.Image) error {
	return atomicfile.Write(path, 0600, func(w io.Writer) error {
		return png.Encode(w, img)
	})
}

func ExtractFile(in string, password string) ([]byte, error) {
	// #nosec G304 -- path provided by caller
	f, err := os.Open(in)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}

	return steg.Extract(img, password)
}
