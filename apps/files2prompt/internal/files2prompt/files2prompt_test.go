package files2prompt

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/toozej/files2prompt/pkg/config"
)

func TestReadGitignore(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{
			name:     "valid gitignore file",
			path:     "testdata/gitignore_valid",
			expected: []string{"*.log", "node_modules/", "dist/", "temp/"},
		},
		{
			name:     "gitignore with empty lines and comments",
			path:     "testdata/gitignore_valid",
			expected: []string{"*.log", "node_modules/", "dist/", "temp/"},
		},
		{
			name:     "non-existent gitignore file",
			path:     "testdata/gitignore_nonexistent",
			expected: nil,
		},
		{
			name:     "empty gitignore file",
			path:     "testdata/gitignore_empty",
			expected: nil,
		},
		{
			name:     "gitignore with only comments and empty lines",
			path:     "testdata/gitignore_comments_only",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := readGitignore(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		gitignoreRules []string
		expected       bool
	}{
		{
			name:           "exact filename match",
			path:           "test.log",
			gitignoreRules: []string{"*.log"},
			expected:       true,
		},
		{
			name:           "directory match",
			path:           "node_modules",
			gitignoreRules: []string{"node_modules/"},
			expected:       true,
		},
		{
			name:           "directory path match",
			path:           "node_modules/package",
			gitignoreRules: []string{"node_modules/"},
			expected:       true,
		},
		{
			name:           "no match",
			path:           "src/main.go",
			gitignoreRules: []string{"*.log", "node_modules/"},
			expected:       false,
		},
		{
			name:           "empty rules",
			path:           "any/path",
			gitignoreRules: []string{},
			expected:       false,
		},
		{
			name:           "hidden file match",
			path:           ".DS_Store",
			gitignoreRules: []string{".DS_Store"},
			expected:       true,
		},
		{
			name:           "pattern with multiple wildcards",
			path:           "src/main.go",
			gitignoreRules: []string{"src/*.go"},
			expected:       true,
		},
		{
			name:           "directory pattern without trailing slash",
			path:           "temp/files",
			gitignoreRules: []string{"temp"},
			expected:       true,
		},
		{
			name:           "temp directory with trailing slash pattern",
			path:           "testdata/test_project/temp",
			gitignoreRules: []string{"*.log", "node_modules/", "temp/"},
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldIgnore(tt.path, tt.gitignoreRules)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessFile(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		config      config.Config
		expected    string
		expectedErr bool
	}{
		{
			name:     "simple file without line numbers",
			filePath: "testdata/file1.txt",
			config: config.Config{
				LineNumbers: false,
				ClaudeXML:   false,
			},
			expected:    "testdata/file1.txt\n---\nline 1\nline 2\nline 3---\n\n",
			expectedErr: false,
		},
		{
			name:     "file with line numbers",
			filePath: "testdata/file2.txt",
			config: config.Config{
				LineNumbers: true,
				ClaudeXML:   false,
			},
			expected:    "testdata/file2.txt\n---\n 1 │ first line\n 2 │ second line\n---\n\n",
			expectedErr: false,
		},
		{
			name:     "file with Claude XML format",
			filePath: "testdata/file3.txt",
			config: config.Config{
				LineNumbers: false,
				ClaudeXML:   true,
			},
			expected:    "<document index=\"1\">\n<source>testdata/file3.txt</source>\n<document_content>\nxml content</document_content>\n</document>\n",
			expectedErr: false,
		},
		{
			name:     "file with line numbers and Claude XML",
			filePath: "testdata/file4.txt",
			config: config.Config{
				LineNumbers: true,
				ClaudeXML:   true,
			},
			expected:    "<document index=\"1\">\n<source>testdata/file4.txt</source>\n<document_content>\n 1 │ line 1\n 2 │ line 2\n</document_content>\n</document>\n",
			expectedErr: false,
		},
		{
			name:     "non-existent file",
			filePath: "testdata/nonexistent.txt",
			config: config.Config{
				LineNumbers: false,
				ClaudeXML:   false,
			},
			expected:    "",
			expectedErr: false, // Function logs warning and returns nil
		},
		{
			name:     "empty file",
			filePath: "testdata/empty.txt",
			config: config.Config{
				LineNumbers: false,
				ClaudeXML:   false,
			},
			expected:    "testdata/empty.txt\n---\n---\n\n",
			expectedErr: false,
		},
		{
			name:     "file with Markdown format",
			filePath: "testdata/file1.txt",
			config: config.Config{
				LineNumbers: false,
				ClaudeXML:   false,
				Markdown:    true,
			},
			expected:    "testdata/file1.txt\n```\nline 1\nline 2\nline 3```\n",
			expectedErr: false,
		},
		{
			name:     "file with line numbers and Markdown",
			filePath: "testdata/file2.txt",
			config: config.Config{
				LineNumbers: true,
				ClaudeXML:   false,
				Markdown:    true,
			},
			expected:    "testdata/file2.txt\n```\n 1 │ first line\n 2 │ second line\n```\n",
			expectedErr: false,
		},
		{
			name:     "Go file with Markdown",
			filePath: "testdata/test_project/src/main.go",
			config: config.Config{
				LineNumbers: false,
				ClaudeXML:   false,
				Markdown:    true,
			},
			expected:    "testdata/test_project/src/main.go\n```go\npackage main\n\nfunc main() {}\n```\n",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			globalIndex := 1

			err := processFile(tt.filePath, tt.config, &buf, &globalIndex)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, buf.String())
			}
		})
	}
}

func TestProcessPath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		config      config.Config
		expected    string
		expectedErr bool
	}{
		{
			name: "single file",
			path: "testdata/test_project/src/main.go",
			config: config.Config{
				Extensions: []string{".go"},
			},
			expected:    "testdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\n",
			expectedErr: false,
		},
		{
			name: "directory with multiple files",
			path: "testdata/test_project",
			config: config.Config{
				Extensions: []string{".go", ".txt"},
			},
			expected:    "testdata/test_project/docs/README.txt\n---\nHello world---\n\ntestdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\ntestdata/test_project/temp/file.txt\n---\ntemp file---\n\n",
			expectedErr: false,
		},
		{
			name: "directory with gitignore",
			path: "testdata/test_project",
			config: config.Config{
				IgnoreGitignore: true,
				Extensions:      []string{".go"},
			},
			expected:    "testdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\n",
			expectedErr: false,
		},
		{
			name: "directory with ignore patterns",
			path: "testdata/test_project",
			config: config.Config{
				IgnorePatterns: []string{"*.log", "temp/"},
				Extensions:     []string{".go", ".txt"},
			},
			expected:    "testdata/test_project/docs/README.txt\n---\nHello world---\n\ntestdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\n",
			expectedErr: false,
		},
		{
			name: "include hidden files",
			path: "testdata/test_project",
			config: config.Config{
				IncludeHidden: true,
				Extensions:    []string{".go"},
			},
			expected:    "testdata/test_project/.hidden.go\n---\nhidden code---\n\ntestdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\n",
			expectedErr: false,
		},
		{
			name: "exclude hidden files",
			path: "testdata/test_project",
			config: config.Config{
				IncludeHidden: false,
				Extensions:    []string{".go"},
			},
			expected:    "testdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\n",
			expectedErr: false,
		},
		{
			name: "current directory (dot)",
			path: "testdata",
			config: config.Config{
				Extensions: []string{".txt"},
			},
			expected:    "testdata/empty.txt\n---\n---\n\ntestdata/file1.txt\n---\nline 1\nline 2\nline 3---\n\ntestdata/file2.txt\n---\nfirst line\nsecond line---\n\ntestdata/file3.txt\n---\nxml content---\n\ntestdata/file4.txt\n---\nline 1\nline 2---\n\ntestdata/test_project/docs/README.txt\n---\nHello world---\n\ntestdata/test_project/temp/file.txt\n---\ntemp file---\n\n",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			var gitignoreRules []string
			globalIndex := 1

			err := processPath(tt.path, tt.config, &buf, gitignoreRules, &globalIndex)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, buf.String())
			}
		})
	}
}
func TestRun(t *testing.T) {
	tests := []struct {
		name        string
		config      config.Config
		expected    string
		expectedErr bool
	}{
		{
			name: "basic run with single file",
			config: config.Config{
				Paths:      []string{"testdata/test_project/src/main.go"},
				Extensions: []string{".go"},
			},
			expected:    "testdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\n",
			expectedErr: false,
		},
		{
			name: "run with Claude XML format",
			config: config.Config{
				Paths:      []string{"testdata/test_project/src/main.go"},
				ClaudeXML:  true,
				Extensions: []string{".go"},
			},
			expected:    "<documents>\n<document index=\"1\">\n<source>testdata/test_project/src/main.go</source>\n<document_content>\npackage main\n\nfunc main() {}\n</document_content>\n</document>\n</documents>\n",
			expectedErr: false,
		},
		{
			name: "run with multiple paths",
			config: config.Config{
				Paths:      []string{"testdata/test_project/src", "testdata/test_project/docs"},
				Extensions: []string{".go", ".txt"},
			},
			expected:    "testdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\ntestdata/test_project/docs/README.txt\n---\nHello world---\n\n",
			expectedErr: false,
		},
		{
			name: "run with output file",
			config: config.Config{
				Paths:      []string{"testdata/test_project/src/main.go"},
				OutputFile: "/tmp/output.txt",
				Extensions: []string{".go"},
			},
			expected:    "", // Output goes to file, not stdout
			expectedErr: false,
		},
		{
			name: "run with gitignore",
			config: config.Config{
				Paths:           []string{"testdata/test_project"},
				IgnoreGitignore: true,
				Extensions:      []string{".go"},
			},
			expected:    "testdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\n",
			expectedErr: false,
		},
		{
			name: "run with Markdown format",
			config: config.Config{
				Paths:      []string{"testdata/test_project/src/main.go"},
				Markdown:   true,
				Extensions: []string{".go"},
			},
			expected:    "testdata/test_project/src/main.go\n```go\npackage main\n\nfunc main() {}\n```\n",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			// Redirect stdout for testing
			originalStdout := osStdout
			osStdout = &buf
			defer func() { osStdout = originalStdout }()

			err := Run(tt.config)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, buf.String())
			}
		})
	}
}
