package fixer

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/toozej/terranotate/internal/parser"
	"github.com/toozej/terranotate/internal/validator"
)

func TestNewCommentFixer(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := validator.ValidationSchema{
		Global: validator.GlobalRules{
			RequiredPrefixes: []string{"@metadata"},
		},
	}

	fixer := NewCommentFixer(fs, schema)
	if fixer.fs == nil {
		t.Error("Filesystem not properly assigned")
	}
	if len(fixer.schema.Global.RequiredPrefixes) == 0 {
		t.Error("Schema not properly assigned")
	}
}

func TestGroupErrorsByResource(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := validator.ValidationSchema{}
	fixer := NewCommentFixer(fs, schema)

	errors := []validator.ValidationError{
		{ResourceType: "aws_vpc", ResourceName: "main", Message: "Missing required comment prefix: @metadata"},
		{ResourceType: "aws_vpc", ResourceName: "main", Message: "@metadata: Missing required field 'owner'"},
		{ResourceType: "aws_subnet", ResourceName: "public", Message: "Missing required comment prefix: @metadata"},
	}

	grouped := fixer.groupErrorsByResource(errors)

	if len(grouped) != 2 {
		t.Errorf("Expected 2 resources with errors, got %d", len(grouped))
	}

	vpcErrors := grouped["aws_vpc.main"]
	if len(vpcErrors) != 2 {
		t.Errorf("Expected 2 errors for aws_vpc.main, got %d", len(vpcErrors))
	}

	subnetErrors := grouped["aws_subnet.public"]
	if len(subnetErrors) != 1 {
		t.Errorf("Expected 1 error for aws_subnet.public, got %d", len(subnetErrors))
	}
}

func TestBuildCommentBlock(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := validator.ValidationSchema{
		Global: validator.GlobalRules{
			PrefixRules: map[string]validator.PrefixRule{
				"@metadata": {
					RequiredFields: []string{"owner", "team"},
					OptionalFields: []string{"purpose"},
				},
			},
		},
	}
	fixer := NewCommentFixer(fs, schema)

	fixes := []CommentFix{
		{
			Prefix: "@metadata",
			Fields: map[string]string{
				"owner":   "CHANGEME",
				"team":    "CHANGEME",
				"purpose": "CHANGEME",
			},
		},
	}

	lines := fixer.buildCommentBlock(fixes)

	if len(lines) == 0 {
		t.Fatal("buildCommentBlock returned no lines")
	}

	// Should contain prefix
	if !strings.Contains(lines[0], "@metadata") {
		t.Error("Comment block should contain @metadata prefix")
	}

	// Should contain fields
	commentLine := lines[0]
	if !strings.Contains(commentLine, "owner:") {
		t.Error("Comment block should contain owner field")
	}
	if !strings.Contains(commentLine, "team:") {
		t.Error("Comment block should contain team field")
	}
}

func TestGetPlaceholderValue(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := validator.ValidationSchema{}
	fixer := NewCommentFixer(fs, schema)

	tests := []struct {
		field    string
		expected string
	}{
		{"owner", "CHANGEME"},
		{"team", "CHANGEME"},
		{"purpose", "CHANGEME"},
		{"unknown_field", "CHANGEME"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got := fixer.getPlaceholderValue(tt.field)
			if got != tt.expected {
				t.Errorf("getPlaceholderValue(%q) = %q, want %q", tt.field, got, tt.expected)
			}
		})
	}
}

func TestHasValidComments(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := validator.ValidationSchema{}
	fixer := NewCommentFixer(fs, schema)

	tests := []struct {
		name     string
		resource parser.TerraformResource
		errors   []validator.ValidationError
		expected bool
	}{
		{
			name: "resource with valid comments",
			resource: parser.TerraformResource{
				Type: "aws_vpc",
				Name: "main",
				PrecedingComments: []parser.StructuredComment{
					{
						Prefix: "@metadata",
						Raw:    "# @metadata owner:team-a team:platform",
						Fields: map[string]interface{}{
							"owner": "team-a",
							"team":  "platform",
						},
					},
				},
			},
			errors:   []validator.ValidationError{},
			expected: true,
		},
		{
			name: "resource without comments",
			resource: parser.TerraformResource{
				Type:              "aws_vpc",
				Name:              "main",
				PrecedingComments: []parser.StructuredComment{},
			},
			errors: []validator.ValidationError{
				{ResourceType: "aws_vpc", ResourceName: "main", Message: "Missing required comment prefix: @metadata"},
			},
			expected: false,
		},
		{
			name: "resource with placeholder comments",
			resource: parser.TerraformResource{
				Type: "aws_vpc",
				Name: "main",
				PrecedingComments: []parser.StructuredComment{
					{
						Prefix: "@metadata",
						Raw:    "# @metadata owner:CHANGEME team:CHANGEME",
						Fields: map[string]interface{}{
							"owner": "CHANGEME",
							"team":  "CHANGEME",
						},
					},
				},
			},
			errors:   []validator.ValidationError{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixer.hasValidComments(tt.resource, tt.errors)
			if got != tt.expected {
				t.Errorf("hasValidComments() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFindInsertionPoint(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := validator.ValidationSchema{}
	fixer := NewCommentFixer(fs, schema)

	tests := []struct {
		name              string
		lines             []string
		resourceStartLine int
		expected          int
	}{
		{
			name: "resource with no preceding comments",
			lines: []string{
				"",
				"resource \"aws_vpc\" \"main\" {",
				"  cidr_block = \"10.0.0.0/16\"",
				"}",
			},
			resourceStartLine: 1,
			expected:          0, // inserts at blank line before resource
		},
		{
			name: "resource with user comment",
			lines: []string{
				"",
				"# This is a user comment",
				"resource \"aws_vpc\" \"main\" {",
				"  cidr_block = \"10.0.0.0/16\"",
				"}",
			},
			resourceStartLine: 2,
			expected:          2, // inserts after user comment
		},
		{
			name: "resource with managed comment",
			lines: []string{
				"",
				"# @metadata owner:team-a",
				"resource \"aws_vpc\" \"main\" {",
				"  cidr_block = \"10.0.0.0/16\"",
				"}",
			},
			resourceStartLine: 2,
			expected:          0, // skips managed comment and inserts at blank line
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixer.findInsertionPoint(tt.lines, tt.resourceStartLine)
			if got != tt.expected {
				t.Errorf("findInsertionPoint() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestInsertLines(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := validator.ValidationSchema{}
	fixer := NewCommentFixer(fs, schema)

	original := []string{"line1", "line2", "line4"}
	toInsert := []string{"line3"}

	result := fixer.insertLines(original, 2, toInsert)

	expected := []string{"line1", "line2", "line3", "line4"}
	if len(result) != len(expected) {
		t.Fatalf("Expected %d lines, got %d", len(expected), len(result))
	}

	for i, line := range expected {
		if result[i] != line {
			t.Errorf("Line %d: expected %q, got %q", i, line, result[i])
		}
	}
}

func TestFixFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a test Terraform file
	tfContent := `resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}
`
	err := afero.WriteFile(fs, "/test.tf", []byte(tfContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	schema := validator.ValidationSchema{
		Global: validator.GlobalRules{
			RequiredPrefixes: []string{"@metadata"},
			PrefixRules: map[string]validator.PrefixRule{
				"@metadata": {
					RequiredFields: []string{"owner", "team"},
				},
			},
		},
	}

	fixer := NewCommentFixer(fs, schema)

	resources := []parser.TerraformResource{
		{
			Type:      "aws_vpc",
			Name:      "main",
			StartLine: 0,
			EndLine:   2,
		},
	}

	errors := []validator.ValidationError{
		{
			ResourceType: "aws_vpc",
			ResourceName: "main",
			Message:      "Missing required comment prefix: @metadata",
		},
	}

	fixedContent, fixCount, err := fixer.FixFile("/test.tf", resources, errors)
	if err != nil {
		t.Fatalf("FixFile failed: %v", err)
	}

	if fixCount == 0 {
		t.Error("Expected at least one fix to be applied")
	}

	if !strings.Contains(fixedContent, "@metadata") {
		t.Error("Fixed content should contain @metadata comment")
	}

	if !strings.Contains(fixedContent, "owner:") {
		t.Error("Fixed content should contain owner field")
	}

	if !strings.Contains(fixedContent, "team:") {
		t.Error("Fixed content should contain team field")
	}
}

func TestCopyFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create source file
	srcContent := "test content"
	err := afero.WriteFile(fs, "/source.txt", []byte(srcContent), 0644)
	if err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	// Copy file
	err = CopyFile(fs, "/source.txt", "/dest.txt")
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify destination file
	destContent, err := afero.ReadFile(fs, "/dest.txt")
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(destContent) != srcContent {
		t.Errorf("Destination content = %q, want %q", string(destContent), srcContent)
	}
}

func TestGetApplicableRules(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := validator.ValidationSchema{
		Global: validator.GlobalRules{
			RequiredPrefixes: []string{"@metadata"},
			PrefixRules: map[string]validator.PrefixRule{
				"@metadata": {
					RequiredFields: []string{"owner"},
				},
			},
		},
		ResourceTypes: map[string]validator.ResourceRules{
			"aws_vpc": {
				RequiredPrefixes: []string{"@metadata", "@docs"},
				PrefixRules: map[string]validator.PrefixRule{
					"@metadata": {
						RequiredFields: []string{"owner", "team"},
					},
				},
			},
		},
	}

	fixer := NewCommentFixer(fs, schema)

	// Test resource-specific rules
	vpcRules := fixer.getApplicableRules("aws_vpc")
	if len(vpcRules.RequiredPrefixes) != 2 {
		t.Errorf("Expected 2 required prefixes for aws_vpc, got %d", len(vpcRules.RequiredPrefixes))
	}

	// Test fallback to global rules
	subnetRules := fixer.getApplicableRules("aws_subnet")
	if len(subnetRules.RequiredPrefixes) != 1 {
		t.Errorf("Expected 1 required prefix (global), got %d", len(subnetRules.RequiredPrefixes))
	}
}
