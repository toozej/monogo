package steg

import (
	"bytes"
	"testing"

	"github.com/toozej/lego-stego/internal/ecc"
)

func TestECCRecovery(t *testing.T) {
	data := []byte("this is resilient data")
	password := "pass"

	payload, err := BuildPayload(data, password, 3)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	h, offset, err := DecodeHeader(payload)
	if err != nil {
		t.Fatalf("header decode failed: %v", err)
	}

	eccBlock := payload[offset : offset+int(h.Length)]

	if len(eccBlock) < ecc.OrigLenSize {
		t.Fatalf("ecc block too small")
	}

	eccData := eccBlock[ecc.OrigLenSize:]

	shardSize := len(eccData) / ecc.TotalShards
	if shardSize == 0 {
		t.Fatalf("shard size is zero")
	}

	corruptedShard1 := (ecc.OrigLenSize + 10) / shardSize
	corruptedShard2 := (ecc.OrigLenSize + 25) / shardSize
	missing := []int{corruptedShard1}
	if corruptedShard2 != corruptedShard1 {
		missing = append(missing, corruptedShard2)
	}

	recovered, err := ecc.DecodeWithMissing(eccBlock, missing)
	if err != nil {
		t.Fatalf("ecc decode failed: %v", err)
	}

	newHeader := Header{
		Version: h.Version,
		Flags:   h.Flags &^ 2,
		Length:  uint32(len(recovered)),
	}

	hdr, err := EncodeHeader(newHeader)
	if err != nil {
		t.Fatalf("encode header failed: %v", err)
	}

	rebuilt := make([]byte, 0, len(hdr)+len(recovered))
	rebuilt = append(rebuilt, hdr...)
	rebuilt = append(rebuilt, recovered...)

	out, err := ParsePayload(rebuilt, password)
	if err != nil {
		t.Fatalf("parse after recovery failed: %v", err)
	}

	if !bytes.Equal(out, data) {
		t.Fatalf("data mismatch after recovery: got %s", out)
	}
}

func TestECCRoundTrip(t *testing.T) {
	data := []byte("some test data for ecc")

	encoded, err := ecc.Encode(data)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := ecc.Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bytes.Equal(decoded, data) {
		t.Fatalf("mismatch: got %s, want %s", decoded, data)
	}
}
