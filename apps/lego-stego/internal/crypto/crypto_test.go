package crypto

import (
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	input := []byte("hello secret world")
	password := []byte("testpass")

	ciphertext, err := Encrypt(input, password)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	plaintext, err := Decrypt(ciphertext, password)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if string(plaintext) != string(input) {
		t.Fatalf("mismatch: got %s, want %s", plaintext, input)
	}
}

func TestEncryptDecryptEmptyData(t *testing.T) {
	input := []byte("")
	password := []byte("testpass")

	ciphertext, err := Encrypt(input, password)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	plaintext, err := Decrypt(ciphertext, password)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if string(plaintext) != "" {
		t.Fatalf("expected empty, got %s", plaintext)
	}
}

func TestWrongPassword(t *testing.T) {
	input := []byte("secret data")
	ciphertext, err := Encrypt(input, []byte("correct"))
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = Decrypt(ciphertext, []byte("wrong"))
	if err == nil {
		t.Fatalf("expected error with wrong password")
	}
}

func TestDecryptTruncatedData(t *testing.T) {
	_, err := Decrypt([]byte("short"), []byte("pass"))
	if err == nil {
		t.Fatalf("expected error for truncated data")
	}
}

func TestDecryptEmptyData(t *testing.T) {
	_, err := Decrypt([]byte{}, []byte("pass"))
	if err == nil {
		t.Fatalf("expected error for empty data")
	}
}

func TestCiphertextDiffersFromPlaintext(t *testing.T) {
	input := []byte("hello world")
	password := []byte("pass")

	ciphertext, err := Encrypt(input, password)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if len(ciphertext) <= len(input) {
		t.Fatalf("ciphertext should be larger than plaintext due to salt+nonce+tag")
	}
}

func TestDifferentSaltsProduceDifferentCiphertexts(t *testing.T) {
	input := []byte("same data")
	password := []byte("samepass")

	ct1, err := Encrypt(input, password)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	ct2, err := Encrypt(input, password)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if string(ct1) == string(ct2) {
		t.Fatalf("two encryptions of same data should differ due to random salt/nonce")
	}
}
