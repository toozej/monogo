package generator

import (
	"strings"
	"testing"

	"github.com/toozej/terranotate/internal/parser"
	"github.com/toozej/terranotate/internal/validator"
)

func TestNewMarkdownGenerator(t *testing.T) {
	schema := validator.ValidationSchema{
		Global: validator.GlobalRules{
			RequiredPrefixes: []string{"@metadata"},
		},
	}

	gen := NewMarkdownGenerator(schema)
	if gen.schema.Global.RequiredPrefixes[0] != "@metadata" {
		t.Error("Schema not properly assigned to generator")
	}
}

func TestGroupResourcesByType(t *testing.T) {
	schema := validator.ValidationSchema{}
	gen := NewMarkdownGenerator(schema)

	resources := []parser.TerraformResource{
		{Type: "aws_vpc", Name: "main"},
		{Type: "aws_subnet", Name: "public"},
		{Type: "aws_vpc", Name: "secondary"},
		{Type: "aws_subnet", Name: "private"},
	}

	grouped := gen.groupResourcesByType(resources)

	if len(grouped) != 2 {
		t.Errorf("Expected 2 resource types, got %d", len(grouped))
	}

	if len(grouped["aws_vpc"]) != 2 {
		t.Errorf("Expected 2 aws_vpc resources, got %d", len(grouped["aws_vpc"]))
	}

	if len(grouped["aws_subnet"]) != 2 {
		t.Errorf("Expected 2 aws_subnet resources, got %d", len(grouped["aws_subnet"]))
	}
}

func TestGetSortedResourceTypes(t *testing.T) {
	schema := validator.ValidationSchema{}
	gen := NewMarkdownGenerator(schema)

	resourcesByType := map[string][]parser.TerraformResource{
		"zulu_resource":  {{Type: "zulu_resource", Name: "test"}},
		"alpha_resource": {{Type: "alpha_resource", Name: "test"}},
		"beta_resource":  {{Type: "beta_resource", Name: "test"}},
	}

	sorted := gen.getSortedResourceTypes(resourcesByType)

	if len(sorted) != 3 {
		t.Fatalf("Expected 3 types, got %d", len(sorted))
	}

	// Should be alphabetically sorted
	if sorted[0] != "alpha_resource" || sorted[1] != "beta_resource" || sorted[2] != "zulu_resource" {
		t.Errorf("Resources not sorted correctly: %v", sorted)
	}
}

func TestExtractFieldValue(t *testing.T) {
	schema := validator.ValidationSchema{}
	gen := NewMarkdownGenerator(schema)

	resource := parser.TerraformResource{
		Type: "aws_vpc",
		Name: "main",
		PrecedingComments: []parser.StructuredComment{
			{
				Prefix: "@metadata",
				Fields: map[string]interface{}{
					"owner": "team-a",
					"team":  "platform",
				},
			},
		},
	}

	tests := []struct {
		fieldName string
		expected  string
	}{
		{"@metadata:owner", "team-a"},
		{"@metadata:team", "platform"},
		{"@metadata:nonexistent", "-"},
		{"@unknown:field", "-"},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			got := gen.extractFieldValue(resource, tt.fieldName)
			if got != tt.expected {
				t.Errorf("extractFieldValue(%q) = %q, want %q", tt.fieldName, got, tt.expected)
			}
		})
	}
}

func TestExtractDescription(t *testing.T) {
	schema := validator.ValidationSchema{}
	gen := NewMarkdownGenerator(schema)

	tests := []struct {
		name     string
		resource parser.TerraformResource
		expected string
	}{
		{
			name: "description in @docs",
			resource: parser.TerraformResource{
				Type: "aws_vpc",
				Name: "main",
				PrecedingComments: []parser.StructuredComment{
					{
						Prefix: "@docs",
						Fields: map[string]interface{}{
							"description": "Main VPC",
						},
					},
				},
			},
			expected: "Main VPC",
		},
		{
			name: "description in @metadata",
			resource: parser.TerraformResource{
				Type: "aws_vpc",
				Name: "main",
				PrecedingComments: []parser.StructuredComment{
					{
						Prefix: "@metadata",
						Fields: map[string]interface{}{
							"description": "Secondary VPC",
						},
					},
				},
			},
			expected: "Secondary VPC",
		},
		{
			name: "no description",
			resource: parser.TerraformResource{
				Type: "aws_vpc",
				Name: "main",
				PrecedingComments: []parser.StructuredComment{
					{
						Prefix: "@metadata",
						Fields: map[string]interface{}{
							"owner": "team-a",
						},
					},
				},
			},
			expected: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gen.extractDescription(tt.resource)
			if got != tt.expected {
				t.Errorf("extractDescription() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGenerateDocumentation(t *testing.T) {
	schema := validator.ValidationSchema{
		Global: validator.GlobalRules{
			PrefixRules: map[string]validator.PrefixRule{
				"@metadata": {
					RequiredFields: []string{"owner", "team"},
				},
			},
		},
	}
	gen := NewMarkdownGenerator(schema)

	resources := []parser.TerraformResource{
		{
			Type: "aws_vpc",
			Name: "main",
			PrecedingComments: []parser.StructuredComment{
				{
					Prefix: "@metadata",
					Fields: map[string]interface{}{
						"owner": "team-networking",
						"team":  "platform",
					},
				},
			},
		},
		{
			Type: "aws_subnet",
			Name: "public",
			PrecedingComments: []parser.StructuredComment{
				{
					Prefix: "@metadata",
					Fields: map[string]interface{}{
						"owner": "team-networking",
						"team":  "platform",
					},
				},
			},
		},
	}

	markdown := gen.GenerateDocumentation("test-module", resources)

	// Check for essential elements
	if markdown == "" {
		t.Fatal("Generated markdown is empty")
	}

	// Should contain module name
	if !strings.Contains(markdown, "test-module") {
		t.Error("Markdown should contain module name")
	}

	// Should contain resource types
	if !strings.Contains(markdown, "aws_vpc") {
		t.Error("Markdown should contain aws_vpc")
	}
	if !strings.Contains(markdown, "aws_subnet") {
		t.Error("Markdown should contain aws_subnet")
	}

	// Should contain summary
	if !strings.Contains(markdown, "Total Resources") {
		t.Error("Markdown should contain total resources summary")
	}
}

func TestGenerateTableForType(t *testing.T) {
	schema := validator.ValidationSchema{
		Global: validator.GlobalRules{
			PrefixRules: map[string]validator.PrefixRule{
				"@metadata": {
					RequiredFields: []string{"owner"},
				},
			},
		},
	}
	gen := NewMarkdownGenerator(schema)

	resources := []parser.TerraformResource{
		{
			Type: "aws_vpc",
			Name: "main",
			PrecedingComments: []parser.StructuredComment{
				{
					Prefix: "@metadata",
					Fields: map[string]interface{}{
						"owner": "team-a",
					},
				},
			},
		},
	}

	table := gen.generateTableForType("aws_vpc", resources)

	// Should contain resource type header
	if !strings.Contains(table, "## aws_vpc") {
		t.Error("Table should contain resource type header")
	}

	// Should contain resource name
	if !strings.Contains(table, "`main`") {
		t.Error("Table should contain resource name")
	}

	// Should contain field value
	if !strings.Contains(table, "team-a") {
		t.Error("Table should contain field value")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		slice    []string
		item     string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
		{[]string{"test"}, "test", true},
	}

	for _, tt := range tests {
		got := contains(tt.slice, tt.item)
		if got != tt.expected {
			t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.item, got, tt.expected)
		}
	}
}
