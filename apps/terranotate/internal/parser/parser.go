package parser

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/spf13/afero"
)

// StructuredComment represents a parsed comment with prefix-based fields
type StructuredComment struct {
	Prefix  string                 // e.g., "metadata", "docs", "validation"
	Fields  map[string]interface{} // Parsed key-value pairs (supports nested structures)
	Raw     string                 // Original comment text
	Line    int                    // Starting line number in file
	EndLine int                    // Ending line number (for multi-line comments)
}

// TerraformResource represents a parsed resource with associated comments
type TerraformResource struct {
	Type              string
	Name              string
	StartLine         int
	EndLine           int
	Attributes        map[string]interface{}
	PrecedingComments []StructuredComment
	InlineComments    []StructuredComment
}

// CommentParser handles parsing of Terraform files with comment extraction
type CommentParser struct {
	fs       afero.Fs
	prefixes []string // Comment prefixes to look for (e.g., "@metadata", "@docs")
}

func NewCommentParser(fs afero.Fs, prefixes []string) *CommentParser {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	return &CommentParser{fs: fs, prefixes: prefixes}
}

// ParseFile parses a Terraform file and extracts resources with their comments
func (cp *CommentParser) ParseFile(filename string) ([]TerraformResource, error) {
	// Clean the path
	filename = filepath.Clean(filename)

	// #nosec G304 - File path provided by user, cleaned above.
	// Using afero abstraction which defaults to OsFs.
	f, err := cp.fs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	src, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// Parse the HCL file
	file, diags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse error: %s", diags.Error())
	}

	// Get all tokens including comments
	tokens, diags := hclsyntax.LexConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("lex error: %s", diags.Error())
	}

	// Extract all comments with their positions
	comments := cp.extractComments(tokens)

	// Parse resources from the syntax tree
	body := file.Body.(*hclsyntax.Body)
	var resources []TerraformResource

	for _, block := range body.Blocks {
		if block.Type == "resource" {
			resource := cp.parseResource(block, comments)
			resources = append(resources, resource)
		}
	}

	return resources, nil
}

// extractComments extracts all comments from tokens and parses structured fields
func (cp *CommentParser) extractComments(tokens hclsyntax.Tokens) []StructuredComment {
	var comments []StructuredComment
	var commentBuffer []string
	var bufferStartLine int
	var inMultiLine bool

	for i, token := range tokens {
		if token.Type == hclsyntax.TokenComment {
			text := string(token.Bytes)
			line := token.Range.Start.Line

			// Check if this starts a new comment block
			if !inMultiLine {
				bufferStartLine = line
				inMultiLine = true
			}

			commentBuffer = append(commentBuffer, text)

			// Check if next token is also a comment on the next line (continuation)
			isLastToken := i == len(tokens)-1
			nextIsComment := !isLastToken && tokens[i+1].Type == hclsyntax.TokenComment
			nextIsAdjacent := !isLastToken && tokens[i+1].Range.Start.Line == line+1

			// If this is the end of a comment block, process it
			if isLastToken || !nextIsComment || !nextIsAdjacent {
				structured := cp.parseMultiLineComment(commentBuffer, bufferStartLine, line)
				if structured != nil {
					comments = append(comments, *structured)
				}
				commentBuffer = nil
				inMultiLine = false
			}
		}
	}

	return comments
}

// parseMultiLineComment processes a buffer of comment lines
func (cp *CommentParser) parseMultiLineComment(lines []string, startLine, endLine int) *StructuredComment {
	if len(lines) == 0 {
		return nil
	}

	// Clean and combine all lines
	var cleanedLines []string
	for _, line := range lines {
		cleaned := strings.TrimPrefix(line, "//")
		cleaned = strings.TrimPrefix(cleaned, "#")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned != "" {
			cleanedLines = append(cleanedLines, cleaned)
		}
	}

	if len(cleanedLines) == 0 {
		return nil
	}

	// Check if first line starts with any of our prefixes
	var matchedPrefix string
	for _, prefix := range cp.prefixes {
		if strings.HasPrefix(cleanedLines[0], prefix) {
			matchedPrefix = prefix
			break
		}
	}

	if matchedPrefix == "" {
		return nil
	}

	// Join all lines for parsing
	fullText := strings.Join(cleanedLines, "\n")

	// Parse fields with support for nested structures
	fields := cp.parseCommentFields(fullText)

	return &StructuredComment{
		Prefix:  matchedPrefix,
		Fields:  fields,
		Raw:     fullText,
		Line:    startLine,
		EndLine: endLine,
	}
}

// parseCommentFields extracts key:value pairs from a comment with nested structure support
// Supports formats like:
//
//	Simple: @metadata owner:john.doe team:platform priority:high
//	Nested: @metadata owner:john.doe contact.email:john@example.com contact.slack:@john
//	Multi-line with indentation for nested fields
func (cp *CommentParser) parseCommentFields(text string) map[string]interface{} {
	fields := make(map[string]interface{})

	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return fields
	}

	// Remove prefix from first line
	firstLine := lines[0]
	for _, prefix := range cp.prefixes {
		if strings.HasPrefix(firstLine, prefix) {
			firstLine = strings.TrimSpace(strings.TrimPrefix(firstLine, prefix))
			lines[0] = firstLine
			break
		}
	}

	// Parse all lines for key:value pairs
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract all key:value pairs from this line
		cp.extractKeyValuePairs(line, fields)
	}

	// Store the full content
	fullContent := strings.TrimSpace(strings.Join(lines, "\n"))
	if fullContent != "" {
		fields["_content"] = fullContent
	}

	return fields
}

// extractKeyValuePairs extracts key:value pairs and handles nested keys
func (cp *CommentParser) extractKeyValuePairs(line string, fields map[string]interface{}) {
	// Pattern to match key:value pairs (supports nested keys with dots)
	// Matches word.word:value where value stops before the next word.word: pattern
	fieldRegex := regexp.MustCompile(`([\w\.]+):(\S+)`)
	matches := fieldRegex.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) == 3 {
			key := match[1]
			value := strings.TrimSpace(match[2])

			// Handle nested keys (e.g., "contact.email" or "config.db.host")
			if strings.Contains(key, ".") {
				cp.setNestedField(fields, key, value)
			} else {
				// Try to parse value as different types
				fields[key] = cp.parseValue(value)
			}
		}
	}
}

// setNestedField sets a value in a nested map structure based on dot notation
func (cp *CommentParser) setNestedField(fields map[string]interface{}, key string, value string) {
	parts := strings.Split(key, ".")
	current := fields

	// Navigate/create nested structure
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]interface{})
		}

		// Type assertion to navigate deeper
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			// If it's not a map, we can't nest further, so store at current level
			current[key] = cp.parseValue(value)
			return
		}
	}

	// Set the final value
	finalKey := parts[len(parts)-1]
	current[finalKey] = cp.parseValue(value)
}

// parseValue attempts to parse a string value into appropriate types
func (cp *CommentParser) parseValue(value string) interface{} {
	value = strings.TrimSpace(value)

	// Try to parse as boolean
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}

	// Try to parse as number
	if num, err := fmt.Sscanf(value, "%d", new(int)); err == nil && num == 1 {
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}

	if num, err := fmt.Sscanf(value, "%f", new(float64)); err == nil && num == 1 {
		var f float64
		if _, err := fmt.Sscanf(value, "%f", &f); err == nil {
			return f
		}
	}

	// Check for array notation [item1,item2,item3]
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimPrefix(value, "[")
		value = strings.TrimSuffix(value, "]")
		items := strings.Split(value, ",")
		var result []interface{}
		for _, item := range items {
			result = append(result, strings.TrimSpace(item))
		}
		return result
	}

	// Return as string
	return value
}

// parseResource extracts resource information and associates comments
func (cp *CommentParser) parseResource(block *hclsyntax.Block, comments []StructuredComment) TerraformResource {
	resource := TerraformResource{
		Type:       block.Labels[0],
		Name:       block.Labels[1],
		StartLine:  block.DefRange().Start.Line,
		EndLine:    block.Range().End.Line,
		Attributes: make(map[string]interface{}),
	}

	// Extract attributes
	for name, attr := range block.Body.Attributes {
		resource.Attributes[name] = cp.extractAttributeValue(attr)
	}

	// Associate comments with this resource
	for _, comment := range comments {
		// Preceding comments: within 5 lines before the resource
		if comment.Line < resource.StartLine && comment.Line >= resource.StartLine-5 {
			resource.PrecedingComments = append(resource.PrecedingComments, comment)
		}

		// Inline comments: within the resource block
		if comment.Line >= resource.StartLine && comment.Line <= resource.EndLine {
			resource.InlineComments = append(resource.InlineComments, comment)
		}
	}

	return resource
}

// extractAttributeValue extracts the value from an attribute
func (cp *CommentParser) extractAttributeValue(attr *hclsyntax.Attribute) interface{} {
	// This is a simplified version - you might want more sophisticated extraction
	tokens := attr.Expr.Range().SliceBytes(attr.Expr.StartRange().SliceBytes([]byte{}))
	return string(tokens)
}

// GetCommentsByPrefix filters comments by prefix for a resource
func (r *TerraformResource) GetCommentsByPrefix(prefix string) []StructuredComment {
	var result []StructuredComment

	allComments := make([]StructuredComment, 0, len(r.PrecedingComments)+len(r.InlineComments))
	allComments = append(allComments, r.PrecedingComments...)
	allComments = append(allComments, r.InlineComments...)
	for _, comment := range allComments {
		if comment.Prefix == prefix {
			result = append(result, comment)
		}
	}

	return result
}

// GetNestedField retrieves a nested field value using dot notation
func (r *TerraformResource) GetNestedField(commentPrefix, fieldPath string) interface{} {
	comments := r.GetCommentsByPrefix(commentPrefix)
	if len(comments) == 0 {
		return nil
	}

	// Use the first matching comment
	comment := comments[0]

	// Navigate nested fields
	parts := strings.Split(fieldPath, ".")
	current := comment.Fields

	for i, part := range parts {
		if val, exists := current[part]; exists {
			if i == len(parts)-1 {
				// This is the final key
				return val
			}
			// Navigate deeper if it's a nested map
			if nested, ok := val.(map[string]interface{}); ok {
				current = nested
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	return nil
}
