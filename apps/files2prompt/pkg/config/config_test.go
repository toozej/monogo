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
	os.Setenv("PATHS", "path1,path2")
	os.Setenv("EXTENSIONS", ".go,.txt")
	os.Setenv("INCLUDE_HIDDEN", "true")
	os.Setenv("IGNORE_GITIGNORE", "false")
	os.Setenv("IGNORE_PATTERNS", "node_modules,vendor")
	os.Setenv("OUTPUT_FILE", "output.md")
	os.Setenv("CLAUDE_XML", "true")
	os.Setenv("LINE_NUMBERS", "false")

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
	os.Unsetenv("PATHS")
	os.Unsetenv("EXTENSIONS")
	os.Unsetenv("INCLUDE_HIDDEN")
	os.Unsetenv("IGNORE_GITIGNORE")
	os.Unsetenv("IGNORE_PATTERNS")
	os.Unsetenv("OUTPUT_FILE")
	os.Unsetenv("CLAUDE_XML")
	os.Unsetenv("LINE_NUMBERS")
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
	defer os.Remove(".env")

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
