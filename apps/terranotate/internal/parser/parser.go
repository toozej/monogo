package parser

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

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

type positionedCommentLine struct {
	text string
	line int
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

	// Extract all comments with their positions.
	comments := cp.extractComments(tokens)

	// Parse resources from the syntax tree
	body := file.Body.(*hclsyntax.Body)
	var resources []TerraformResource

	for _, block := range body.Blocks {
		if block.Type == "resource" {
			resource := cp.parseResource(block)
			resources = append(resources, resource)
		}
	}
	cp.associateComments(resources, comments)

	return resources, nil
}

// extractComments extracts all comments from tokens and parses structured fields
func (cp *CommentParser) extractComments(tokens hclsyntax.Tokens) []StructuredComment {
	var comments []StructuredComment
	var block []positionedCommentLine
	lastLine := 0
	flush := func() {
		if len(block) == 0 {
			return
		}
		comments = append(comments, cp.parseCommentBlock(block)...)
		block = nil
	}

	for _, token := range tokens {
		if token.Type != hclsyntax.TokenComment {
			flush()
			lastLine = 0
			continue
		}
		lines := strings.Split(strings.TrimSuffix(string(token.Bytes), "\n"), "\n")
		for offset, raw := range lines {
			line := token.Range.Start.Line + offset
			if len(block) > 0 && line > lastLine+1 {
				flush()
			}
			cleaned := cleanCommentLine(raw)
			if cleaned != "" {
				block = append(block, positionedCommentLine{text: cleaned, line: line})
				lastLine = line
			}
		}
	}
	flush()

	return comments
}

func cleanCommentLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "//")
	line = strings.TrimPrefix(line, "#")
	line = strings.TrimPrefix(line, "/*")
	line = strings.TrimSuffix(line, "*/")
	line = strings.TrimPrefix(strings.TrimSpace(line), "*")
	return strings.TrimSpace(line)
}

func (cp *CommentParser) matchPrefix(line string) string {
	for _, prefix := range cp.prefixes {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := strings.TrimPrefix(line, prefix)
		if rest == "" || unicode.IsSpace(rune(rest[0])) {
			return prefix
		}
	}
	return ""
}

func (cp *CommentParser) parseCommentBlock(lines []positionedCommentLine) []StructuredComment {
	var comments []StructuredComment
	var current []positionedCommentLine
	var prefix string
	flush := func() {
		if prefix == "" || len(current) == 0 {
			return
		}
		text := make([]string, 0, len(current))
		for _, line := range current {
			text = append(text, line.text)
		}
		fullText := strings.Join(text, "\n")
		comments = append(comments, StructuredComment{
			Prefix:  prefix,
			Fields:  cp.parseCommentFields(fullText),
			Raw:     fullText,
			Line:    current[0].line,
			EndLine: current[len(current)-1].line,
		})
	}

	for _, line := range lines {
		if matched := cp.matchPrefix(line.text); matched != "" {
			flush()
			prefix = matched
			current = current[:0]
		}
		if prefix != "" {
			current = append(current, line)
		}
	}
	flush()
	return comments
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
		if cp.matchPrefix(firstLine) == prefix {
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
	matches := findFieldMarkers(line)

	for i, match := range matches {
		if match.keyStart < match.keyEnd {
			key := line[match.keyStart:match.keyEnd]
			valueEnd := len(line)
			if i+1 < len(matches) {
				valueEnd = matches[i+1].keyStart
			}
			value := strings.TrimSpace(line[match.valueStart:valueEnd])
			value = strings.Trim(value, `"'`)

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

type fieldMarker struct {
	keyStart   int
	keyEnd     int
	valueStart int
}

func findFieldMarkers(line string) []fieldMarker {
	var markers []fieldMarker
	var quote byte
	escaped := false
	for i := 0; i < len(line); i++ {
		char := line[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if (char == '\'' || char == '"') && (i == 0 || unicode.IsSpace(rune(line[i-1])) || strings.ContainsRune(":,[", rune(line[i-1]))) {
			quote = char
			continue
		}
		if i > 0 && !unicode.IsSpace(rune(line[i-1])) {
			continue
		}
		end := i
		for end < len(line) && isFieldKeyChar(line[end]) {
			end++
		}
		if end == i || end >= len(line) || line[end] != ':' {
			continue
		}
		markers = append(markers, fieldMarker{keyStart: i, keyEnd: end, valueStart: end + 1})
		i = end
	}
	return markers
}

func isFieldKeyChar(char byte) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
		char >= '0' && char <= '9' || char == '_' || char == '.'
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

	// Parse numbers only when the complete value is numeric.
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
		return f
	}

	// Check for array notation [item1,item2,item3]
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimPrefix(value, "[")
		value = strings.TrimSuffix(value, "]")
		if strings.TrimSpace(value) == "" {
			return []interface{}{}
		}
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
func (cp *CommentParser) parseResource(block *hclsyntax.Block) TerraformResource {
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

	return resource
}

func (cp *CommentParser) associateComments(resources []TerraformResource, comments []StructuredComment) {
	for commentIndex, comment := range comments {
		associated := false
		for i := range resources {
			if comment.Line >= resources[i].StartLine && comment.EndLine <= resources[i].EndLine {
				resources[i].InlineComments = append(resources[i].InlineComments, comment)
				associated = true
				break
			}
		}
		if associated {
			continue
		}
		blockEndLine := comment.EndLine
		for next := commentIndex + 1; next < len(comments) && comments[next].Line <= blockEndLine+1; next++ {
			blockEndLine = comments[next].EndLine
		}
		for i := range resources {
			if blockEndLine < resources[i].StartLine && blockEndLine >= resources[i].StartLine-5 {
				resources[i].PrecedingComments = append(resources[i].PrecedingComments, comment)
				break
			}
		}
	}
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
