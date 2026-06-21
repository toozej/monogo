package ecc

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	data := []byte("this is some test data for ecc round trip")

	encoded, err := Encode(data)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bytes.Equal(decoded, data) {
		t.Fatalf("mismatch: got %s, want %s", decoded, data)
	}
}

func TestEncodeDecodeEmptyData(t *testing.T) {
	data := []byte("")

	_, err := Encode(data)
	if err == nil {
		t.Fatalf("expected error for empty data: not enough data to fill shards")
	}
}

func TestRecoveryWithMissingShard(t *testing.T) {
	data := []byte("resilient data that should survive shard loss")

	encoded, err := Encode(data)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	recovered, err := DecodeWithMissing(encoded, []int{0})
	if err != nil {
		t.Fatalf("decode with missing shard 0 failed: %v", err)
	}

	if !bytes.Equal(recovered, data) {
		t.Fatalf("mismatch after recovery: got %s, want %s", recovered, data)
	}
}

func TestRecoveryWithMultipleMissingShards(t *testing.T) {
	data := []byte("data with multiple missing shards for recovery test")

	encoded, err := Encode(data)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	recovered, err := DecodeWithMissing(encoded, []int{0, 2})
	if err != nil {
		t.Fatalf("decode with missing shards failed: %v", err)
	}

	if !bytes.Equal(recovered, data) {
		t.Fatalf("mismatch after recovery: got %s, want %s", recovered, data)
	}
}

func TestDecodeInvalidData(t *testing.T) {
	_, err := Decode([]byte("short"))
	if err == nil {
		t.Fatalf("expected error for invalid data")
	}
}

func TestDecodeWithMissingInvalidData(t *testing.T) {
	_, err := DecodeWithMissing([]byte("x"), nil)
	if err == nil {
		t.Fatalf("expected error for data too short")
	}
}

func TestEncodedSizeLargerThanInput(t *testing.T) {
	data := []byte("hello")
	encoded, err := Encode(data)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	if len(encoded) <= len(data) {
		t.Fatalf("encoded data should be larger due to parity shards and length prefix")
	}
}
