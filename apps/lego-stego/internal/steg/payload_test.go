package steg

import (
	"encoding/binary"
	"testing"
)

func TestPayloadRoundTrip(t *testing.T) {
	input := []byte("hello secret world")
	password := "testpass"

	packed, err := BuildPayload(input, password, 3)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	out, err := ParsePayload(packed, password)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if string(out) != string(input) {
		t.Fatalf("mismatch: got %s", out)
	}
}

func TestParsePayloadRejectsInvalidHeaderFieldsAndLength(t *testing.T) {
	valid, err := BuildPayload([]byte("secret"), "password", 3)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func([]byte)
	}{
		{name: "version", mutate: func(data []byte) { data[4] = 2 }},
		{name: "flags", mutate: func(data []byte) { data[5] = 0xff }},
		{name: "channels", mutate: func(data []byte) { data[6] = 255 }},
		{name: "oversized length", mutate: func(data []byte) { binary.BigEndian.PutUint32(data[8:12], ^uint32(0)) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := append([]byte(nil), valid...)
			tt.mutate(data)
			if _, err := ParsePayload(data, "password"); err == nil {
				t.Fatal("expected malformed payload error")
			}
		})
	}
}

func TestWrongPassword(t *testing.T) {
	input := []byte("secret")
	password := "correct"

	packed, _ := BuildPayload(input, password, 3)

	_, err := ParsePayload(packed, "wrong")
	if err == nil {
		t.Fatalf("expected failure with wrong password")
	}
}
