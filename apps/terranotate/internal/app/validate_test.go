package app

import (
	"testing"

	"github.com/spf13/afero"
)

func TestValidate(t *testing.T) {
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
	tfContent := `
# @metadata owner:team-a
resource "aws_vpc" "main" { cidr_block = "10.0.0.0/16" }
`
	err = afero.WriteFile(fs, "/main.tf", []byte(tfContent), 0644)
	if err != nil {
		t.Fatalf("failed to write main.tf: %v", err)
	}

	// Test Validate
	err = Validate(fs, "/main.tf", "/schema.yaml")
	if err != nil {
		t.Errorf("Validate() failed: %v", err)
	}

	// Test Validate with failure
	err = afero.WriteFile(fs, "/invalid.tf", []byte(`resource "aws_vpc" "bad" {}`), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid.tf: %v", err)
	}
	err = Validate(fs, "/invalid.tf", "/schema.yaml")
	if err == nil {
		t.Error("Validate() should have failed for invalid TF")
	}
}

func TestValidateAuto(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Setup: Schema
	schemaContent := `global: { required_prefixes: ["@metadata"] }`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	// Setup: Single file
	err = afero.WriteFile(fs, "/single.tf", []byte(`# @metadata ok:true`+"\n"+`resource "a" "b" {}`), 0644)
	if err != nil {
		t.Fatalf("failed to write single.tf: %v", err)
	}

	// Setup: Module
	err = fs.MkdirAll("/module/modules", 0755)
	if err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	err = afero.WriteFile(fs, "/module/main.tf", []byte(`# @metadata ok:true`+"\n"+`resource "a" "b" {}`), 0644)
	if err != nil {
		t.Fatalf("failed to write module/main.tf: %v", err)
	}

	// Setup: Workspace
	err = fs.MkdirAll("/workspace/environments/prod", 0755)
	if err != nil {
		t.Fatalf("failed to create workspace dir: %v", err)
	}
	err = afero.WriteFile(fs, "/workspace/environments/prod/main.tf", []byte(`# @metadata ok:true`+"\n"+`resource "a" "b" {}`), 0644)
	if err != nil {
		t.Fatalf("failed to write workspace main.tf: %v", err)
	}

	// Test ValidateAuto
	if err := ValidateAuto(fs, "/single.tf", "/schema.yaml"); err != nil {
		t.Errorf("ValidateAuto() single file failed: %v", err)
	}

	if err := ValidateAuto(fs, "/module", "/schema.yaml"); err != nil {
		t.Errorf("ValidateAuto() module failed: %v", err)
	}

	if err := ValidateAuto(fs, "/workspace", "/schema.yaml"); err != nil {
		t.Errorf("ValidateAuto() workspace failed: %v", err)
	}
}

func TestDetectDirectoryType(t *testing.T) {
	fs := afero.NewMemMapFs()

	// 1. Module (contains modules/ dir)
	err := fs.MkdirAll("/my-module/modules", 0755)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if detectDirectoryType(fs, "/my-module") != "module" {
		t.Error("Expected /my-module to be detected as module")
	}

	// 2. Module (is inside modules/ dir)
	err = fs.MkdirAll("/project/modules/my-submodule", 0755)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if detectDirectoryType(fs, "/project/modules/my-submodule") != "module" {
		t.Error("Expected submodule to be detected as module")
	}

	// 3. Workspace (contains environments/ dir)
	err = fs.MkdirAll("/my-workspace/environments/dev", 0755)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if detectDirectoryType(fs, "/my-workspace") != "workspace" {
		t.Error("Expected /my-workspace to be detected as workspace")
	}

	// 4. Simple directory
	err = fs.MkdirAll("/simple", 0755)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if detectDirectoryType(fs, "/simple") != "directory" {
		t.Error("Expected /simple to be detected as directory")
	}
}

func TestValidateDirectory(t *testing.T) {
	fs := afero.NewMemMapFs()

	schemaContent := `global: { required_prefixes: ["@metadata"] }`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	err = fs.MkdirAll("/dir", 0755)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	err = afero.WriteFile(fs, "/dir/main.tf", []byte(`# @metadata ok:true`+"\n"+`resource "a" "b" {}`), 0644)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	err = validateDirectory(fs, "/dir", "/schema.yaml")
	if err != nil {
		t.Errorf("validateDirectory() failed: %v", err)
	}

	// Test with no .tf files
	err = fs.Mkdir("/empty", 0755)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	err = validateDirectory(fs, "/empty", "/schema.yaml")
	if err == nil {
		t.Error("validateDirectory() should have failed for empty directory")
	}
}

func TestValidateModule(t *testing.T) {
	fs := afero.NewMemMapFs()
	schemaContent := `global: { required_prefixes: ["@metadata"] }`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	err = fs.MkdirAll("/module/modules/sub", 0755)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	err = afero.WriteFile(fs, "/module/main.tf", []byte(`# @metadata ok:true`+"\n"+`resource "a" "b" {}`), 0644)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	err = afero.WriteFile(fs, "/module/modules/sub/main.tf", []byte(`# @metadata ok:true`+"\n"+`resource "c" "d" {}`), 0644)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	err = ValidateModule(fs, "/module", "/schema.yaml")
	if err != nil {
		t.Errorf("ValidateModule() failed: %v", err)
	}
}

func TestValidateWorkspace(t *testing.T) {
	fs := afero.NewMemMapFs()
	schemaContent := `global: { required_prefixes: ["@metadata"] }`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	err = fs.MkdirAll("/workspace/env/prod", 0755)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	err = afero.WriteFile(fs, "/workspace/env/prod/main.tf", []byte(`# @metadata ok:true`+"\n"+`resource "a" "b" {}`), 0644)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	err = ValidateWorkspace(fs, "/workspace", "/schema.yaml")
	if err != nil {
		t.Errorf("ValidateWorkspace() failed: %v", err)
	}
}
