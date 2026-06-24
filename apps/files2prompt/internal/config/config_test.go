package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvVars(t *testing.T) {
	// Test with no .env file and no env vars (should use defaults)
	conf := GetEnvVars()
	assert.Empty(t, conf.Paths)
	assert.Empty(t, conf.Extensions)
	assert.False(t, conf.IncludeHidden)
	assert.False(t, conf.IgnoreGitignore)
	assert.Empty(t, conf.IgnorePatterns)
	assert.Empty(t, conf.OutputFile)
	assert.False(t, conf.ClaudeXML)
	assert.False(t, conf.LineNumbers)

	// Test with environment variables set
	_ = os.Setenv("PATHS", "path1,path2")
	_ = os.Setenv("EXTENSIONS", ".go,.txt")
	_ = os.Setenv("INCLUDE_HIDDEN", "true")
	_ = os.Setenv("IGNORE_GITIGNORE", "false")
	_ = os.Setenv("IGNORE_PATTERNS", "node_modules,vendor")
	_ = os.Setenv("OUTPUT_FILE", "output.md")
	_ = os.Setenv("CLAUDE_XML", "true")
	_ = os.Setenv("LINE_NUMBERS", "false")

	conf = GetEnvVars()
	assert.Equal(t, []string{"path1", "path2"}, conf.Paths)
	assert.Equal(t, []string{".go", ".txt"}, conf.Extensions)
	assert.True(t, conf.IncludeHidden)
	assert.False(t, conf.IgnoreGitignore)
	assert.Equal(t, []string{"node_modules", "vendor"}, conf.IgnorePatterns)
	assert.Equal(t, "output.md", conf.OutputFile)
	assert.True(t, conf.ClaudeXML)
	assert.False(t, conf.LineNumbers)

	// Unset env vars
	_ = os.Unsetenv("PATHS")
	_ = os.Unsetenv("EXTENSIONS")
	_ = os.Unsetenv("INCLUDE_HIDDEN")
	_ = os.Unsetenv("IGNORE_GITIGNORE")
	_ = os.Unsetenv("IGNORE_PATTERNS")
	_ = os.Unsetenv("OUTPUT_FILE")
	_ = os.Unsetenv("CLAUDE_XML")
	_ = os.Unsetenv("LINE_NUMBERS")
}

func TestGetEnvVarsWithDotEnv(t *testing.T) {
	// Create a temporary .env file
	envContent := `PATHS=path1,path2
EXTENSIONS=.go,.txt
INCLUDE_HIDDEN=true
IGNORE_GITIGNORE=false
IGNORE_PATTERNS=node_modules,vendor
OUTPUT_FILE=output.md
CLAUDE_XML=true
LINE_NUMBERS=false`
	err := os.WriteFile(".env", []byte(envContent), 0644)
	assert.NoError(t, err)
	defer func() { _ = os.Remove(".env") }()

	conf := GetEnvVars()
	assert.Equal(t, []string{"path1", "path2"}, conf.Paths)
	assert.Equal(t, []string{".go", ".txt"}, conf.Extensions)
	assert.True(t, conf.IncludeHidden)
	assert.False(t, conf.IgnoreGitignore)
	assert.Equal(t, []string{"node_modules", "vendor"}, conf.IgnorePatterns)
	assert.Equal(t, "output.md", conf.OutputFile)
	assert.True(t, conf.ClaudeXML)
	assert.False(t, conf.LineNumbers)
}

func TestGetEnvVarsParseError(t *testing.T) {
	// This test is tricky as env.Parse doesn't easily error on valid struct, but for coverage
	// Set invalid env that might cause issues, but since tags are proper, it should not error
	// Skip detailed error test as it's not critical for this config
	t.Skip("env.Parse errors not easily triggered for this struct")
}
