package validator

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/toozej/monogo/apps/terranotate/internal/parser"
)

func TestNewSchemaValidator(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a test schema file
	schemaContent := `global:
  required_prefixes:
    - "@metadata"
  prefix_rules:
    "@metadata":
      required_fields:
        - owner
        - team
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	validator, err := NewSchemaValidator(fs, "/schema.yaml")
	if err != nil {
		t.Fatalf("NewSchemaValidator failed: %v", err)
	}

	if len(validator.schema.Global.RequiredPrefixes) == 0 {
		t.Error("Schema not properly loaded")
	}
}

func TestNewSchemaValidator_InvalidFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	_, err := NewSchemaValidator(fs, "/nonexistent.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent schema file")
	}
}

func TestNewSchemaValidator_InvalidYAML(t *testing.T) {
	fs := afero.NewMemMapFs()

	invalidYAML := `this is not: valid: yaml: content`
	err := afero.WriteFile(fs, "/invalid.yaml", []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid yaml: %v", err)
	}

	_, err = NewSchemaValidator(fs, "/invalid.yaml")
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestNewSchemaValidator_RejectsUnknownKeysAndInvalidPatterns(t *testing.T) {
	tests := map[string]string{
		"unknown key": `global:
  required_prefixes: []
unknown_setting: true
`,
		"invalid pattern": `field_validations:
  owner:
    type: string
    pattern: "["
`,
	}
	for name, schema := range tests {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			if err := afero.WriteFile(fs, "/schema.yaml", []byte(schema), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewSchemaValidator(fs, "/schema.yaml"); err == nil {
				t.Fatal("expected invalid schema to be rejected")
			}
		})
	}
}

func TestValidateResources_Pass(t *testing.T) {
	fs := afero.NewMemMapFs()

	schemaContent := `global:
  required_prefixes:
    - "@metadata"
  prefix_rules:
    "@metadata":
      required_fields:
        - owner
        - team
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	validator, err := NewSchemaValidator(fs, "/schema.yaml")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	if validator == nil {
		t.Fatal("validator is nil")
	}

	resources := []parser.TerraformResource{
		{
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
		},
	}

	result := validator.ValidateResources(resources)

	if !result.Passed {
		t.Error("Expected validation to pass")
	}

	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got %d", len(result.Errors))
	}
}

func TestValidateResources_MissingPrefix(t *testing.T) {
	fs := afero.NewMemMapFs()

	schemaContent := `global:
  required_prefixes:
    - "@metadata"
  prefix_rules:
    "@metadata":
      required_fields:
        - owner
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	validator, err := NewSchemaValidator(fs, "/schema.yaml")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	if validator == nil {
		t.Fatal("validator is nil")
	}

	resources := []parser.TerraformResource{
		{
			Type:              "aws_vpc",
			Name:              "main",
			StartLine:         1,
			PrecedingComments: []parser.StructuredComment{},
		},
	}

	result := validator.ValidateResources(resources)

	if result.Passed {
		t.Error("Expected validation to fail")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected at least one error")
	}

	// Check error message
	found := false
	for _, err := range result.Errors {
		if err.ResourceType == "aws_vpc" && err.ResourceName == "main" {
			found = true
			if !contains(err.Message, "Missing required comment prefix") {
				t.Errorf("Expected missing prefix error, got: %s", err.Message)
			}
		}
	}

	if !found {
		t.Error("Expected error for aws_vpc.main")
	}
}

func TestValidateResources_MissingField(t *testing.T) {
	fs := afero.NewMemMapFs()

	schemaContent := `global:
  required_prefixes:
    - "@metadata"
  prefix_rules:
    "@metadata":
      required_fields:
        - owner
        - team
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	validator, err := NewSchemaValidator(fs, "/schema.yaml")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	if validator == nil {
		t.Fatal("validator is nil")
	}

	resources := []parser.TerraformResource{
		{
			Type:      "aws_vpc",
			Name:      "main",
			StartLine: 1,
			PrecedingComments: []parser.StructuredComment{
				{
					Prefix: "@metadata",
					Fields: map[string]interface{}{
						"owner": "team-a",
						// Missing "team" field
					},
				},
			},
		},
	}

	result := validator.ValidateResources(resources)

	if result.Passed {
		t.Error("Expected validation to fail")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected at least one error")
	}

	// Check for missing field error
	found := false
	for _, err := range result.Errors {
		if contains(err.Message, "Missing required field") && contains(err.Message, "team") {
			found = true
		}
	}

	if !found {
		t.Error("Expected missing field error for 'team'")
	}
}

func TestValidateResources_ResourceTypeRules(t *testing.T) {
	fs := afero.NewMemMapFs()

	schemaContent := `global:
  required_prefixes:
    - "@metadata"
  prefix_rules:
    "@metadata":
      required_fields:
        - owner

resource_types:
  aws_vpc:
    required_prefixes:
      - "@metadata"
      - "@docs"
    prefix_rules:
      "@metadata":
        required_fields:
          - owner
          - team
      "@docs":
        required_fields:
          - description
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	validator, err := NewSchemaValidator(fs, "/schema.yaml")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	if validator == nil {
		t.Fatal("validator is nil")
	}

	// Test aws_vpc with specific rules
	vpcResources := []parser.TerraformResource{
		{
			Type:      "aws_vpc",
			Name:      "main",
			StartLine: 1,
			PrecedingComments: []parser.StructuredComment{
				{
					Prefix: "@metadata",
					Fields: map[string]interface{}{
						"owner": "team-a",
						"team":  "platform",
					},
				},
				{
					Prefix: "@docs",
					Fields: map[string]interface{}{
						"description": "Main VPC",
					},
				},
			},
		},
	}

	result := validator.ValidateResources(vpcResources)

	if !result.Passed {
		t.Errorf("Expected validation to pass for aws_vpc with all fields, got errors: %v", result.Errors)
	}

	// Test aws_subnet with only global rules
	subnetResources := []parser.TerraformResource{
		{
			Type:      "aws_subnet",
			Name:      "public",
			StartLine: 1,
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

	result = validator.ValidateResources(subnetResources)

	if !result.Passed {
		t.Error("Expected validation to pass for aws_subnet with only owner field (global rules)")
	}
}

func TestValidateResources_MultipleResources(t *testing.T) {
	fs := afero.NewMemMapFs()

	schemaContent := `global:
  required_prefixes:
    - "@metadata"
  prefix_rules:
    "@metadata":
      required_fields:
        - owner
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	validator, err := NewSchemaValidator(fs, "/schema.yaml")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	if validator == nil {
		t.Fatal("validator is nil")
	}

	resources := []parser.TerraformResource{
		{
			Type:      "aws_vpc",
			Name:      "main",
			StartLine: 1,
			PrecedingComments: []parser.StructuredComment{
				{
					Prefix: "@metadata",
					Fields: map[string]interface{}{
						"owner": "team-a",
					},
				},
			},
		},
		{
			Type:              "aws_subnet",
			Name:              "public",
			StartLine:         5,
			PrecedingComments: []parser.StructuredComment{},
		},
	}

	result := validator.ValidateResources(resources)

	if result.Passed {
		t.Error("Expected validation to fail (aws_subnet missing annotations)")
	}

	// Should have errors for aws_subnet but not aws_vpc
	vpcErrors := 0
	subnetErrors := 0

	for _, err := range result.Errors {
		if err.ResourceType == "aws_vpc" {
			vpcErrors++
		}
		if err.ResourceType == "aws_subnet" {
			subnetErrors++
		}
	}

	if vpcErrors > 0 {
		t.Errorf("Expected no errors for aws_vpc, got %d", vpcErrors)
	}

	if subnetErrors == 0 {
		t.Error("Expected errors for aws_subnet")
	}
}

func TestValidateResources_NestedFields(t *testing.T) {
	fs := afero.NewMemMapFs()

	schemaContent := `global:
  required_prefixes:
    - "@metadata"
  prefix_rules:
    "@metadata":
      required_fields:
        - owner
      nested_fields:
        tags:
          required_fields:
            - environment
            - project
`
	err := afero.WriteFile(fs, "/schema.yaml", []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("failed to write schema file: %v", err)
	}

	validator, err := NewSchemaValidator(fs, "/schema.yaml")
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}
	if validator == nil {
		t.Fatal("validator is nil")
	}

	// Test with all nested fields
	resourcesWithNested := []parser.TerraformResource{
		{
			Type:      "aws_vpc",
			Name:      "main",
			StartLine: 1,
			PrecedingComments: []parser.StructuredComment{
				{
					Prefix: "@metadata",
					Fields: map[string]interface{}{
						"owner": "team-a",
						"tags": map[string]interface{}{
							"environment": "production",
							"project":     "main-app",
						},
					},
				},
			},
		},
	}

	result := validator.ValidateResources(resourcesWithNested)

	if !result.Passed {
		t.Errorf("Expected validation to pass with nested fields, got errors: %v", result.Errors)
	}
}

func TestValidateResources_AppliesValidationToNestedLeafFields(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := `global:
  required_prefixes: ["@metadata"]
  prefix_rules:
    "@metadata":
      nested_fields:
        contact:
          required_fields: [email]
field_validations:
  email:
    type: string
    pattern: "^[^@]+@[^@]+$"
`
	if err := afero.WriteFile(fs, "/schema.yaml", []byte(schema), 0o600); err != nil {
		t.Fatal(err)
	}
	validator, err := NewSchemaValidator(fs, "/schema.yaml")
	if err != nil {
		t.Fatal(err)
	}
	resource := parser.TerraformResource{
		Type: "test", Name: "nested", StartLine: 1,
		PrecedingComments: []parser.StructuredComment{{
			Prefix: "@metadata",
			Fields: map[string]interface{}{
				"contact": map[string]interface{}{"email": "not-an-email"},
			},
		}},
	}
	result := validator.ValidateResources([]parser.TerraformResource{resource})
	if result.Passed || len(result.Errors) == 0 {
		t.Fatalf("invalid nested email passed validation: %+v", result)
	}
	if !contains(result.Errors[0].Message, "contact.email") {
		t.Fatalf("nested field path missing from error: %s", result.Errors[0].Message)
	}
}

func TestValidateResources_EnforcesExplicitZeroBounds(t *testing.T) {
	fs := afero.NewMemMapFs()
	schema := `global:
  required_prefixes: ["@metadata"]
  prefix_rules:
    "@metadata": {}
field_validations:
  replicas:
    type: integer
    min: 0
`
	if err := afero.WriteFile(fs, "/schema.yaml", []byte(schema), 0o600); err != nil {
		t.Fatal(err)
	}
	validator, err := NewSchemaValidator(fs, "/schema.yaml")
	if err != nil {
		t.Fatal(err)
	}
	resource := parser.TerraformResource{
		Type: "test", Name: "zero", StartLine: 1,
		PrecedingComments: []parser.StructuredComment{{
			Prefix: "@metadata",
			Fields: map[string]interface{}{"replicas": -1},
		}},
	}
	result := validator.ValidateResources([]parser.TerraformResource{resource})
	if result.Passed || len(result.Errors) == 0 {
		t.Fatalf("explicit min: 0 was not enforced: %+v", result)
	}
}

func TestPrintValidationResults(t *testing.T) {
	// This test just ensures the function doesn't panic
	result := ValidationResult{
		Passed: true,
		Errors: []ValidationError{},
	}

	// Should not panic
	PrintValidationResults(result)

	result = ValidationResult{
		Passed: false,
		Errors: []ValidationError{
			{
				ResourceType: "aws_vpc",
				ResourceName: "main",
				Line:         1,
				Message:      "Missing required field 'owner'",
			},
		},
	}

	// Should not panic
	PrintValidationResults(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
