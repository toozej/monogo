package security

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncryptionRotationAndBinding(t *testing.T) {
	k := Keyring{Active: "one", Keys: map[string]string{"one": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("1", 32))), "two": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("2", 32)))}}
	e, err := k.Encrypt("credential-id", []byte("password"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Data == "password" {
		t.Fatal("plaintext was stored")
	}
	if _, err := k.Decrypt("other-record", e); err == nil {
		t.Fatal("ciphertext moved to another record")
	}
	k.Active = "two"
	plain, err := k.Decrypt("credential-id", e)
	if err != nil || string(plain) != "password" {
		t.Fatal("old key was not available during rotation")
	}
	e2, err := k.Encrypt("credential-id", plain)
	if err != nil || e2.Version != "two" {
		t.Fatal("active version not used")
	}
	delete(k.Keys, "one")
	if _, err := k.Decrypt("credential-id", e); err == nil {
		t.Fatal("missing old key did not fail")
	}
}
func TestTokens(t *testing.T) {
	a, b := Token(), Token()
	if a == b || len(a) < 40 || Hash(a) == a {
		t.Fatal("invalid token generation")
	}
}
