package app

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestGenerate(t *testing.T) {
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

	// Test Generate to stdout (outputFile = "")
	// We check if it doesn't fail
	err = Generate(fs, "/main.tf", "/schema.yaml", "")
	if err != nil {
		t.Errorf("Generate() to stdout failed: %v", err)
	}

	// Test Generate to file
	err = Generate(fs, "/main.tf", "/schema.yaml", "/output.md")
	if err != nil {
		t.Errorf("Generate() to file failed: %v", err)
	}

	exists, _ := afero.Exists(fs, "/output.md")
	if !exists {
		t.Error("Expected output.md to exist")
	}

	// Test Generate on directory
	err = fs.MkdirAll("/infra", 0755)
	if err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	err = afero.WriteFile(fs, "/infra/vpc.tf", []byte(tfContent), 0644)
	if err != nil {
		t.Fatalf("failed to write vpc.tf: %v", err)
	}

	err = Generate(fs, "/infra", "/schema.yaml", "/infra_doc.md")
	if err != nil {
		t.Errorf("Generate() on directory failed: %v", err)
	}

	// Test failure cases
	err = Generate(fs, "/non-existent", "/schema.yaml", "")
	if err == nil {
		t.Error("Generate() should have failed for non-existent path")
	}

	err = Generate(fs, "/main.tf", "/non-existent.yaml", "")
	if err == nil {
		t.Error("Generate() should have failed for non-existent schema")
	}

	// Test no resources found
	err = afero.WriteFile(fs, "/empty.tf", []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write empty.tf: %v", err)
	}
	err = Generate(fs, "/empty.tf", "/schema.yaml", "")
	if err == nil {
		t.Error("Generate() should have failed for file with no resources")
	}
}

func TestFindTerraformFilesForGeneration(t *testing.T) {
	fs := afero.NewMemMapFs()

	err := fs.MkdirAll("/project/sub", 0755)
	if err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	err = afero.WriteFile(fs, "/project/main.tf", []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	err = afero.WriteFile(fs, "/project/sub/other.tf", []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	err = afero.WriteFile(fs, "/project/main_test.tf", []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	files, err := findTerraformFilesForGeneration(fs, "/project")
	if err != nil {
		t.Fatalf("findTerraformFilesForGeneration() failed: %v", err)
	}

	// Should have main.tf and sub/other.tf, but not main_test.tf
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d: %v", len(files), files)
	}

	for _, f := range files {
		if strings.HasSuffix(f, "_test.tf") {
			t.Errorf("Found ignored test file: %s", f)
		}
	}
}
