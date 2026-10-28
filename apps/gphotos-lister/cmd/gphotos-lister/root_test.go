package cmd

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/gphotos-lister/internal/config"
	"github.com/toozej/monogo/apps/gphotos-lister/internal/lister"
)

const albumURL = "https://photos.google.com/share/album"

type fakeBackend struct {
	names []string
	err   error
	album string
}

func (f *fakeBackend) LikedFilenames(_ context.Context, album string) ([]string, error) {
	f.album = album
	return f.names, f.err
}
func testConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{Output: filepath.Join(t.TempDir(), "names.txt"), Timeout: time.Minute, WebHost: "127.0.0.1", WebPort: 8080}
}
func testDependencies(conf config.Config, fake *fakeBackend) dependencies {
	return dependencies{load: func() (config.Config, error) { return conf, nil }, reader: func(config.Config, *slog.Logger) backend { return fake }}
}

func TestCLIList(t *testing.T) {
	for _, args := range [][]string{{albumURL}, {"--album", albumURL}, {}} {
		conf := testConfig(t)
		conf.Album = albumURL
		fake := &fakeBackend{names: []string{"IMG.JPG", "旅行.heic"}}
		command := newRootCommand(testDependencies(conf, fake))
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetArgs(append(args, "--prefix", "pre/", "--suffix", ".suffix"))
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(conf.Output)
		if err != nil || string(data) != "pre/IMG.JPG.suffix\npre/旅行.heic.suffix\n" || fake.album != albumURL || !strings.Contains(output.String(), "Wrote 2 filenames") {
			t.Fatalf("%s %s %v", data, output.String(), err)
		}
	}
}

func TestCLIFlagPrecedence(t *testing.T) {
	conf := testConfig(t)
	conf.Album = albumURL
	conf.Copy = true
	conf.Prefix = "environment"
	fake := &fakeBackend{}
	deps := testDependencies(conf, fake)
	var got config.Config
	deps.reader = func(c config.Config, _ *slog.Logger) backend { got = c; return fake }
	command := newRootCommand(deps)
	command.SetArgs([]string{"--copy=false", "--prefix=", "--timeout", "3m", "--no-overwrite", "--source", "src", "--destination", "dst", "--output", conf.Output})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Copy || got.Prefix != "" || got.Timeout != 3*time.Minute || !got.NoOverwrite || got.Source != "src" || got.Destination != "dst" {
		t.Fatalf("%+v", got)
	}
}

func TestDebugLogging(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configured bool
		args       []string
		want       bool
	}{
		{name: "default"},
		{name: "flag", args: []string{"--debug"}, want: true},
		{name: "environment", configured: true, want: true},
		{name: "disabled flag", configured: true, args: []string{"--debug=false"}},
		{name: "server", args: []string{"serve", "--debug"}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf := testConfig(t)
			conf.Album = albumURL
			conf.Debug = tc.configured
			fake := &fakeBackend{names: []string{"IMG.JPG"}}
			deps := testDependencies(conf, fake)
			deps.reader = func(c config.Config, logger *slog.Logger) backend {
				if c.Debug != tc.want {
					t.Fatalf("debug = %v, want %v", c.Debug, tc.want)
				}
				logger.Debug("Reader diagnostics", "photo_count", 1)
				return fake
			}
			deps.serve = func(context.Context, config.Config, lister.AlbumReader) error { return nil }
			command := newRootCommand(deps)
			var stdout, stderr bytes.Buffer
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetArgs(tc.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(stderr.String(), "Reader diagnostics") != tc.want {
				t.Fatalf("unexpected debug log: %s", stderr.String())
			}
			if strings.Contains(stdout.String(), "DEBUG") {
				t.Fatalf("debug log reached stdout: %s", stdout.String())
			}
		})
	}
}

func TestCLIEmptySelection(t *testing.T) {
	conf := testConfig(t)
	conf.Album = albumURL
	command := newRootCommand(testDependencies(conf, &fakeBackend{}))
	var stderr bytes.Buffer
	command.SetErr(&stderr)
	command.SetOut(&bytes.Buffer{})
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(conf.Output)
	if err != nil || len(data) != 0 || !strings.Contains(stderr.String(), "No photo filenames matched") {
		t.Fatalf("data = %q, error = %v, stderr = %q", data, err, stderr.String())
	}
}

func TestCLIUtilityCommandsSkipConfig(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--help"}, {"serve", "--help"}} {
		deps := dependencies{load: func() (config.Config, error) { t.Fatal("utility read config"); return config.Config{}, nil }}
		command := newRootCommand(deps)
		command.SetArgs(args)
		command.SetOut(&bytes.Buffer{})
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCLIServe(t *testing.T) {
	conf := testConfig(t)
	fake := &fakeBackend{}
	deps := testDependencies(conf, fake)
	served := false
	deps.serve = func(_ context.Context, c config.Config, r lister.AlbumReader) error {
		served = true
		if c.WebPort != 9090 || c.WebHost != "localhost" || r != fake {
			t.Fatalf("%+v", c)
		}
		return nil
	}
	for _, args := range [][]string{{"serve", "--port", "9090", "--host", "localhost"}} {
		command := newRootCommand(deps)
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	if !served {
		t.Fatal("command did not run")
	}
}

func TestCLIRejectsInvalidInput(t *testing.T) {
	for _, args := range [][]string{{}, {"--album", "https://evil.test/album/a"}, {"--album", albumURL, albumURL}, {"--album", albumURL, "--copy"}, {"--album", albumURL, "--prefix", "bad\n"}, {"--timeout", "0s"}, {"serve", "--host", "0.0.0.0"}, {albumURL, "extra"}} {
		conf := testConfig(t)
		fake := &fakeBackend{}
		command := newRootCommand(testDependencies(conf, fake))
		command.SetArgs(args)
		if err := command.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
		if fake.album != "" {
			t.Fatal("reader ran for invalid input")
		}
	}
}

func TestCLIErrors(t *testing.T) {
	conf := testConfig(t)
	conf.Album = albumURL
	expected := errors.New("load failed")
	deps := testDependencies(conf, &fakeBackend{})
	deps.load = func() (config.Config, error) { return config.Config{}, expected }
	command := newRootCommand(deps)
	command.SetArgs(nil)
	if err := command.Execute(); !errors.Is(err, expected) {
		t.Fatal(err)
	}
	fake := &fakeBackend{err: expected}
	command = newRootCommand(testDependencies(conf, fake))
	command.SetArgs(nil)
	if err := command.Execute(); !errors.Is(err, expected) {
		t.Fatal(err)
	}
	if _, err := os.Stat(conf.Output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("album error created a list")
	}
	conf.Output = filepath.Join(t.TempDir(), "missing", "names.txt")
	command = newRootCommand(testDependencies(conf, &fakeBackend{names: []string{"a.jpg"}}))
	command.SetArgs(nil)
	if err := command.Execute(); err == nil {
		t.Fatal("expected write error")
	}
}

func TestCLICopyReportAndPartialFailure(t *testing.T) {
	conf := testConfig(t)
	conf.Album = albumURL
	conf.Copy = true
	conf.Source = t.TempDir()
	conf.Destination = t.TempDir()
	if err := os.WriteFile(filepath.Join(conf.Source, "a.jpg"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &fakeBackend{names: []string{"a.jpg", "missing.jpg"}}
	var out, stderr bytes.Buffer
	command := newRootCommand(testDependencies(conf, fake))
	command.SetArgs(nil)
	command.SetOut(&out)
	command.SetErr(&stderr)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Copied: 1; missing: 1") || !strings.Contains(stderr.String(), "Missing: missing.jpg") {
		t.Fatal(out.String(), stderr.String())
	}
	if err := os.Mkdir(filepath.Join(conf.Destination, "missing.jpg"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conf.Source, "missing.jpg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	command = newRootCommand(testDependencies(conf, fake))
	command.SetArgs(nil)
	if err := command.Execute(); err == nil {
		t.Fatal("expected a copy error")
	}
	data, err := os.ReadFile(conf.Output)
	if err != nil || string(data) != "a.jpg\nmissing.jpg\n" {
		t.Fatalf("%s %v", data, err)
	}
}
