package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// sampleConfig exercises the common field kinds the loader must support:
// plain strings, ints (which surface env.Parse errors), envDefault values, and
// a YAML-tagged field for the YAML layer.
type sampleConfig struct {
	Name    string `env:"SAMPLE_NAME" yaml:"name"`
	Port    int    `env:"SAMPLE_PORT" yaml:"port"`
	Verbose bool   `env:"SAMPLE_VERBOSE" yaml:"verbose"`
	Region  string `env:"SAMPLE_REGION" yaml:"region" envDefault:"us"`
}

// isolateEnv clears the sample env vars before and after a test. godotenv.Load
// mutates the process environment, so without this the variables loaded from a
// .env file in one test would leak into later tests.
func isolateEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"SAMPLE_NAME", "SAMPLE_PORT", "SAMPLE_VERBOSE", "SAMPLE_REGION"} {
		k := k
		_ = os.Unsetenv(k)
		t.Cleanup(func() { _ = os.Unsetenv(k) })
	}
}

func TestLoadEnvOnly(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	// No env vars set: defaults apply, others are zero values.
	conf, err := Load[sampleConfig]()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Name != "" || conf.Port != 0 || conf.Verbose {
		t.Errorf("expected zero values, got %+v", conf)
	}
	if conf.Region != "us" {
		t.Errorf("expected default region 'us', got %q", conf.Region)
	}

	// Env vars override.
	t.Setenv("SAMPLE_NAME", "alice")
	t.Setenv("SAMPLE_PORT", "9090")
	t.Setenv("SAMPLE_VERBOSE", "true")
	t.Setenv("SAMPLE_REGION", "eu")

	conf, err = Load[sampleConfig]()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Name != "alice" || conf.Port != 9090 || !conf.Verbose || conf.Region != "eu" {
		t.Errorf("env vars not applied, got %+v", conf)
	}
}

func TestLoadDotEnvFile(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SAMPLE_NAME=fromdotenv\nSAMPLE_PORT=1234\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	conf, err := Load[sampleConfig]()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Name != "fromdotenv" || conf.Port != 1234 {
		t.Errorf(".env values not applied, got %+v", conf)
	}
}

func TestLoadDotEnvDoesNotOverrideEnv(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SAMPLE_NAME=fromdotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	t.Setenv("SAMPLE_NAME", "fromenv")

	conf, err := Load[sampleConfig]()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Name != "fromenv" {
		t.Errorf("env should take precedence over .env, got %q", conf.Name)
	}
}

func TestLoadYAMLLayer(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	yamlBody := "name: fromyaml\nport: 4321\nverbose: true\nregion: ap\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	// YAML provides the base. Region has an envDefault, so env.Parse overwrites
	// it back to the default - this is the documented behavior.
	conf, err := Load[sampleConfig](WithYAMLFile("config.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Name != "fromyaml" || conf.Port != 4321 || !conf.Verbose {
		t.Errorf("YAML values not applied, got %+v", conf)
	}
	if conf.Region != "us" {
		t.Errorf("expected envDefault to override YAML region, got %q", conf.Region)
	}

	// Env overrides YAML.
	t.Setenv("SAMPLE_NAME", "fromenv")
	conf, err = Load[sampleConfig](WithYAMLFile("config.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Name != "fromenv" {
		t.Errorf("env should override YAML, got %q", conf.Name)
	}
}

func TestLoadYAMLOptionalMissing(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	// Optional YAML file that does not exist: not an error.
	if _, err := Load[sampleConfig](WithYAMLFile("config.yaml")); err != nil {
		t.Errorf("optional missing YAML should not error, got %v", err)
	}

	// Empty path is ignored.
	if _, err := Load[sampleConfig](WithYAMLFile("")); err != nil {
		t.Errorf("empty YAML path should be ignored, got %v", err)
	}
}

func TestLoadYAMLRequiredMissing(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	_, err := Load[sampleConfig](WithRequiredYAMLFile("config.yaml"))
	if err == nil {
		t.Fatal("expected error for missing required YAML file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("name: [unterminated\n"), 0o600); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	_, err := Load[sampleConfig](WithYAMLFile("config.yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadEnvParseError(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	// A non-numeric value for an int field makes env.Parse fail.
	t.Setenv("SAMPLE_PORT", "not-a-number")

	_, err := Load[sampleConfig]()
	if err == nil {
		t.Fatal("expected env.Parse error for invalid int")
	}
}

func TestWithoutDotEnv(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SAMPLE_NAME=fromdotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	conf, err := Load[sampleConfig](WithoutDotEnv())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if conf.Name != "" {
		t.Errorf("expected .env to be ignored, got %q", conf.Name)
	}
}

func TestLoadInto(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("SAMPLE_NAME", "into")
	var conf sampleConfig
	if err := LoadInto(&conf); err != nil {
		t.Fatalf("LoadInto returned error: %v", err)
	}
	if conf.Name != "into" {
		t.Errorf("LoadInto did not populate target, got %+v", conf)
	}
}

func TestDotEnvPathTraversalDetected(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	// Force filepathAbs to report a path outside the working directory.
	original := filepathAbs
	filepathAbs = func(path string) (string, error) {
		if filepath.Base(path) == ".env" {
			return "/somewhere/else/.env", nil
		}
		return original(path)
	}
	t.Cleanup(func() { filepathAbs = original })

	_, err := Load[sampleConfig]()
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestDotEnvGetwdError(t *testing.T) {
	isolateEnv(t)
	original := osGetwd
	osGetwd = func() (string, error) {
		return "", fmt.Errorf("boom")
	}
	t.Cleanup(func() { osGetwd = original })

	_, err := Load[sampleConfig]()
	if err == nil {
		t.Fatal("expected error when os.Getwd fails")
	}
}

func TestMustLoad(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("SAMPLE_NAME", "must")
	conf := MustLoad[sampleConfig]()
	if conf.Name != "must" {
		t.Errorf("MustLoad did not populate config, got %+v", conf)
	}
}

func TestMustLoadExit(t *testing.T) {
	isolateEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)

	exited := false
	originalExit := osExit
	osExit = func(int) { exited = true }
	t.Cleanup(func() { osExit = originalExit })

	// Invalid int triggers a load error and therefore an exit.
	t.Setenv("SAMPLE_PORT", "not-a-number")
	MustLoad[sampleConfig]()

	if !exited {
		t.Error("expected MustLoad to call osExit on load failure")
	}
}
