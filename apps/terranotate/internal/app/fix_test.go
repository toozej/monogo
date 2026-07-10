package app

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestFix(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Setup: Create a schema file
	schemaContent := `
global:
  required_prefixes: ["@metadata"]
  prefix_rules:
    "@metadata":
      required_fields: ["owner"]
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	// Setup: Create a directory with multiple .tf files
	err = fs.MkdirAll("/infra", 0755)
	if err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	tfContent := `resource "aws_vpc" "main" { cidr_block = "10.0.0.0/16" }`
	err = afero.WriteFile(fs, "/infra/main.tf", []byte(tfContent), 0644)
	if err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	err = afero.WriteFile(fs, "/infra/sub.tf", []byte(tfContent), 0644)
	if err != nil {
		t.Fatalf("failed to write sub.tf: %v", err)
	}

	// Test Fix on directory
	err = Fix(fs, "/infra", "/schema.yaml")
	if err != nil {
		t.Errorf("Fix() directory failed: %v", err)
	}

	// Verify backups were created
	exists, _ := afero.Exists(fs, "/infra/main.tf.bak")
	if !exists {
		t.Error("Expected backup main.tf.bak to exist")
	}

	// Test Fix on single file
	err = Fix(fs, "/infra/main.tf", "/schema.yaml")
	if err != nil {
		t.Errorf("Fix() file failed: %v", err)
	}

	// Test Fix on non-existent path
	err = Fix(fs, "/non-existent", "/schema.yaml")
	if err == nil {
		t.Error("Fix() should have failed for non-existent path")
	}
}

func TestFixSingleFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Setup: Schema
	schemaContent := `
global:
  required_prefixes: ["@metadata"]
  prefix_rules:
    "@metadata":
      required_fields: ["owner"]
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	// Setup: TF file
	tfContent := `resource "aws_vpc" "main" { cidr_block = "10.0.0.0/16" }`
	err = afero.WriteFile(fs, "/vpc.tf", []byte(tfContent), 0644)
	if err != nil {
		t.Fatalf("failed to write vpc.tf: %v", err)
	}

	// Test fixSingleFile
	fixed, count, err := fixSingleFile(fs, "/vpc.tf", "/schema.yaml")
	if err != nil {
		t.Fatalf("fixSingleFile() failed: %v", err)
	}

	if !fixed {
		t.Error("Expected file to be fixed")
	}

	if count != 1 {
		t.Errorf("Expected 1 fix, got %d", count)
	}

	// Verify content was updated
	newContent, _ := afero.ReadFile(fs, "/vpc.tf")
	if !contains(string(newContent), "@metadata") {
		t.Error("Fixed file should contain @metadata")
	}

	// Test fixSingleFile on already valid file
	fixed, _, err = fixSingleFile(fs, "/vpc.tf", "/schema.yaml")
	if err != nil {
		t.Fatalf("fixSingleFile() failed on valid file: %v", err)
	}
	if fixed {
		t.Error("Expected file not to be fixed again")
	}
}

func TestLoadSchema(t *testing.T) {
	fs := afero.NewMemMapFs()
	schemaContent := `
global:
  required_prefixes: ["@metadata"]
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	schema, err := loadSchema(fs, "/schema.yaml")
	if err != nil {
		t.Fatalf("loadSchema() failed: %v", err)
	}

	if len(schema.Global.RequiredPrefixes) != 1 {
		t.Errorf("Expected 1 required prefix, got %d", len(schema.Global.RequiredPrefixes))
	}

	// Test invalid schema file
	_, err = loadSchema(fs, "/non-existent.yaml")
	if err == nil {
		t.Error("loadSchema() should have failed for non-existent file")
	}

	// Test invalid YAML
	err = afero.WriteFile(fs, "/invalid.yaml", []byte("invalid: ["), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid yaml: %v", err)
	}
	_, err = loadSchema(fs, "/invalid.yaml")
	if err == nil {
		t.Error("loadSchema() should have failed for invalid YAML")
	}
}

func TestFindTerraformFiles(t *testing.T) {
	fs := afero.NewMemMapFs()

	err := fs.MkdirAll("/project/modules/vpc", 0755)
	if err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	err = fs.MkdirAll("/project/.terraform", 0755)
	if err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	err = afero.WriteFile(fs, "/project/main.tf", []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	err = afero.WriteFile(fs, "/project/modules/vpc/main.tf", []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	err = afero.WriteFile(fs, "/project/.terraform/ignored.tf", []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	files, err := findTerraformFiles(fs, "/project")
	if err != nil {
		t.Fatalf("findTerraformFiles() failed: %v", err)
	}

	// Should have main.tf and modules/vpc/main.tf, but not .terraform/ignored.tf
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d: %v", len(files), files)
	}
}

func TestRevertFix(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Setup: File and backup
	err := afero.WriteFile(fs, "/main.tf", []byte("new content"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	err = afero.WriteFile(fs, "/main.tf.bak", []byte("old content"), 0644)
	if err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	// Test RevertFix on file
	err = RevertFix(fs, "/main.tf")
	if err != nil {
		t.Errorf("RevertFix() failed: %v", err)
	}

	// Verify reversion
	content, _ := afero.ReadFile(fs, "/main.tf")
	if string(content) != "old content" {
		t.Errorf("Revert failed, got %q, want %q", string(content), "old content")
	}

	// Verify backup removal
	exists, _ := afero.Exists(fs, "/main.tf.bak")
	if exists {
		t.Error("Backup file should have been removed")
	}

	// Setup for directory revert
	err = fs.MkdirAll("/dir", 0755)
	if err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	err = afero.WriteFile(fs, "/dir/a.tf.bak", []byte("backup a"), 0644)
	if err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	err = RevertFix(fs, "/dir")
	if err != nil {
		t.Errorf("RevertFix() on directory failed: %v", err)
	}

	content, _ = afero.ReadFile(fs, "/dir/a.tf")
	if string(content) != "backup a" {
		t.Error("Directory revert failed")
	}

	// Test RevertFix on non-existent path
	err = RevertFix(fs, "/non-existent")
	if err == nil {
		t.Error("RevertFix() should have failed for non-existent path")
	}
}

func TestFixPreservesOriginalBackupAcrossPartialRuns(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := `global:
  required_prefixes: ["@metadata"]
  prefix_rules:
    "@metadata":
      required_fields: [owner]
field_validations:
  owner:
    type: string
    pattern: "^valid-owner$"
`
	original := "resource \"test\" \"example\" {}\n"
	if err := afero.WriteFile(fs, "/schema.yaml", []byte(schema), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(fs, "/main.tf", []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Fix(fs, "/main.tf", "/schema.yaml"); err == nil {
		t.Fatal("expected generated placeholder to require manual correction")
	}
	if info, err := fs.Stat("/main.tf"); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm() != 0o600 {
		t.Fatalf("fix changed file mode to %o", info.Mode().Perm())
	}
	if err := Fix(fs, "/main.tf", "/schema.yaml"); err == nil {
		t.Fatal("expected second partial fix to remain nonzero")
	}
	backup, err := afero.ReadFile(fs, "/main.tf.bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatalf("backup was overwritten: %q", backup)
	}
	if err := RevertFix(fs, "/main.tf"); err != nil {
		t.Fatal(err)
	}
	restored, err := afero.ReadFile(fs, "/main.tf")
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Fatalf("restored content = %q, want original", restored)
	}
}

func TestDirectoryRevertIgnoresUnrelatedBackupFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll("/dir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(fs, "/dir/notes.bak", []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(fs, "/dir/main.tf.bak", []byte("terraform"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RevertFix(fs, "/dir"); err != nil {
		t.Fatal(err)
	}
	if exists, _ := afero.Exists(fs, "/dir/notes"); exists {
		t.Fatal("unrelated .bak file was restored")
	}
	if exists, _ := afero.Exists(fs, "/dir/notes.bak"); !exists {
		t.Fatal("unrelated .bak file was removed")
	}
}

func TestFixReturnsBatchFailures(t *testing.T) {
	fs := afero.NewMemMapFs()
	if err := afero.WriteFile(fs, "/schema.yaml", []byte(`global: {required_prefixes: []}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fs.MkdirAll("/infra", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(fs, "/infra/valid.tf", []byte(`resource "test" "valid" {}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(fs, "/infra/invalid.tf", []byte(`resource "test" "invalid" {`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Fix(fs, "/infra", "/schema.yaml"); err == nil {
		t.Fatal("expected batch fix to return the per-file parse error")
	}
}

// Helper for tests
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
