package steg

import (
	"bytes"
)

func bytesToBits(data []byte) []uint8 {
	var bits []uint8
	for _, b := range data {
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>i)&1)
		}
	}
	return bits
}

func bitsToBytes(bits []uint8) []byte {
	var buf bytes.Buffer

	for i := 0; i < len(bits); i += 8 {
		var b uint8
		for j := 0; j < 8; j++ {
			b = (b << 1) | bits[i+j]
		}
		buf.WriteByte(b)
	}

	return buf.Bytes()
}
