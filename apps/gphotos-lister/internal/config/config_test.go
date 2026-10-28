package config

import (
	"os"
	"testing"
	"time"
)

var envNames = []string{"GPHOTOS_ALBUM", "GPHOTOS_OUTPUT", "GPHOTOS_PREFIX", "GPHOTOS_SUFFIX", "GPHOTOS_COPY", "GPHOTOS_SOURCE", "GPHOTOS_DESTINATION", "GPHOTOS_NO_OVERWRITE", "GPHOTOS_DEBUG", "GPHOTOS_TIMEOUT", "GPHOTOS_WEB_HOST", "GPHOTOS_WEB_PORT", "GPHOTOS_WEB_USERNAME", "GPHOTOS_WEB_PASSWORD"}

func cleanEnv(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, name := range envNames {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	cleanEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Output != "liked-photos.txt" || c.Timeout != 15*time.Minute || c.WebHost != "127.0.0.1" || c.WebPort != 8080 || c.Copy || c.NoOverwrite || c.Debug {
		t.Fatalf("%+v", c)
	}
}

func TestLoadEnvironment(t *testing.T) {
	cleanEnv(t)
	values := map[string]string{"GPHOTOS_ALBUM": "https://photos.google.com/share/album", "GPHOTOS_OUTPUT": "names.txt", "GPHOTOS_PREFIX": "pre/", "GPHOTOS_SUFFIX": ".suf", "GPHOTOS_COPY": "true", "GPHOTOS_SOURCE": "source", "GPHOTOS_DESTINATION": "destination", "GPHOTOS_NO_OVERWRITE": "true", "GPHOTOS_TIMEOUT": "2m", "GPHOTOS_WEB_HOST": "localhost", "GPHOTOS_WEB_PORT": "9000", "GPHOTOS_WEB_USERNAME": "user", "GPHOTOS_WEB_PASSWORD": "password"}
	for name, value := range values {
		t.Setenv(name, value)
	}
	t.Setenv("GPHOTOS_DEBUG", "true")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.Debug {
		t.Fatal("debug logging is disabled")
	}
	if c.Album != values["GPHOTOS_ALBUM"] || c.Output != "names.txt" || c.Timeout != 2*time.Minute || c.WebPort != 9000 || c.WebHost != "localhost" || c.WebUsername != "user" || c.WebPassword != "password" {
		t.Fatalf("%+v", c)
	}
	o := c.Options()
	if !o.Copy || !o.NoOverwrite || o.Prefix != "pre/" || o.Suffix != ".suf" || o.Source != "source" || o.Destination != "destination" {
		t.Fatalf("%+v", o)
	}
}

func TestLoadDotEnv(t *testing.T) {
	cleanEnv(t)
	if err := os.WriteFile(".env", []byte("GPHOTOS_PREFIX=from-file\nGPHOTOS_SUFFIX=.file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GPHOTOS_SUFFIX", ".environment")
	c, err := Load()
	if err != nil || c.Prefix != "from-file" || c.Suffix != ".environment" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestLoadInvalidValues(t *testing.T) {
	for _, name := range []string{"GPHOTOS_COPY", "GPHOTOS_NO_OVERWRITE", "GPHOTOS_DEBUG", "GPHOTOS_WEB_PORT", "GPHOTOS_TIMEOUT"} {
		t.Run(name, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv(name, "invalid")
			if _, err := Load(); err == nil {
				t.Fatal("accepted invalid environment value")
			}
		})
	}
}
