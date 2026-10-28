package lister

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type readerFunc func(context.Context, string) ([]string, error)

func (f readerFunc) LikedFilenames(ctx context.Context, album string) ([]string, error) {
	return f(ctx, album)
}
func reader(names ...string) AlbumReader {
	return readerFunc(func(context.Context, string) ([]string, error) { return names, nil })
}
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRunFormatsOriginalNames(t *testing.T) {
	for _, tc := range []struct{ name, prefix, suffix, want string }{
		{"plain", "", "", "IMG 1.JPG\n旅行.heic\n"},
		{"prefix", "/photos/", "", "/photos/IMG 1.JPG\n/photos/旅行.heic\n"},
		{"suffix", "", ".original", "IMG 1.JPG.original\n旅行.heic.original\n"},
		{"both", "\"", "\"", "\"IMG 1.JPG\"\n\"旅行.heic\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Run(context.Background(), reader("IMG 1.JPG", "旅行.heic", "IMG 1.JPG"), Options{Album: "album", Prefix: tc.prefix, Suffix: tc.suffix})
			if err != nil || result.Text != tc.want || len(result.Filenames) != 2 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
	result, err := Run(context.Background(), reader(), Options{Album: "album"})
	if err != nil || result.Text != "" || len(result.Filenames) != 0 {
		t.Fatalf("empty: %+v %v", result, err)
	}
}

func TestRunValidation(t *testing.T) {
	for _, options := range []Options{{}, {Album: " \t"}, {Album: "a", Prefix: "x\ny"}, {Album: "a", Suffix: "\r"}, {Album: "a", Suffix: "\x00"}, {Album: "a", Copy: true}, {Album: "a", Copy: true, Source: "source"}} {
		_, err := Run(context.Background(), readerFunc(func(context.Context, string) ([]string, error) { t.Fatal("reader must not run"); return nil, nil }), options)
		if err == nil {
			t.Fatalf("accepted %+v", options)
		}
	}
	for _, name := range []string{"", ".", "..", "../a.jpg", "a/b.jpg", "a\\b.jpg", "a\nb.jpg", "a\rb.jpg", "a\x00b.jpg", "/a.jpg"} {
		if _, err := Run(context.Background(), reader(name), Options{Album: "a"}); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	expected := errors.New("session expired")
	_, err := Run(context.Background(), readerFunc(func(context.Context, string) ([]string, error) { return nil, expected }), Options{Album: "a"})
	if !errors.Is(err, expected) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, reader("a.jpg"), Options{Album: "a"}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestCopiesAndReports(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(t.TempDir(), "new")
	write(t, filepath.Join(source, "nested", "found.jpg"), "original")
	write(t, filepath.Join(source, "dup.jpg"), "one")
	write(t, filepath.Join(source, "nested", "dup.jpg"), "two")
	write(t, filepath.Join(destination, "found.jpg"), "old")
	write(t, filepath.Join(destination, "keep.jpg"), "keep")
	result, err := Run(context.Background(), reader("found.jpg", "missing.jpg", "dup.jpg"), Options{Album: "a", Prefix: "prefix/", Suffix: ".suffix", Copy: true, Source: source, Destination: destination})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Copied, []string{"found.jpg"}) || !reflect.DeepEqual(result.Missing, []string{"missing.jpg"}) || !reflect.DeepEqual(result.Ambiguous, []string{"dup.jpg"}) {
		t.Fatalf("%+v", result)
	}
	if read(t, filepath.Join(destination, "found.jpg")) != "original" || read(t, filepath.Join(source, "nested", "found.jpg")) != "original" || read(t, filepath.Join(destination, "keep.jpg")) != "keep" {
		t.Fatal("incorrect copy")
	}
	srcInfo, _ := os.Stat(filepath.Join(source, "nested", "found.jpg"))
	dstInfo, _ := os.Stat(filepath.Join(destination, "found.jpg"))
	if os.SameFile(srcInfo, dstInfo) {
		t.Fatal("copy must use a separate file")
	}
	if !strings.Contains(result.Text, "prefix/found.jpg.suffix\n") {
		t.Fatal(result.Text)
	}
}

func TestCopyNoOverwrite(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	write(t, filepath.Join(source, "a.jpg"), "new")
	write(t, filepath.Join(source, "b.jpg"), "b")
	write(t, filepath.Join(destination, "a.jpg"), "old")
	result, err := Run(context.Background(), reader("a.jpg", "b.jpg"), Options{Album: "a", Copy: true, Source: source, Destination: destination, NoOverwrite: true})
	if err != nil || !reflect.DeepEqual(result.Skipped, []string{"a.jpg"}) || !reflect.DeepEqual(result.Copied, []string{"b.jpg"}) {
		t.Fatalf("%+v %v", result, err)
	}
	if read(t, filepath.Join(destination, "a.jpg")) != "old" {
		t.Fatal("overwrote an existing file")
	}
}

func TestNestedDestinationIsExcluded(t *testing.T) {
	source := t.TempDir()
	destination := filepath.Join(source, "copies")
	write(t, filepath.Join(source, "a.jpg"), "original")
	write(t, filepath.Join(destination, "a.jpg"), "old")
	for range 2 {
		result, err := Run(context.Background(), reader("a.jpg"), Options{Album: "a", Copy: true, Source: source, Destination: destination})
		if err != nil || len(result.Copied) != 1 || len(result.Ambiguous) != 0 {
			t.Fatalf("%+v %v", result, err)
		}
	}
}

func TestCopySymlinks(t *testing.T) {
	source, destination, outside := t.TempDir(), t.TempDir(), t.TempDir()
	write(t, filepath.Join(outside, "external.jpg"), "external")
	if err := os.Symlink(filepath.Join(outside, "external.jpg"), filepath.Join(source, "link.jpg")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "directory")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(source, "a.jpg"), "original")
	if err := os.Symlink(filepath.Join(outside, "external.jpg"), filepath.Join(destination, "a.jpg")); err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), reader("link.jpg", "external.jpg", "a.jpg"), Options{Album: "a", Copy: true, Source: source, Destination: destination})
	if err != nil || len(result.Missing) != 2 || len(result.Copied) != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	if read(t, filepath.Join(outside, "external.jpg")) != "external" {
		t.Fatal("wrote through a destination symlink")
	}
	info, err := os.Lstat(filepath.Join(destination, "a.jpg"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("not a regular copy: %v", err)
	}
}

func TestCopyFailures(t *testing.T) {
	for _, kind := range []string{"missing-source", "source-file", "same-directory", "destination-file", "destination-collision"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			destination := filepath.Join(root, "destination")
			write(t, filepath.Join(source, "a.jpg"), "original")
			switch kind {
			case "missing-source":
				source = filepath.Join(root, "missing")
			case "source-file":
				source = filepath.Join(source, "a.jpg")
			case "same-directory":
				destination = source
			case "destination-file":
				write(t, destination, "file")
			case "destination-collision":
				if err := os.MkdirAll(filepath.Join(destination, "a.jpg"), 0o750); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Run(context.Background(), reader("a.jpg"), Options{Album: "a", Copy: true, Source: source, Destination: destination}); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestCopySameFileAndCancellation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "a.jpg")
	destination := filepath.Join(root, "b.jpg")
	write(t, source, "original")
	if err := os.Link(source, destination); err != nil {
		t.Skip(err)
	}
	if err := copyFile(context.Background(), source, destination, false); err == nil {
		t.Fatal("accepted the same inode")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := copyFile(ctx, source, filepath.Join(root, "c.jpg"), false); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "c.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestWriteTextAtomic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "list.txt")
	write(t, path, "old")
	if err := WriteText(path, "a.jpg\nb.jpg\n"); err != nil {
		t.Fatal(err)
	}
	if read(t, path) != "a.jpg\nb.jpg\n" {
		t.Fatal("incorrect list")
	}
	expected := errors.New("disk failed")
	err := atomicWrite(path, func(f *os.File) error { _, _ = f.WriteString("partial"); return expected }, false)
	if !errors.Is(err, expected) || read(t, path) != "a.jpg\nb.jpg\n" {
		t.Fatal(err)
	}
	if err := WriteText(path, ""); err != nil || read(t, path) != "" {
		t.Fatal(err)
	}
	if err := WriteText(filepath.Join(root, "list.csv"), "x"); err == nil {
		t.Fatal("accepted a non-txt file")
	}
	if err := WriteText(filepath.Join(root, "missing", "list.txt"), "x"); err == nil {
		t.Fatal("accepted a missing parent")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files remain: %v %v", entries, err)
	}
}
