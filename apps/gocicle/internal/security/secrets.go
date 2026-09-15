// Package security encrypts secrets and hashes bearer tokens.
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

type Keyring struct {
	Active string            `json:"active"`
	Keys   map[string]string `json:"keys"`
}
type Encrypted struct {
	Version string `json:"version"`
	Data    string `json:"data"`
}

func Token() string { return base64.RawURLEncoding.EncodeToString(random(32)) }
func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func random(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
func Load(path string) (Keyring, error) {
	var k Keyring
	f, err := os.Open(path) // #nosec G304 -- The operator explicitly configures the external encryption key path.
	if err != nil {
		return k, errors.New("cannot read the encryption key file")
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return k, errors.New("encryption key file permissions must be 0600")
	}
	b, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return k, err
	}
	if err = json.Unmarshal(b, &k); err != nil {
		return k, errors.New("encryption key file is invalid")
	}
	_, err = k.aead(k.Active)
	return k, err
}
func (k Keyring) aead(version string) (cipher.AEAD, error) {
	raw, ok := k.Keys[version]
	if !ok || version == "" {
		return nil, errors.New("encryption key version is unavailable")
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(b) != 32 {
		return nil, errors.New("encryption keys must contain 32 base64-encoded bytes")
	}
	block, err := aes.NewCipher(b)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// The associated data binds ciphertext to its database record.
func (k Keyring) Encrypt(id string, plain []byte) (Encrypted, error) {
	a, err := k.aead(k.Active)
	if err != nil {
		return Encrypted{}, err
	}
	nonce := random(a.NonceSize())
	return Encrypted{Version: k.Active, Data: base64.StdEncoding.EncodeToString(a.Seal(nonce, nonce, plain, []byte(id)))}, nil
}
func (k Keyring) Decrypt(id string, e Encrypted) ([]byte, error) {
	a, err := k.aead(e.Version)
	if err != nil {
		return nil, err
	}
	b, err := base64.StdEncoding.DecodeString(e.Data)
	if err != nil || len(b) < a.NonceSize() {
		return nil, errors.New("encrypted secret is invalid")
	}
	return a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte(id))
}
func Redact(value string, secrets []string) string {
	secrets = append([]string(nil), secrets...)
	sort.SliceStable(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	for _, s := range secrets {
		if s != "" {
			value = strings.ReplaceAll(value, s, "[REDACTED]")
		}
	}
	return value
}
