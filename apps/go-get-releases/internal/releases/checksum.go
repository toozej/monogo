package releases

import (
	"bufio"
	"crypto/md5"  // #nosec G501 -- Verify an upstream legacy checksum.
	"crypto/sha1" // #nosec G505 -- Verify an upstream legacy checksum.
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"path"
	"regexp"
	"strings"
)

type checksum struct{ Algorithm, Value string }
type checksums map[string][]checksum

var bsdChecksum = regexp.MustCompile(`(?i)^(MD5|SHA1|SHA256|SHA512)\s*\((.*)\)\s*=\s*([0-9a-f]+)$`)
var checksumName = regexp.MustCompile(`(?i)(?:^|[._-])(?:checksums?|hashes|sha(?:1|256|512)(?:sums?)?|md5(?:sums?)?|shasums)(?:[._-]|$)`)

var hexChecksum = regexp.MustCompile(`^[0-9a-fA-F]+$`)

func checksumAsset(name string) bool {
	n := strings.ToLower(name)
	if strings.HasSuffix(n, ".asc") || strings.HasSuffix(n, ".sig") || strings.HasSuffix(n, ".sigstore.json") {
		return false
	}
	if checksumName.MatchString(n) {
		return true
	}
	return strings.HasSuffix(n, ".sha")
}

func algorithmForLength(n int) string {
	switch n {
	case 32:
		return "md5"
	case 40:
		return "sha1"
	case 64:
		return "sha256"
	case 128:
		return "sha512"
	}
	return ""
}

func newHash(algorithm string) (hash.Hash, error) {
	switch strings.ToLower(algorithm) {
	case "md5":
		// Verify the published checksum. Store a separate SHA-256 for the local cache.
		// noopengrep: go.lang.security.audit.crypto.use_of_weak_crypto.use-of-md5
		return md5.New(), nil // #nosec G401 -- Verify an upstream legacy checksum.
	case "sha1":
		// Verify the published checksum. Store a separate SHA-256 for the local cache.
		// noopengrep: go.lang.security.audit.crypto.use_of_weak_crypto.use-of-sha1
		return sha1.New(), nil // #nosec G401 -- Verify an upstream legacy checksum.
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	}
	return nil, fmt.Errorf("unsupported checksum algorithm: %s", algorithm)
}

func parseDigest(digest string) (checksum, error) {
	algorithm, value, ok := strings.Cut(digest, ":")
	h, err := newHash(algorithm)
	if !ok || err != nil || len(value) != h.Size()*2 || !hexChecksum.MatchString(value) {
		return checksum{}, fmt.Errorf("invalid or unsupported asset digest: %q", digest)
	}
	return checksum{strings.ToLower(algorithm), strings.ToLower(value)}, nil
}

// parseChecksums supports GNU, BSD, and bare per-asset checksum files.
func parseChecksums(name string, data []byte) (checksums, error) {
	result := checksums{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var file, value, algorithm string
		if m := bsdChecksum.FindStringSubmatch(line); m != nil {
			algorithm = strings.ToLower(m[1])
			file = m[2]
			value = m[3]
		} else {
			fields := strings.Fields(line)
			value = fields[0]
			algorithm = algorithmForLength(len(value))
			if algorithm == "" || !hexChecksum.MatchString(value) {
				return nil, fmt.Errorf("invalid checksum record in %s", name)
			}
			if len(fields) > 1 {
				file = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line[len(value):]), "*"))
			} else {
				// Sidecars use asset.ext.sha256 or asset.ext.sha256.txt.
				file = strings.TrimSuffix(name, ".txt")
				suffix := ""
				for _, candidate := range []string{"." + algorithm + "sum", "." + algorithm, ".sha"} {
					if strings.HasSuffix(strings.ToLower(file), candidate) {
						suffix = candidate
						break
					}
				}
				if suffix == "" {
					return nil, fmt.Errorf("bare checksum has no matching asset name: %s", name)
				}
				file = file[:len(file)-len(suffix)]
			}
		}
		file = path.Base(strings.ReplaceAll(file, "\\", "/"))
		if _, err := parseDigest(algorithm + ":" + value); err != nil {
			return nil, err
		}
		result[file] = append(result[file], checksum{algorithm, strings.ToLower(value)})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("checksum file %s contains no supported checksums", name)
	}
	return result, nil
}

func verify(reader io.Reader, expected []checksum) (string, error) {
	hashes := map[string]hash.Hash{"sha256": sha256.New()}
	for _, c := range expected {
		h, err := newHash(c.Algorithm)
		if err != nil {
			return "", err
		}
		hashes[c.Algorithm] = h
	}
	writers := make([]io.Writer, 0, len(hashes))
	for _, h := range hashes {
		writers = append(writers, h)
	}
	if _, err := io.Copy(io.MultiWriter(writers...), reader); err != nil {
		return "", err
	}
	for _, c := range expected {
		actual := hex.EncodeToString(hashes[c.Algorithm].Sum(nil))
		if actual != c.Value {
			return "", errors.New("checksum does not match: " + c.Algorithm)
		}
	}
	return hex.EncodeToString(hashes["sha256"].Sum(nil)), nil
}
