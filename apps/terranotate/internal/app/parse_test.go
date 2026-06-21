package app

import (
	"testing"

	"github.com/spf13/afero"
)

func TestParse(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a test Terraform file
	tfContent := `
# @metadata owner:team-a
# @docs description:This is a test VPC
resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}
`
	err := afero.WriteFile(fs, "/test.tf", []byte(tfContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test Parse function
	err = Parse(fs, "/test.tf")
	if err != nil {
		t.Errorf("Parse() failed: %v", err)
	}

	// Test non-existent file
	err = Parse(fs, "/nonexistent.tf")
	if err == nil {
		t.Error("Parse() should have failed for non-existent file")
	}
}

func TestPrintFields(t *testing.T) {
	// This test just ensures the function doesn't panic
	fields := map[string]interface{}{
		"owner": "team-a",
		"tags": map[string]interface{}{
			"env": "prod",
		},
		"list":     []interface{}{"a", "b"},
		"_content": "should be skipped",
	}

	// Should not panic
	printFields(fields, "  ")
}
