package version

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestGet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
	}{
		{name: "returns current version info"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expectedInfo := Info{
				Commit:  Commit,
				Version: Version,
				Branch:  Branch,
				BuiltAt: BuiltAt,
				Builder: Builder,
			}

			info, err := Get()
			if err != nil {
				t.Errorf("Error getting Info object: %v", err)
			}
			if info != expectedInfo {
				t.Errorf("Loaded Info object does not match expected. Got %v, expected %v", info, expectedInfo)
			}
		})
	}
}

func TestGet_WithInjectedValues(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	origBranch := Branch
	origBuiltAt := BuiltAt
	origBuilder := Builder
	defer func() {
		Version = origVersion
		Commit = origCommit
		Branch = origBranch
		BuiltAt = origBuiltAt
		Builder = origBuilder
	}()

	Version = "v1.2.3"
	Commit = "deadbeef"
	Branch = "release"
	BuiltAt = "2024-06-01T00:00:00Z"
	Builder = "goreleaser"

	info, err := Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if info.Version != "v1.2.3" {
		t.Errorf("Expected Version %q, got %q", "v1.2.3", info.Version)
	}
	if info.Commit != "deadbeef" {
		t.Errorf("Expected Commit %q, got %q", "deadbeef", info.Commit)
	}
	if info.Branch != "release" {
		t.Errorf("Expected Branch %q, got %q", "release", info.Branch)
	}
	if info.BuiltAt != "2024-06-01T00:00:00Z" {
		t.Errorf("Expected BuiltAt %q, got %q", "2024-06-01T00:00:00Z", info.BuiltAt)
	}
	if info.Builder != "goreleaser" {
		t.Errorf("Expected Builder %q, got %q", "goreleaser", info.Builder)
	}
}

func TestCommand(t *testing.T) {
	t.Parallel()
	cmd := Command()
	if cmd.Use != "version" {
		t.Errorf("Expected Use %q, got %q", "version", cmd.Use)
	}
	if cmd.Short != "Print the version." {
		t.Errorf("Expected Short %q, got %q", "Print the version.", cmd.Short)
	}
	if cmd.Long != "Print the version and build information." {
		t.Errorf("Expected Long %q, got %q", "Print the version and build information.", cmd.Long)
	}
	if cmd.RunE == nil {
		t.Error("Expected RunE to be set")
	}
}

func TestCommand_RunE(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	var result Info
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}
	if result.Version != Version {
		t.Errorf("Expected Version %q, got %q", Version, result.Version)
	}
	if result.Commit != Commit {
		t.Errorf("Expected Commit %q, got %q", Commit, result.Commit)
	}
	if result.Branch != Branch {
		t.Errorf("Expected Branch %q, got %q", Branch, result.Branch)
	}
	if result.BuiltAt != BuiltAt {
		t.Errorf("Expected BuiltAt %q, got %q", BuiltAt, result.BuiltAt)
	}
	if result.Builder != Builder {
		t.Errorf("Expected Builder %q, got %q", Builder, result.Builder)
	}
}

func TestCommand_RunE_JSONFormat(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	defer func() { Version = origVersion; Commit = origCommit }()

	Version = "v9.9.9"
	Commit = "abc123"

	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}
	if result["Version"] != "v9.9.9" {
		t.Errorf("Expected Version %q in JSON, got %v", "v9.9.9", result["Version"])
	}
	if result["Commit"] != "abc123" {
		t.Errorf("Expected Commit %q in JSON, got %v", "abc123", result["Commit"])
	}
}

func TestCommand_NoArgs(t *testing.T) {
	cmd := Command()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command should accept no args: %v", err)
	}
}

func TestInfo_Fields(t *testing.T) {
	t.Parallel()
	info := Info{
		Commit:  "abc123",
		Version: "v1.0.0",
		Branch:  "main",
		BuiltAt: "2024-01-01",
		Builder: "goreleaser",
	}
	if info.Commit != "abc123" {
		t.Errorf("Expected Commit %q, got %q", "abc123", info.Commit)
	}
	if info.Version != "v1.0.0" {
		t.Errorf("Expected Version %q, got %q", "v1.0.0", info.Version)
	}
	if info.Branch != "main" {
		t.Errorf("Expected Branch %q, got %q", "main", info.Branch)
	}
	if info.BuiltAt != "2024-01-01" {
		t.Errorf("Expected BuiltAt %q, got %q", "2024-01-01", info.BuiltAt)
	}
	if info.Builder != "goreleaser" {
		t.Errorf("Expected Builder %q, got %q", "goreleaser", info.Builder)
	}
}

func TestVersionDefaults(t *testing.T) {
	if Version != "local" {
		t.Errorf("Expected default Version %q, got %q (version may have been set via ldflags in this build)", "local", Version)
	}
}

func TestCommand_ReturnsCobraCommand(t *testing.T) {
	t.Parallel()
	cmd := Command()
	if cmd == nil {
		t.Fatal("Command() returned nil")
	}
}

func TestCommand_RunE_WithInjectedValues(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	origBranch := Branch
	origBuiltAt := BuiltAt
	origBuilder := Builder
	defer func() {
		Version = origVersion
		Commit = origCommit
		Branch = origBranch
		BuiltAt = origBuiltAt
		Builder = origBuilder
	}()

	Version = "v2.0.0"
	Commit = "cafe00"
	Branch = "feature"
	BuiltAt = "2025-12-31T23:59:59Z"
	Builder = "test-builder"

	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	var result Info
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}
	if result.Version != "v2.0.0" {
		t.Errorf("Expected Version %q, got %q", "v2.0.0", result.Version)
	}
	if result.Commit != "cafe00" {
		t.Errorf("Expected Commit %q, got %q", "cafe00", result.Commit)
	}
	if result.Branch != "feature" {
		t.Errorf("Expected Branch %q, got %q", "feature", result.Branch)
	}
	if result.BuiltAt != "2025-12-31T23:59:59Z" {
		t.Errorf("Expected BuiltAt %q, got %q", "2025-12-31T23:59:59Z", result.BuiltAt)
	}
	if result.Builder != "test-builder" {
		t.Errorf("Expected Builder %q, got %q", "test-builder", result.Builder)
	}
}

func TestCommand_RunE_OutputIsJSON(t *testing.T) {
	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	if !json.Valid(buf.Bytes()) {
		t.Errorf("Output is not valid JSON: %q", buf.String())
	}
}

func TestCommand_RunE_AllFieldsPopulated(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	origBranch := Branch
	origBuiltAt := BuiltAt
	origBuilder := Builder
	defer func() {
		Version = origVersion
		Commit = origCommit
		Branch = origBranch
		BuiltAt = origBuiltAt
		Builder = origBuilder
	}()

	Version = "v3.0.0"
	Commit = "b00c"
	Branch = "main"
	BuiltAt = "2026-01-01T00:00:00Z"
	Builder = "ci"

	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	var result Info
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}
	if result.Version != "v3.0.0" {
		t.Errorf("Expected Version %q, got %q", "v3.0.0", result.Version)
	}
	if result.Commit != "b00c" {
		t.Errorf("Expected Commit %q, got %q", "b00c", result.Commit)
	}
	if result.Branch != "main" {
		t.Errorf("Expected Branch %q, got %q", "main", result.Branch)
	}
	if result.BuiltAt != "2026-01-01T00:00:00Z" {
		t.Errorf("Expected BuiltAt %q, got %q", "2026-01-01T00:00:00Z", result.BuiltAt)
	}
	if result.Builder != "ci" {
		t.Errorf("Expected Builder %q, got %q", "ci", result.Builder)
	}
}

func TestCommand_RunE_GetError(t *testing.T) {
	origGetFunc := getFunc
	defer func() { getFunc = origGetFunc }()

	getFunc = func() (Info, error) {
		return Info{}, fmt.Errorf("get error: %w", errGetFailed)
	}

	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error when getFunc fails, got nil")
	}
}

func TestCommand_RunE_MarshalError(t *testing.T) {
	origMarshalFunc := marshalFunc
	defer func() { marshalFunc = origMarshalFunc }()

	marshalFunc = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("marshal error: %w", errMarshalFailed)
	}

	cmd := Command()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error when marshalFunc fails, got nil")
	}
}

func TestCommand_RunE_FprintlnError(t *testing.T) {
	cmd := Command()
	cmd.SetOut(&errWriter{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error from Fprintln with failing writer, got nil")
	}
}

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write error: %w", errWriteTest)
}

var errWriteTest = fmt.Errorf("underlying write failure")
var errGetFailed = fmt.Errorf("get failed")
var errMarshalFailed = fmt.Errorf("marshal failed")
