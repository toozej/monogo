package steg

import (
	"bytes"
	"encoding/binary"
	"errors"
)

var magic = []byte("HQR1")

type Header struct {
	Version  uint8
	Flags    uint8
	Channels uint8
	_        uint8
	Length   uint32
}

func EncodeHeader(h Header) ([]byte, error) {
	buf := new(bytes.Buffer)

	buf.Write(magic)
	buf.WriteByte(h.Version)
	buf.WriteByte(h.Flags)
	buf.WriteByte(h.Channels)
	buf.WriteByte(0)

	if err := binary.Write(buf, binary.BigEndian, h.Length); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DecodeHeader(data []byte) (Header, int, error) {
	if len(data) < 12 {
		return Header{}, 0, errors.New("header too small")
	}

	if !bytes.Equal(data[:4], magic) {
		return Header{}, 0, errors.New("invalid magic")
	}

	h := Header{
		Version:  data[4],
		Flags:    data[5],
		Channels: data[6],
		Length:   binary.BigEndian.Uint32(data[8:12]),
	}

	return h, 12, nil
}
