package validator

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/afero"
	"github.com/toozej/terranotate/internal/parser"
	"gopkg.in/yaml.v3"
)

// ValidationSchema represents the complete validation schema
type ValidationSchema struct {
	Global           GlobalRules                `yaml:"global"`
	ResourceTypes    map[string]ResourceRules   `yaml:"resource_types"`
	FieldValidations map[string]FieldValidation `yaml:"field_validations"`
}

// GlobalRules defines rules that apply to all resources
type GlobalRules struct {
	RequiredPrefixes []string              `yaml:"required_prefixes"`
	PrefixRules      map[string]PrefixRule `yaml:"prefix_rules"`
}

// ResourceRules defines rules for a specific resource type
type ResourceRules struct {
	RequiredPrefixes []string              `yaml:"required_prefixes"`
	PrefixRules      map[string]PrefixRule `yaml:"prefix_rules"`
}

// PrefixRule defines validation rules for a comment prefix
type PrefixRule struct {
	RequiredFields []string              `yaml:"required_fields"`
	OptionalFields []string              `yaml:"optional_fields"`
	NestedFields   map[string]NestedRule `yaml:"nested_fields"`
}

// NestedRule defines validation for nested field structures
type NestedRule struct {
	RequiredFields []string `yaml:"required_fields"`
	OptionalFields []string `yaml:"optional_fields"`
}

// FieldValidation defines type and value constraints for fields
type FieldValidation struct {
	Type          string   `yaml:"type"`
	AllowedValues []string `yaml:"allowed_values"`
	Pattern       string   `yaml:"pattern"`
	MinLength     int      `yaml:"min_length"`
	Min           float64  `yaml:"min"`
	Max           float64  `yaml:"max"`
	MinItems      int      `yaml:"min_items"`
}

// ValidationError represents a validation failure
type ValidationError struct {
	ResourceType string
	ResourceName string
	Line         int
	Severity     string // "error" or "warning"
	Message      string
}

// ValidationResult contains all validation errors
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []ValidationError
	Passed   bool
}

// SchemaValidator handles schema-based validation
type SchemaValidator struct {
	fs     afero.Fs
	schema ValidationSchema
}

// NewSchemaValidator creates a new validator from a schema file
func NewSchemaValidator(fs afero.Fs, schemaFile string) (*SchemaValidator, error) {
	if fs == nil {
		fs = afero.NewOsFs()
	}

	// #nosec G304 - Schema file is provided by user via CLI, using afero abstraction
	f, err := fs.Open(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file content: %w", err)
	}

	var schema ValidationSchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	return &SchemaValidator{fs: fs, schema: schema}, nil
}

// ValidateResources validates all resources against the schema
func (sv *SchemaValidator) ValidateResources(resources []parser.TerraformResource) ValidationResult {
	result := ValidationResult{
		Passed: true,
	}

	for _, resource := range resources {
		errors := sv.validateResource(resource)
		result.Errors = append(result.Errors, errors...)
		if len(errors) > 0 {
			result.Passed = false
		}
	}

	return result
}

// validateResource validates a single resource
func (sv *SchemaValidator) validateResource(resource parser.TerraformResource) []ValidationError {
	var errors []ValidationError

	// Get applicable rules (resource-specific or global)
	rules := sv.getApplicableRules(resource.Type)

	// Check required prefixes
	errors = append(errors, sv.checkRequiredPrefixes(resource, rules)...)

	// Validate each prefix's fields
	for prefix, prefixRule := range rules.PrefixRules {
		comments := resource.GetCommentsByPrefix(prefix)
		if len(comments) == 0 {
			// Only error if this prefix is required
			if sv.isPrefixRequired(prefix, rules) {
				continue // Already reported in checkRequiredPrefixes
			}
			continue
		}

		for _, comment := range comments {
			errors = append(errors, sv.validatePrefixFields(resource, comment, prefix, prefixRule)...)
		}
	}

	return errors
}

// getApplicableRules returns resource-specific rules or falls back to global
func (sv *SchemaValidator) getApplicableRules(resourceType string) ResourceRules {
	if rules, exists := sv.schema.ResourceTypes[resourceType]; exists {
		return rules
	}

	// Return global rules as ResourceRules
	return ResourceRules{
		RequiredPrefixes: sv.schema.Global.RequiredPrefixes,
		PrefixRules:      sv.schema.Global.PrefixRules,
	}
}

// isPrefixRequired checks if a prefix is required
func (sv *SchemaValidator) isPrefixRequired(prefix string, rules ResourceRules) bool {
	for _, req := range rules.RequiredPrefixes {
		if req == prefix {
			return true
		}
	}
	return false
}

// checkRequiredPrefixes validates that all required prefixes are present
func (sv *SchemaValidator) checkRequiredPrefixes(resource parser.TerraformResource, rules ResourceRules) []ValidationError {
	var errors []ValidationError

	for _, requiredPrefix := range rules.RequiredPrefixes {
		comments := resource.GetCommentsByPrefix(requiredPrefix)
		if len(comments) == 0 {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         resource.StartLine,
				Severity:     "error",
				Message:      fmt.Sprintf("Missing required comment prefix: %s", requiredPrefix),
			})
		}
	}

	return errors
}

// validatePrefixFields validates fields within a comment prefix
func (sv *SchemaValidator) validatePrefixFields(resource parser.TerraformResource, comment parser.StructuredComment, prefix string, rule PrefixRule) []ValidationError {
	var errors []ValidationError

	// Check required fields
	for _, requiredField := range rule.RequiredFields {
		if !sv.fieldExists(comment.Fields, requiredField) {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Missing required field '%s'", prefix, requiredField),
			})
		}
	}

	// Validate nested fields
	for nestedPath, nestedRule := range rule.NestedFields {
		errors = append(errors, sv.validateNestedFields(resource, comment, prefix, nestedPath, nestedRule)...)
	}

	// Validate field values
	errors = append(errors, sv.validateFieldValues(resource, comment, prefix)...)

	return errors
}

// validateNestedFields validates nested field structures
func (sv *SchemaValidator) validateNestedFields(resource parser.TerraformResource, comment parser.StructuredComment, prefix, nestedPath string, rule NestedRule) []ValidationError {
	var errors []ValidationError

	// Get the nested object
	parts := strings.Split(nestedPath, ".")
	current := comment.Fields

	for _, part := range parts {
		if val, exists := current[part]; exists {
			if nested, ok := val.(map[string]interface{}); ok {
				current = nested
			} else {
				// Not a nested object, can't validate further
				return errors
			}
		} else {
			// Nested path doesn't exist - check if any required fields
			if len(rule.RequiredFields) > 0 {
				errors = append(errors, ValidationError{
					ResourceType: resource.Type,
					ResourceName: resource.Name,
					Line:         comment.Line,
					Severity:     "error",
					Message:      fmt.Sprintf("%s: Missing nested structure '%s'", prefix, nestedPath),
				})
			}
			return errors
		}
	}

	// Check required fields in the nested object
	for _, requiredField := range rule.RequiredFields {
		// Support dot notation in required fields (e.g., "primary.email")
		if strings.Contains(requiredField, ".") {
			fullPath := nestedPath + "." + requiredField
			if !sv.fieldExists(comment.Fields, fullPath) {
				errors = append(errors, ValidationError{
					ResourceType: resource.Type,
					ResourceName: resource.Name,
					Line:         comment.Line,
					Severity:     "error",
					Message:      fmt.Sprintf("%s: Missing required nested field '%s'", prefix, fullPath),
				})
			}
		} else {
			if _, exists := current[requiredField]; !exists {
				errors = append(errors, ValidationError{
					ResourceType: resource.Type,
					ResourceName: resource.Name,
					Line:         comment.Line,
					Severity:     "error",
					Message:      fmt.Sprintf("%s: Missing required field '%s.%s'", prefix, nestedPath, requiredField),
				})
			}
		}
	}

	return errors
}

// fieldExists checks if a field exists (supports dot notation)
func (sv *SchemaValidator) fieldExists(fields map[string]interface{}, fieldPath string) bool {
	parts := strings.Split(fieldPath, ".")
	current := fields

	for i, part := range parts {
		if val, exists := current[part]; exists {
			if i == len(parts)-1 {
				// Found the final field
				return true
			}
			// Navigate deeper
			if nested, ok := val.(map[string]interface{}); ok {
				current = nested
			} else {
				return false
			}
		} else {
			return false
		}
	}

	return false
}

// validateFieldValues validates field value constraints
func (sv *SchemaValidator) validateFieldValues(resource parser.TerraformResource, comment parser.StructuredComment, prefix string) []ValidationError {
	var errors []ValidationError

	for fieldName, fieldValue := range comment.Fields {
		if fieldName == "_content" {
			continue
		}

		// Get validation rules for this field
		validation, exists := sv.schema.FieldValidations[fieldName]
		if !exists {
			continue // No validation rules defined
		}

		errors = append(errors, sv.validateFieldValue(resource, comment, prefix, fieldName, fieldValue, validation)...)
	}

	return errors
}

// validateFieldValue validates a single field value
func (sv *SchemaValidator) validateFieldValue(resource parser.TerraformResource, comment parser.StructuredComment, prefix, fieldName string, fieldValue interface{}, validation FieldValidation) []ValidationError {
	var errors []ValidationError

	// Type validation
	switch validation.Type {
	case "string":
		strVal, ok := fieldValue.(string)
		if !ok {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' must be a string, got %T", prefix, fieldName, fieldValue),
			})
			return errors
		}

		// Pattern validation
		if validation.Pattern != "" {
			matched, err := regexp.MatchString(validation.Pattern, strVal)
			if err == nil && !matched {
				errors = append(errors, ValidationError{
					ResourceType: resource.Type,
					ResourceName: resource.Name,
					Line:         comment.Line,
					Severity:     "error",
					Message:      fmt.Sprintf("%s: Field '%s' value '%s' does not match required pattern '%s'", prefix, fieldName, strVal, validation.Pattern),
				})
			}
		}

		// Allowed values
		if len(validation.AllowedValues) > 0 {
			found := false
			for _, allowed := range validation.AllowedValues {
				if strVal == allowed {
					found = true
					break
				}
			}
			if !found {
				errors = append(errors, ValidationError{
					ResourceType: resource.Type,
					ResourceName: resource.Name,
					Line:         comment.Line,
					Severity:     "error",
					Message:      fmt.Sprintf("%s: Field '%s' value '%s' not in allowed values: %v", prefix, fieldName, strVal, validation.AllowedValues),
				})
			}
		}

		// Min length
		if validation.MinLength > 0 && len(strVal) < validation.MinLength {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' must be at least %d characters, got %d", prefix, fieldName, validation.MinLength, len(strVal)),
			})
		}

	case "boolean":
		if _, ok := fieldValue.(bool); !ok {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' must be a boolean, got %T", prefix, fieldName, fieldValue),
			})
		}

	case "integer":
		intVal, ok := fieldValue.(int)
		if !ok {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' must be an integer, got %T", prefix, fieldName, fieldValue),
			})
			return errors
		}

		if validation.Min != 0 && float64(intVal) < validation.Min {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' value %d is below minimum %v", prefix, fieldName, intVal, validation.Min),
			})
		}

		if validation.Max != 0 && float64(intVal) > validation.Max {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' value %d exceeds maximum %v", prefix, fieldName, intVal, validation.Max),
			})
		}

	case "float":
		floatVal, ok := fieldValue.(float64)
		if !ok {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' must be a float, got %T", prefix, fieldName, fieldValue),
			})
			return errors
		}

		if validation.Min != 0 && floatVal < validation.Min {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' value %.2f is below minimum %.2f", prefix, fieldName, floatVal, validation.Min),
			})
		}

		if validation.Max != 0 && floatVal > validation.Max {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' value %.2f exceeds maximum %.2f", prefix, fieldName, floatVal, validation.Max),
			})
		}

	case "array":
		arrVal, ok := fieldValue.([]interface{})
		if !ok {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' must be an array, got %T", prefix, fieldName, fieldValue),
			})
			return errors
		}

		if validation.MinItems > 0 && len(arrVal) < validation.MinItems {
			errors = append(errors, ValidationError{
				ResourceType: resource.Type,
				ResourceName: resource.Name,
				Line:         comment.Line,
				Severity:     "error",
				Message:      fmt.Sprintf("%s: Field '%s' must have at least %d items, got %d", prefix, fieldName, validation.MinItems, len(arrVal)),
			})
		}
	}

	return errors
}

// PrintValidationResults prints validation results in a user-friendly format
func PrintValidationResults(result ValidationResult) {
	if result.Passed {
		fmt.Println("\n✅ All validation checks passed!")
		return
	}

	fmt.Println("\n❌ Validation failed with the following errors:")
	fmt.Println(strings.Repeat("=", 80))

	// Group errors by resource
	resourceErrors := make(map[string][]ValidationError)
	for _, err := range result.Errors {
		key := fmt.Sprintf("%s.%s", err.ResourceType, err.ResourceName)
		resourceErrors[key] = append(resourceErrors[key], err)
	}

	// Print errors grouped by resource
	for resource, errors := range resourceErrors {
		fmt.Printf("\n🔴 %s\n", resource)
		fmt.Println(strings.Repeat("-", 80))

		for _, err := range errors {
			severity := "ERROR"
			icon := "❌"
			if err.Severity == "warning" {
				severity = "WARNING"
				icon = "⚠️"
			}

			fmt.Printf("  %s [%s] Line %d\n", icon, severity, err.Line)
			fmt.Printf("     %s\n\n", err.Message)
		}
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\nTotal errors: %d\n", len(result.Errors))
}
