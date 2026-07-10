package fixer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/toozej/monogo/apps/terranotate/internal/parser"
	"github.com/toozej/monogo/apps/terranotate/internal/validator"
)

// CommentFixer handles automatic fixing of validation errors
type CommentFixer struct {
	fs     afero.Fs
	schema validator.ValidationSchema
}

// NewCommentFixer creates a new comment fixer
func NewCommentFixer(fs afero.Fs, schema validator.ValidationSchema) *CommentFixer {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	return &CommentFixer{fs: fs, schema: schema}
}

// FixFile attempts to fix validation errors in a Terraform file
func (cf *CommentFixer) FixFile(filename string, resources []parser.TerraformResource, errors []validator.ValidationError) (string, int, error) {
	// #nosec G304 - File provided by user via CLI, using afero abstraction
	f, err := cf.fs.Open(filename)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()

	content, err := io.ReadAll(f)
	if err != nil {
		return "", 0, err
	}

	lines := strings.Split(string(content), "\n")
	fixCount := 0

	// Group errors by resource
	errorsByResource := cf.groupErrorsByResource(errors)

	// Process each resource
	for resourceIndex := len(resources) - 1; resourceIndex >= 0; resourceIndex-- {
		resource := resources[resourceIndex]
		resourceKey := fmt.Sprintf("%s.%s", resource.Type, resource.Name)
		resourceErrors, hasErrors := errorsByResource[resourceKey]

		if !hasErrors {
			continue
		}

		// Check if resource already has valid comments (including placeholders like "CHANGEME")
		if cf.hasValidComments(resource, resourceErrors) {
			continue
		}

		// Generate fixes for this resource
		fixes := cf.generateFixes(resource, resourceErrors)

		if len(fixes) == 0 {
			continue
		}

		// Insert comment block immediately before the resource declaration
		// Skip any existing comments directly above the resource
		insertLine := cf.findInsertionPoint(lines, resource.StartLine)

		// Build comment block
		commentBlock := cf.buildCommentBlock(fixes)

		// Insert the comment block
		lines = cf.insertLines(lines, insertLine, commentBlock)
		fixCount += len(fixes)
	}

	return strings.Join(lines, "\n"), fixCount, nil
}

// groupErrorsByResource groups validation errors by resource
func (cf *CommentFixer) groupErrorsByResource(errors []validator.ValidationError) map[string][]validator.ValidationError {
	result := make(map[string][]validator.ValidationError)

	for _, err := range errors {
		// Remove filename suffix if present
		resourceType := err.ResourceType
		if idx := strings.Index(resourceType, " ("); idx != -1 {
			resourceType = resourceType[:idx]
		}

		key := fmt.Sprintf("%s.%s", resourceType, err.ResourceName)
		result[key] = append(result[key], err)
	}

	return result
}

// generateFixes generates comment fixes for a resource
func (cf *CommentFixer) generateFixes(resource parser.TerraformResource, errors []validator.ValidationError) []CommentFix {
	var fixes []CommentFix

	// Get applicable schema rules
	rules := cf.getApplicableRules(resource.Type)

	// Track which prefixes we need to add
	missingPrefixes := make(map[string]bool)
	missingFields := make(map[string][]string) // prefix -> []fields

	for _, err := range errors {
		// Check if it's a missing prefix error
		if strings.Contains(err.Message, "Missing required comment prefix:") {
			prefix := strings.TrimSpace(strings.TrimPrefix(err.Message, "Missing required comment prefix:"))
			missingPrefixes[prefix] = true
			continue
		}

		// Check if it's a missing field error
		if strings.Contains(err.Message, "Missing required field") {
			// Extract prefix and field from error message
			// Format: "@metadata: Missing required field 'owner'"
			parts := strings.SplitN(err.Message, ":", 2)
			if len(parts) == 2 {
				prefix := strings.TrimSpace(parts[0])
				fieldMsg := strings.TrimSpace(parts[1])

				// Extract field name from quotes
				if start := strings.Index(fieldMsg, "'"); start != -1 {
					if end := strings.Index(fieldMsg[start+1:], "'"); end != -1 {
						field := fieldMsg[start+1 : start+1+end]
						missingFields[prefix] = append(missingFields[prefix], field)
					}
				}
			}
		}
	}

	// Generate fixes for missing prefixes
	for prefix := range missingPrefixes {
		fix := cf.generatePrefixFix(prefix, rules)
		if fix != nil {
			fixes = append(fixes, *fix)
		}
	}

	// Generate fixes for missing fields
	for prefix, fields := range missingFields {
		fix := cf.generateFieldFix(prefix, fields, rules)
		if fix != nil {
			fixes = append(fixes, *fix)
		}
	}

	return fixes
}

// CommentFix represents a fix to apply
type CommentFix struct {
	Prefix string
	Fields map[string]string
}

// generatePrefixFix generates a fix for a missing prefix with default required fields
func (cf *CommentFixer) generatePrefixFix(prefix string, rules validator.ResourceRules) *CommentFix {
	prefixRule, exists := rules.PrefixRules[prefix]
	if !exists {
		return nil
	}

	fix := &CommentFix{
		Prefix: prefix,
		Fields: make(map[string]string),
	}

	// Add placeholders for all required fields
	for _, field := range prefixRule.RequiredFields {
		fix.Fields[field] = cf.getPlaceholderValue(field)
	}

	// Add placeholders for required nested fields
	for nestedPath, nestedRule := range prefixRule.NestedFields {
		for _, field := range nestedRule.RequiredFields {
			fullPath := nestedPath + "." + field
			fix.Fields[fullPath] = cf.getPlaceholderValue(field)
		}
	}

	return fix
}

// generateFieldFix generates a fix for missing fields in an existing prefix
func (cf *CommentFixer) generateFieldFix(prefix string, fields []string, rules validator.ResourceRules) *CommentFix {
	fix := &CommentFix{
		Prefix: prefix,
		Fields: make(map[string]string),
	}

	for _, field := range fields {
		fix.Fields[field] = cf.getPlaceholderValue(field)
	}

	return fix
}

// getPlaceholderValue returns a placeholder value for a field
func (cf *CommentFixer) getPlaceholderValue(field string) string {
	// Remove nested path if present
	parts := strings.Split(field, ".")
	fieldName := parts[len(parts)-1]

	// Common field placeholders
	placeholders := map[string]string{
		"owner":             "CHANGEME",
		"team":              "CHANGEME",
		"priority":          "medium",
		"environment":       "production",
		"email":             "changeme@example.com",
		"slack":             "@changeme",
		"phone":             "555-0000",
		"description":       "CHANGEME: Add description",
		"required":          "true",
		"enabled":           "true",
		"backup":            "true",
		"encrypted":         "true",
		"cost_center":       "CHANGEME",
		"department":        "CHANGEME",
		"emergency_contact": "oncall@example.com",
		"uptime":            "99.9",
		"replicas":          "3",
		"backup_required":   "true",
		"mfa_required":      "true",
		"password_policy":   "strict",
	}

	if val, exists := placeholders[fieldName]; exists {
		return val
	}

	// Check field validation for type hints
	if validation, exists := cf.schema.FieldValidations[fieldName]; exists {
		if len(validation.AllowedValues) > 0 {
			return validation.AllowedValues[0]
		}

		switch validation.Type {
		case "boolean":
			return "true"
		case "integer":
			if validation.Min != nil {
				return fmt.Sprintf("%d", int(*validation.Min))
			}
			return "1"
		case "float":
			if validation.Min != nil {
				return fmt.Sprintf("%.1f", *validation.Min)
			}
			return "1.0"
		case "array":
			return "[CHANGEME]"
		}
	}

	return "CHANGEME"
}

// buildCommentBlock builds a comment block from fixes with fields ordered by schema
func (cf *CommentFixer) buildCommentBlock(fixes []CommentFix) []string {
	var lines []string

	for _, fix := range fixes {
		// Get the schema rules to determine field order
		prefixRule, exists := cf.getSchemaRuleForPrefix(fix.Prefix)
		if !exists {
			// Fallback to unordered if we can't find the rule
			cf.buildUnorderedCommentBlock(fix, &lines)
			continue
		}

		// Group fields by root vs nested
		rootFields := make(map[string]string)
		nestedFields := make(map[string]map[string]string)

		for field, value := range fix.Fields {
			if strings.Contains(field, ".") {
				// Nested field
				parts := strings.SplitN(field, ".", 2)
				prefix := parts[0]
				rest := parts[1]

				if nestedFields[prefix] == nil {
					nestedFields[prefix] = make(map[string]string)
				}
				nestedFields[prefix][rest] = value
			} else {
				// Root field
				rootFields[field] = value
			}
		}

		// Build comment line with ordered root fields
		commentLine := "# " + fix.Prefix

		// Add required fields first in schema order
		for _, field := range prefixRule.RequiredFields {
			if value, ok := rootFields[field]; ok {
				commentLine += fmt.Sprintf(" %s:%s", field, value)
			}
		}

		// Add optional fields in schema order
		for _, field := range prefixRule.OptionalFields {
			if value, ok := rootFields[field]; ok {
				commentLine += fmt.Sprintf(" %s:%s", field, value)
			}
		}

		lines = append(lines, commentLine)

		// Add nested fields on separate lines in schema order
		for nestedPath, nestedRule := range prefixRule.NestedFields {
			if fieldMap, ok := nestedFields[nestedPath]; ok && len(fieldMap) > 0 {
				nestedLine := "#"

				// Add required nested fields first
				for _, field := range nestedRule.RequiredFields {
					if value, ok := fieldMap[field]; ok {
						nestedLine += fmt.Sprintf(" %s.%s:%s", nestedPath, field, value)
					}
				}

				// Add optional nested fields
				for _, field := range nestedRule.OptionalFields {
					if value, ok := fieldMap[field]; ok {
						nestedLine += fmt.Sprintf(" %s.%s:%s", nestedPath, field, value)
					}
				}

				if len(nestedLine) > 1 { // More than just "#"
					lines = append(lines, nestedLine)
				}
			}
		}
	}

	return lines
}

// getSchemaRuleForPrefix retrieves the prefix rule from the schema
func (cf *CommentFixer) getSchemaRuleForPrefix(prefix string) (validator.PrefixRule, bool) {
	// Check global rules first
	if rule, ok := cf.schema.Global.PrefixRules[prefix]; ok {
		return rule, true
	}

	// Could also check resource-specific rules if needed
	// but for now we use global rules
	return validator.PrefixRule{}, false
}

// buildUnorderedCommentBlock is a fallback for when schema rules aren't found
func (cf *CommentFixer) buildUnorderedCommentBlock(fix CommentFix, lines *[]string) {
	// Group fields by prefix (for nested fields)
	rootFields := make(map[string]string)
	nestedFields := make(map[string]map[string]string)

	for field, value := range fix.Fields {
		if strings.Contains(field, ".") {
			// Nested field
			parts := strings.SplitN(field, ".", 2)
			prefix := parts[0]
			rest := parts[1]

			if nestedFields[prefix] == nil {
				nestedFields[prefix] = make(map[string]string)
			}
			nestedFields[prefix][rest] = value
		} else {
			// Root field
			rootFields[field] = value
		}
	}

	// Build comment lines
	commentLine := "# " + fix.Prefix

	// Add root fields
	for field, value := range rootFields {
		commentLine += fmt.Sprintf(" %s:%s", field, value)
	}

	*lines = append(*lines, commentLine)

	// Add nested fields on separate lines
	for prefix, fields := range nestedFields {
		nestedLine := "#"
		for field, value := range fields {
			nestedLine += fmt.Sprintf(" %s.%s:%s", prefix, field, value)
		}
		*lines = append(*lines, nestedLine)
	}
}

// insertLines inserts new lines at the specified position
func (cf *CommentFixer) insertLines(lines []string, position int, newLines []string) []string {
	// Ensure position is valid
	if position < 0 {
		position = 0
	}
	if position > len(lines) {
		position = len(lines)
	}

	// Insert new lines
	result := make([]string, 0, len(lines)+len(newLines))
	result = append(result, lines[:position]...)
	result = append(result, newLines...)
	result = append(result, lines[position:]...)

	return result
}

// hasValidComments checks if a resource already has valid comments that satisfy the schema
// This includes placeholders like "CHANGEME" which are considered valid
func (cf *CommentFixer) hasValidComments(resource parser.TerraformResource, errors []validator.ValidationError) bool {
	// If there are validation errors for this resource, comments are not valid
	// However, we need to check if the errors are only about missing prefixes/fields
	// If comments exist with placeholder values (like "CHANGEME"), they're considered valid

	// Check if any of the resource's comments match the schema structure
	for _, comment := range resource.PrecedingComments {
		// Parse the comment to see if it has the expected prefix format
		if strings.HasPrefix(comment.Raw, "# @") || strings.HasPrefix(comment.Raw, "# terraform:") {
			// This looks like a managed comment - check if it has fields
			if strings.Contains(comment.Raw, ":") {
				// Comment has fields, consider it valid even if values are placeholders
				// Only skip if ALL required prefixes have at least some comment
				return cf.allPrefixesHaveComments(resource, errors)
			}
		}
	}

	return false
}

// allPrefixesHaveComments checks if all required prefixes have at least some comment
func (cf *CommentFixer) allPrefixesHaveComments(resource parser.TerraformResource, errors []validator.ValidationError) bool {
	// Get list of required prefixes from errors
	requiredPrefixes := make(map[string]bool)
	for _, err := range errors {
		if strings.Contains(err.Message, "Missing required comment prefix:") {
			prefix := strings.TrimSpace(strings.TrimPrefix(err.Message, "Missing required comment prefix:"))
			requiredPrefixes[prefix] = true
		}
	}

	// If there are missing prefix errors, comments are not valid
	if len(requiredPrefixes) > 0 {
		return false
	}

	// Check if all errors are only about field values (not structure)
	// If so, the comment structure is valid, just values need updating
	for _, err := range errors {
		if strings.Contains(err.Message, "Missing required comment prefix:") {
			return false
		}
		if strings.Contains(err.Message, "Missing required field") {
			return false
		}
	}

	// All structural requirements are met
	return true
}

// findInsertionPoint finds where to insert comments for a resource
// It places comments immediately above the resource declaration, skipping any existing comments
func (cf *CommentFixer) findInsertionPoint(lines []string, resourceStartLine int) int {
	resourceIndex := resourceStartLine - 1
	if resourceIndex <= 0 {
		return 0
	}
	if resourceIndex > len(lines) {
		resourceIndex = len(lines)
	}

	// Keep a contiguous managed annotation block together. New annotations go
	// before that block; otherwise they go immediately before the resource.
	insertIndex := resourceIndex
	foundManaged := false
	for i := resourceIndex - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		managedStart := strings.HasPrefix(trimmed, "# @") || strings.HasPrefix(trimmed, "# terraform:")
		if managedStart {
			foundManaged = true
			insertIndex = i
			continue
		}
		if foundManaged && strings.HasPrefix(trimmed, "#") {
			insertIndex = i
			continue
		}
		break
	}
	return insertIndex
}

// getApplicableRules returns applicable rules for a resource type
func (cf *CommentFixer) getApplicableRules(resourceType string) validator.ResourceRules {
	if rules, exists := cf.schema.ResourceTypes[resourceType]; exists {
		return rules
	}

	return validator.ResourceRules{
		RequiredPrefixes: cf.schema.Global.RequiredPrefixes,
		PrefixRules:      cf.schema.Global.PrefixRules,
	}
}

// CopyFile copies a file from src to dst. Exported for utility use.
func CopyFile(fs afero.Fs, src, dst string) error {
	// #nosec G304 - Source path provided by user
	sourceFile, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}
	return writeAtomic(fs, dst, sourceFile, info.Mode())
}

// CopyFileExclusive copies src to dst without replacing an existing destination.
func CopyFileExclusive(fs afero.Fs, src, dst string) error {
	sourceFile, err := fs.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()
	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}
	destFile, err := fs.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	removeDestination := true
	defer func() {
		_ = destFile.Close()
		if removeDestination {
			_ = fs.Remove(dst)
		}
	}()
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}
	if syncer, ok := destFile.(interface{ Sync() error }); ok {
		if err := syncer.Sync(); err != nil {
			return err
		}
	}
	if err := destFile.Close(); err != nil {
		return err
	}
	removeDestination = false
	return nil
}

// WriteFileAtomic replaces a file only after its complete new contents are durable.
func WriteFileAtomic(fs afero.Fs, filename string, contents []byte, mode os.FileMode) error {
	return writeAtomic(fs, filename, strings.NewReader(string(contents)), mode)
}

func writeAtomic(fs afero.Fs, dst string, source io.Reader, mode os.FileMode) error {
	dir := filepath.Dir(dst)
	temp, err := afero.TempFile(fs, dir, ".terranotate-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = fs.Remove(tempName)
	}()
	if err := fs.Chmod(tempName, mode.Perm()); err != nil {
		return err
	}
	if _, err := io.Copy(temp, source); err != nil {
		return err
	}
	if syncer, ok := temp.(interface{ Sync() error }); ok {
		if err := syncer.Sync(); err != nil {
			return err
		}
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return fs.Rename(tempName, dst)
}
