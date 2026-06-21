package steg

import (
	"errors"
	"math"

	"github.com/toozej/lego-stego/internal/crypto"
	"github.com/toozej/lego-stego/internal/ecc"
)

func BuildPayload(data []byte, password string, channels uint8) ([]byte, error) {
	enc, err := crypto.Encrypt(data, []byte(password))
	if err != nil {
		return nil, err
	}

	eccData, err := ecc.Encode(enc)
	if err != nil {
		return nil, err
	}

	if uint64(len(eccData)) > math.MaxUint32 {
		return nil, errors.New("payload too large for uint32 length field")
	}
	h := Header{
		Version:  1,
		Flags:    3,
		Channels: channels,
		Length:   uint32(len(eccData)), // #nosec G115 -- guarded by MaxUint32 check above
	}

	hdr, err := EncodeHeader(h)
	if err != nil {
		return nil, err
	}

	return append(hdr, eccData...), nil
}

func ParsePayload(data []byte, password string) ([]byte, error) {
	h, offset, err := DecodeHeader(data)
	if err != nil {
		return nil, err
	}

	if int(h.Length) < 0 {
		return nil, errors.New("payload length overflow")
	}

	if offset+int(h.Length) > len(data) {
		return nil, errors.New("payload length exceeds data size")
	}
	payload := data[offset : offset+int(h.Length)]

	if h.Flags&2 == 2 {
		payload, err = ecc.Decode(payload)
		if err != nil {
			return nil, err
		}
	}

	if h.Flags&1 == 1 {
		return crypto.Decrypt(payload, []byte(password))
	}

	return payload, nil
}
