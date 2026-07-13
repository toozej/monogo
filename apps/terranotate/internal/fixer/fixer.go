package fixer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

		rules := cf.getApplicableRules(resource.Type)
		insertLine := cf.findInsertionPoint(lines, resource.StartLine)
		var missingPrefixFixes []CommentFix
		type continuation struct {
			line int
			fix  CommentFix
		}
		var continuations []continuation
		for _, fix := range fixes {
			comments := resource.GetCommentsByPrefix(fix.Prefix)
			if len(comments) == 0 {
				missingPrefixFixes = append(missingPrefixFixes, fix)
				continue
			}
			// Add fields to the existing structured comment instead of creating
			// another occurrence of the same prefix. The validator checks each
			// occurrence independently, so a duplicate would leave both invalid.
			target := comments[0]
			for _, comment := range comments {
				if comment.Line == fix.Line {
					target = comment
					break
				}
			}
			continuations = append(continuations, continuation{line: target.EndLine, fix: fix})
		}
		sort.Slice(continuations, func(i, j int) bool { return continuations[i].line > continuations[j].line })
		for _, item := range continuations {
			lines = cf.insertLines(lines, item.line, cf.buildContinuationBlock(item.fix, rules))
		}
		if len(missingPrefixFixes) > 0 {
			lines = cf.insertLines(lines, insertLine, cf.buildCommentBlock(missingPrefixFixes, rules))
		}
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
	type commentKey struct {
		prefix string
		line   int
	}
	missingFields := make(map[commentKey][]string)

	for _, err := range errors {
		// Check if it's a missing prefix error
		if strings.Contains(err.Message, "Missing required comment prefix:") {
			prefix := strings.TrimSpace(strings.TrimPrefix(err.Message, "Missing required comment prefix:"))
			missingPrefixes[prefix] = true
			continue
		}

		// Check if it's a missing nested structure. Populate every required
		// field in that structure so the generated continuation is complete.
		if strings.Contains(err.Message, "Missing nested structure") {
			prefix, nestedPath, ok := parseQuotedValidationError(err.Message)
			if !ok {
				continue
			}
			prefixRule, exists := rules.PrefixRules[prefix]
			if !exists {
				continue
			}
			nestedRule, exists := prefixRule.NestedFields[nestedPath]
			if !exists {
				continue
			}
			for _, field := range nestedRule.RequiredFields {
				key := commentKey{prefix: prefix, line: err.Line}
				missingFields[key] = append(missingFields[key], nestedPath+"."+field)
			}
			continue
		}

		// Check if it's a missing field error.
		if strings.Contains(err.Message, "Missing required field") || strings.Contains(err.Message, "Missing required nested field") {
			// Extract prefix and field from error message
			// Format: "@metadata: Missing required field 'owner'"
			if prefix, field, ok := parseQuotedValidationError(err.Message); ok {
				key := commentKey{prefix: prefix, line: err.Line}
				missingFields[key] = append(missingFields[key], field)
			}
		}
	}

	// Generate missing prefixes in schema order so repeated runs and machines
	// produce the same file.
	for _, prefix := range rules.RequiredPrefixes {
		if !missingPrefixes[prefix] {
			continue
		}
		fix := cf.generatePrefixFix(prefix, rules)
		if fix != nil {
			fixes = append(fixes, *fix)
		}
		delete(missingPrefixes, prefix)
	}
	remainingPrefixes := make([]string, 0, len(missingPrefixes))
	for prefix := range missingPrefixes {
		remainingPrefixes = append(remainingPrefixes, prefix)
	}
	sort.Strings(remainingPrefixes)
	for _, prefix := range remainingPrefixes {
		if fix := cf.generatePrefixFix(prefix, rules); fix != nil {
			fixes = append(fixes, *fix)
		}
	}

	// Generate fixes for missing fields
	fieldKeys := make([]commentKey, 0, len(missingFields))
	for key := range missingFields {
		fieldKeys = append(fieldKeys, key)
	}
	sort.Slice(fieldKeys, func(i, j int) bool {
		if fieldKeys[i].line != fieldKeys[j].line {
			return fieldKeys[i].line < fieldKeys[j].line
		}
		return fieldKeys[i].prefix < fieldKeys[j].prefix
	})
	for _, key := range fieldKeys {
		fields := missingFields[key]
		fix := cf.generateFieldFix(key.prefix, fields, rules)
		if fix != nil {
			fix.Line = key.line
			fixes = append(fixes, *fix)
		}
	}

	return fixes
}

func parseQuotedValidationError(message string) (string, string, bool) {
	parts := strings.SplitN(message, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	detail := parts[1]
	start := strings.Index(detail, "'")
	if start == -1 {
		return "", "", false
	}
	end := strings.Index(detail[start+1:], "'")
	if end == -1 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), detail[start+1 : start+1+end], true
}

// CommentFix represents a fix to apply
type CommentFix struct {
	Prefix string
	Fields map[string]string
	Line   int
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

// buildCommentBlock builds a comment block from fixes with fields ordered by
// the rules that apply to the resource being fixed.
func (cf *CommentFixer) buildCommentBlock(fixes []CommentFix, rules validator.ResourceRules) []string {
	var lines []string

	for _, fix := range fixes {
		// Get the schema rules to determine field order
		prefixRule, exists := rules.PrefixRules[fix.Prefix]
		if !exists {
			// Fallback to unordered if we can't find the rule
			cf.buildUnorderedCommentBlock(fix, &lines)
			continue
		}

		rootFields, nestedFields := splitFixFields(fix, prefixRule)

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
		nestedPaths := sortedNestedPaths(prefixRule)
		for _, nestedPath := range nestedPaths {
			nestedRule := prefixRule.NestedFields[nestedPath]
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

func (cf *CommentFixer) buildContinuationBlock(fix CommentFix, rules validator.ResourceRules) []string {
	prefixRule, exists := rules.PrefixRules[fix.Prefix]
	if !exists {
		var lines []string
		cf.buildUnorderedContinuationBlock(fix, &lines)
		return lines
	}

	rootFields, nestedFields := splitFixFields(fix, prefixRule)

	var lines []string
	rootLine := "#"
	for _, field := range append(append([]string{}, prefixRule.RequiredFields...), prefixRule.OptionalFields...) {
		if value, ok := rootFields[field]; ok {
			rootLine += fmt.Sprintf(" %s:%s", field, value)
		}
	}
	if rootLine != "#" {
		lines = append(lines, rootLine)
	}

	nestedPaths := sortedNestedPaths(prefixRule)
	for _, nestedPath := range nestedPaths {
		fieldMap := nestedFields[nestedPath]
		if len(fieldMap) == 0 {
			continue
		}
		nestedLine := "#"
		nestedRule := prefixRule.NestedFields[nestedPath]
		for _, field := range append(append([]string{}, nestedRule.RequiredFields...), nestedRule.OptionalFields...) {
			if value, ok := fieldMap[field]; ok {
				nestedLine += fmt.Sprintf(" %s.%s:%s", nestedPath, field, value)
			}
		}
		if nestedLine != "#" {
			lines = append(lines, nestedLine)
		}
	}
	return lines
}

func splitFixFields(fix CommentFix, rule validator.PrefixRule) (map[string]string, map[string]map[string]string) {
	rootFields := make(map[string]string)
	nestedFields := make(map[string]map[string]string)
	paths := sortedNestedPaths(rule)
	sort.SliceStable(paths, func(i, j int) bool { return len(paths[i]) > len(paths[j]) })

	for field, value := range fix.Fields {
		matched := false
		for _, nestedPath := range paths {
			prefix := nestedPath + "."
			if !strings.HasPrefix(field, prefix) {
				continue
			}
			if nestedFields[nestedPath] == nil {
				nestedFields[nestedPath] = make(map[string]string)
			}
			nestedFields[nestedPath][strings.TrimPrefix(field, prefix)] = value
			matched = true
			break
		}
		if !matched {
			rootFields[field] = value
		}
	}
	return rootFields, nestedFields
}

func sortedNestedPaths(rule validator.PrefixRule) []string {
	paths := make([]string, 0, len(rule.NestedFields))
	for path := range rule.NestedFields {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func (cf *CommentFixer) buildUnorderedContinuationBlock(fix CommentFix, lines *[]string) {
	copyFix := fix
	copyFix.Prefix = ""
	cf.buildUnorderedCommentBlock(copyFix, lines)
	if len(*lines) > 0 {
		(*lines)[0] = strings.TrimSpace((*lines)[0])
		if (*lines)[0] == "#" {
			*lines = (*lines)[1:]
		}
	}
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
