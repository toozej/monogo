package api

import (
	"bytes"
	"image"
	"image/png"
	"os"

	"github.com/toozej/lego-stego/internal/steg"
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

		// #nosec G304 -- path provided by caller
		outFile, err := os.Create(out)
		if err != nil {
			return "", err
		}
		defer func() { _ = outFile.Close() }()

		if err := png.Encode(outFile, qrImg); err != nil {
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
	defer func() { _ = f.Close() }()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	stego, err := steg.Embed(img, data, password)
	if err != nil {
		return err
	}

	// #nosec G304 -- path provided by caller
	outF, err := os.Create(out)
	if err != nil {
		return err
	}
	defer func() { _ = outF.Close() }()

	return png.Encode(outF, stego)
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
