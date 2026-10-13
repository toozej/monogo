package releases

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func hashText(t *testing.T, algorithm, body string) string {
	t.Helper()
	h, err := newHash(algorithm)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = h.Write([]byte(body))
	return hex.EncodeToString(h.Sum(nil))
}

func TestChecksums(t *testing.T) {
	body := "release package"
	for _, algorithm := range []string{"md5", "sha1", "sha256", "sha512"} {
		sum := hashText(t, algorithm, body)
		for _, tc := range []struct{ name, text string }{
			{"checksums.txt", sum + "  app.zip\n"},
			{"checksums.txt", sum + " *app.zip\n"},
			{"checksums.txt", strings.ToUpper(algorithm) + " (app.zip) = " + sum + "\n"},
			{"app.zip." + algorithm, sum + "\n"},
			{"app.zip." + algorithm + "sum", sum + "\n"},
			{"app.zip.sha", sum + "\n"},
			{"app.zip." + algorithm + ".txt", strings.ToUpper(sum) + "\n"},
			{"checksums.txt", "# comment\n\n" + sum + "  ./app.zip\n"},
			{"checksums.txt", sum + "  dist/app.zip\n"},
			{"checksums.txt", sum + "  dist\\app.zip\n"},
		} {
			got, err := parseChecksums(tc.name, []byte(tc.text))
			if err != nil {
				t.Fatal(err)
			}
			expected := got["app.zip"]
			if len(expected) != 1 || expected[0].Algorithm != algorithm {
				t.Fatalf("%+v", got)
			}
			if _, err := verify(strings.NewReader(body), expected); err != nil {
				t.Fatal(err)
			}
			if _, err := verify(strings.NewReader("corrupt"), expected); err == nil {
				t.Fatal("accepted corrupt bytes")
			}
		}
	}
	sum := hashText(t, "sha256", body)
	got, err := parseChecksums("checksums.txt", []byte(sum+"  name with spaces.zip\n"))
	if err != nil || len(got["name with spaces.zip"]) != 1 {
		t.Fatal(got, err)
	}
	if got, err := verify(strings.NewReader(body), nil); err != nil || got != sum {
		t.Fatal(got, err)
	}
}

func TestInvalidChecksums(t *testing.T) {
	for _, digest := range []string{"sha256:xyz", "sha999:123", "sha256", "sha1:abc", "md5:" + strings.Repeat("g", 32)} {
		if _, err := parseDigest(digest); err == nil {
			t.Errorf("accepted %s", digest)
		}
	}
	if _, err := newHash("sha999"); err == nil {
		t.Fatal("accepted algorithm")
	}
	for _, data := range []string{"", "# comment", "not a checksum", strings.Repeat("0", 64) + "  app.zip\ninvalid  another.zip", "SHA256 (app.zip) = abc", strings.Repeat("0", 64), strings.Repeat("x", 1024*1024+1)} {
		if _, err := parseChecksums("checksums.txt", []byte(data)); err == nil {
			t.Errorf("accepted %.40s", data)
		}
	}
	for _, name := range []string{"checksums.txt", "SHA256SUMS", "SHA512SUMS.txt", "app.zip.md5", "app.zip.sha1", "app.zip.sha"} {
		if !checksumAsset(name) {
			t.Errorf("missed %s", name)
		}
	}
	for _, name := range []string{"checksums.txt.asc", "checksums.txt.sig", "checksums.txt.sigstore.json", "app.zip", "md5tool-linux-amd64.zip"} {
		if checksumAsset(name) {
			t.Errorf("misclassified %s", name)
		}
	}
	if _, err := verify(strings.NewReader("x"), []checksum{{"bad", "x"}}); err == nil {
		t.Fatal("accepted algorithm")
	}
	if _, err := verify(failedReader{}, nil); err == nil {
		t.Fatal("accepted failed reader")
	}
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, fmt.Errorf("read failed") }
