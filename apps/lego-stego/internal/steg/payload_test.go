package steg

import "testing"

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

func TestWrongPassword(t *testing.T) {
	input := []byte("secret")
	password := "correct"

	packed, _ := BuildPayload(input, password, 3)

	_, err := ParsePayload(packed, "wrong")
	if err == nil {
		t.Fatalf("expected failure with wrong password")
	}
}
