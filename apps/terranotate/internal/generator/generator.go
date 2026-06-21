package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/toozej/terranotate/internal/parser"
	"github.com/toozej/terranotate/internal/validator"
)

// MarkdownGenerator generates markdown documentation from resources
type MarkdownGenerator struct {
	schema validator.ValidationSchema
}

// NewMarkdownGenerator creates a new markdown generator
func NewMarkdownGenerator(schema validator.ValidationSchema) *MarkdownGenerator {
	return &MarkdownGenerator{
		schema: schema,
	}
}

// GenerateDocumentation generates a markdown document for the given resources
func (mg *MarkdownGenerator) GenerateDocumentation(moduleName string, resources []parser.TerraformResource) string {
	var sb strings.Builder

	// Header
	fmt.Fprintf(&sb, "# %s - Resource Documentation\n\n", moduleName)
	sb.WriteString("This document provides an overview of all Terraform resources with their metadata annotations.\n\n")

	// Group resources by type
	resourcesByType := mg.groupResourcesByType(resources)

	// Generate a table for each resource type
	for _, resourceType := range mg.getSortedResourceTypes(resourcesByType) {
		typeResources := resourcesByType[resourceType]
		sb.WriteString(mg.generateTableForType(resourceType, typeResources))
		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString("---\n\n")
	fmt.Fprintf(&sb, "**Total Resources:** %d\n\n", len(resources))
	fmt.Fprintf(&sb, "**Resource Types:** %d\n", len(resourcesByType))

	return sb.String()
}

// groupResourcesByType groups resources by their type
func (mg *MarkdownGenerator) groupResourcesByType(resources []parser.TerraformResource) map[string][]parser.TerraformResource {
	grouped := make(map[string][]parser.TerraformResource)
	for _, resource := range resources {
		grouped[resource.Type] = append(grouped[resource.Type], resource)
	}
	return grouped
}

// getSortedResourceTypes returns sorted list of resource types
func (mg *MarkdownGenerator) getSortedResourceTypes(resourcesByType map[string][]parser.TerraformResource) []string {
	var types []string
	for resourceType := range resourcesByType {
		types = append(types, resourceType)
	}
	sort.Strings(types)
	return types
}

// generateTableForType generates a markdown table for resources of a specific type
func (mg *MarkdownGenerator) generateTableForType(resourceType string, resources []parser.TerraformResource) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "## %s\n\n", resourceType)

	// Get the required fields from schema
	fields := mg.getRequiredFields(resourceType)

	if len(fields) == 0 {
		// No schema fields defined, create simple table
		sb.WriteString("| Resource Name | Description |\n")
		sb.WriteString("|--------------|-------------|\n")

		for _, resource := range resources {
			desc := mg.extractDescription(resource)
			fmt.Fprintf(&sb, "| `%s` | %s |\n", resource.Name, desc)
		}
	} else {
		// Create table with schema fields as columns
		// Header
		sb.WriteString("| Resource | ")
		for _, field := range fields {
			fmt.Fprintf(&sb, "%s | ", field)
		}
		sb.WriteString("\n")

		// Separator
		sb.WriteString("|----------|")
		for range fields {
			sb.WriteString("--------|")
		}
		sb.WriteString("\n")

		// Rows
		for _, resource := range resources {
			fmt.Fprintf(&sb, "| `%s` |", resource.Name)
			for _, field := range fields {
				value := mg.extractFieldValue(resource, field)
				fmt.Fprintf(&sb, " %s |", value)
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// getRequiredFields gets the list of required fields for a resource type from schema
func (mg *MarkdownGenerator) getRequiredFields(resourceType string) []string {
	var fields []string

	// Check if there's a specific rule for this resource type
	if rules, exists := mg.schema.ResourceTypes[resourceType]; exists {
		for prefix, prefixRule := range rules.PrefixRules {
			// Add required fields with prefix
			for _, field := range prefixRule.RequiredFields {
				fields = append(fields, fmt.Sprintf("%s:%s", prefix, field))
			}
		}
	}

	// Also check global rules
	for prefix, prefixRule := range mg.schema.Global.PrefixRules {
		for _, field := range prefixRule.RequiredFields {
			fieldName := fmt.Sprintf("%s:%s", prefix, field)
			// Only add if not already present
			if !contains(fields, fieldName) {
				fields = append(fields, fieldName)
			}
		}
	}

	return fields
}

// extractFieldValue extracts a field value from a resource's comments
func (mg *MarkdownGenerator) extractFieldValue(resource parser.TerraformResource, fieldName string) string {
	// Parse field name (format: "prefix:field" or "field")
	parts := strings.SplitN(fieldName, ":", 2)
	var prefix, field string

	if len(parts) == 2 {
		prefix = parts[0]
		field = parts[1]
	} else {
		field = fieldName
	}

	// Search through resource comments
	for _, comment := range resource.PrecedingComments {
		// Check if this comment matches the prefix
		if prefix != "" && comment.Prefix != prefix {
			continue
		}

		// Extract field value from the comment's Fields map
		if value, exists := comment.Fields[field]; exists {
			return fmt.Sprintf("%v", value)
		}
	}

	return "-"
}

// extractDescription extracts description from resource comments
func (mg *MarkdownGenerator) extractDescription(resource parser.TerraformResource) string {
	// Try to find description in different comment prefixes
	for _, comment := range resource.PrecedingComments {
		if comment.Prefix == "@docs" || comment.Prefix == "@metadata" {
			if desc, exists := comment.Fields["description"]; exists {
				return fmt.Sprintf("%v", desc)
			}
		}
	}

	return "-"
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
