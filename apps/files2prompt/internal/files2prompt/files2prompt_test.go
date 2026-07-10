package files2prompt

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/toozej/monogo/apps/files2prompt/internal/config"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
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
			expected:    "testdata/file1.txt\n---\nline 1\nline 2\nline 3\n---\n\n",
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
			expected:    "<document index=\"1\">\n<source>testdata/file4.txt</source>\n<document_content>\n 1 │ line 1&#xA; 2 │ line 2&#xA;</document_content>\n</document>\n",
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
			expectedErr: true,
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
			expected:    "testdata/file1.txt\n```\nline 1\nline 2\nline 3\n```\n",
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
			expected:    "testdata/test_project/docs/README.txt\n---\nHello world\n---\n\ntestdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\ntestdata/test_project/temp/file.txt\n---\ntemp file\n---\n\n",
			expectedErr: false,
		},
		{
			name: "directory with gitignore",
			path: "testdata/test_project",
			config: config.Config{
				Extensions: []string{".go"},
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
			expected:    "testdata/test_project/docs/README.txt\n---\nHello world\n---\n\ntestdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\n",
			expectedErr: false,
		},
		{
			name: "include hidden files",
			path: "testdata/test_project",
			config: config.Config{
				IncludeHidden: true,
				Extensions:    []string{".go"},
			},
			expected:    "testdata/test_project/.hidden.go\n---\nhidden code\n---\n\ntestdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\n",
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
			expected:    "testdata/empty.txt\n---\n---\n\ntestdata/file1.txt\n---\nline 1\nline 2\nline 3\n---\n\ntestdata/file2.txt\n---\nfirst line\nsecond line\n---\n\ntestdata/file3.txt\n---\nxml content\n---\n\ntestdata/file4.txt\n---\nline 1\nline 2\n---\n\ntestdata/test_project/docs/README.txt\n---\nHello world\n---\n\ntestdata/test_project/temp/file.txt\n---\ntemp file\n---\n\n",
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			globalIndex := 1

			err := processPath(tt.path, tt.config, &buf, nil, &globalIndex)

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
			expected:    "<documents>\n<document index=\"1\">\n<source>testdata/test_project/src/main.go</source>\n<document_content>\npackage main&#xA;&#xA;func main() {}&#xA;</document_content>\n</document>\n</documents>\n",
			expectedErr: false,
		},
		{
			name: "run with multiple paths",
			config: config.Config{
				Paths:      []string{"testdata/test_project/src", "testdata/test_project/docs"},
				Extensions: []string{".go", ".txt"},
			},
			expected:    "testdata/test_project/src/main.go\n---\npackage main\n\nfunc main() {}\n---\n\ntestdata/test_project/docs/README.txt\n---\nHello world\n---\n\n",
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
				Paths:      []string{"testdata/test_project"},
				Extensions: []string{".go"},
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

func TestGitignoreDefaultsAndSemantics(t *testing.T) {
	root := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "secrets"), 0o755))
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "nested"), 0o755))
	assert.NoError(t, os.MkdirAll(filepath.Join(root, "sibling"), 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("secrets/*.txt\n!secrets/keep.txt\n/root-only.txt\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "nested", ".gitignore"), []byte("*.tmp\n"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "secrets", "token.txt"), []byte("SECRET"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "secrets", "keep.txt"), []byte("KEEP"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "root-only.txt"), []byte("ROOT"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "nested", "ignored.tmp"), []byte("NESTED"), 0o600))
	assert.NoError(t, os.WriteFile(filepath.Join(root, "sibling", "visible.tmp"), []byte("SIBLING"), 0o600))

	var output bytes.Buffer
	originalStdout := osStdout
	osStdout = &output
	t.Cleanup(func() { osStdout = originalStdout })

	assert.NoError(t, Run(config.Config{Paths: []string{root}, IncludeHidden: true}))
	assert.NotContains(t, output.String(), "SECRET")
	assert.NotContains(t, output.String(), "ROOT")
	assert.NotContains(t, output.String(), "NESTED")
	assert.Contains(t, output.String(), "KEEP")
	assert.Contains(t, output.String(), "SIBLING")

	output.Reset()
	assert.NoError(t, Run(config.Config{Paths: []string{root}, IgnoreGitignore: true}))
	assert.Contains(t, output.String(), "SECRET")
	assert.Contains(t, output.String(), "ROOT")
	assert.Contains(t, output.String(), "NESTED")
}

func TestSymlinksAreNotFollowed(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "secret.txt")
	assert.NoError(t, os.WriteFile(external, []byte("EXTERNAL SECRET"), 0o600))
	link := filepath.Join(root, "linked-secret.txt")
	if err := os.Symlink(external, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	var output bytes.Buffer
	index := 1
	assert.NoError(t, processPath(root, config.Config{}, &output, nil, &index))
	assert.NotContains(t, output.String(), "EXTERNAL SECRET")

	output.Reset()
	assert.Error(t, processPath(link, config.Config{}, &output, nil, &index))
}

func TestOutputIsAtomicAndNeverReadAsInput(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input.txt")
	output := filepath.Join(root, "output.txt")
	assert.NoError(t, os.WriteFile(input, []byte("INPUT"), 0o600))
	assert.NoError(t, os.WriteFile(output, []byte("OLD OUTPUT"), 0o600))

	err := Run(config.Config{Paths: []string{input}, OutputFile: input})
	assert.Error(t, err)
	content, readErr := os.ReadFile(input)
	assert.NoError(t, readErr)
	assert.Equal(t, "INPUT", string(content))

	assert.NoError(t, Run(config.Config{Paths: []string{root}, OutputFile: output, Extensions: []string{".txt"}}))
	content, readErr = os.ReadFile(output)
	assert.NoError(t, readErr)
	assert.Contains(t, string(content), "INPUT")
	assert.NotContains(t, string(content), "OLD OUTPUT")
	assert.NotContains(t, string(content), ".files2prompt-")

	assert.NoError(t, os.WriteFile(output, []byte("STILL VALID"), 0o600))
	err = Run(config.Config{Paths: []string{filepath.Join(root, "missing.txt")}, OutputFile: output})
	assert.Error(t, err)
	content, readErr = os.ReadFile(output)
	assert.NoError(t, readErr)
	assert.Equal(t, "STILL VALID", string(content))
}

func TestClaudeXMLIsEscapedAndFormatsAreExclusive(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "special&name.txt")
	assert.NoError(t, os.WriteFile(input, []byte("<tag>&value"), 0o600))

	var output bytes.Buffer
	originalStdout := osStdout
	osStdout = &output
	t.Cleanup(func() { osStdout = originalStdout })
	assert.NoError(t, Run(config.Config{Paths: []string{input}, ClaudeXML: true}))

	var parsed struct {
		Documents []struct {
			Source  string `xml:"source"`
			Content string `xml:"document_content"`
		} `xml:"document"`
	}
	assert.NoError(t, xml.Unmarshal(output.Bytes(), &parsed))
	if assert.Len(t, parsed.Documents, 1) {
		assert.Equal(t, input, parsed.Documents[0].Source)
		assert.Equal(t, "\n<tag>&value", parsed.Documents[0].Content)
	}

	assert.Error(t, Run(config.Config{Paths: []string{input}, ClaudeXML: true, Markdown: true}))
}

func TestExplicitFilesHonorFilters(t *testing.T) {
	root := t.TempDir()
	assert.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.txt\n"), 0o600))
	ignored := filepath.Join(root, "ignored.txt")
	hidden := filepath.Join(root, ".hidden.txt")
	assert.NoError(t, os.WriteFile(ignored, []byte("IGNORED"), 0o600))
	assert.NoError(t, os.WriteFile(hidden, []byte("HIDDEN"), 0o600))

	for _, test := range []struct {
		name string
		path string
		cfg  config.Config
	}{
		{name: "extension", path: ignored, cfg: config.Config{Extensions: []string{".go"}, IgnoreGitignore: true}},
		{name: "gitignore", path: ignored, cfg: config.Config{}},
		{name: "hidden", path: hidden, cfg: config.Config{}},
		{name: "custom ignore", path: ignored, cfg: config.Config{IgnoreGitignore: true, IgnorePatterns: []string{"ignored.txt"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			index := 1
			assert.NoError(t, processPath(test.path, test.cfg, &output, nil, &index))
			assert.Empty(t, output.String())
		})
	}
}

func TestRunPropagatesProcessingAndWriterErrors(t *testing.T) {
	originalStdout := osStdout
	t.Cleanup(func() { osStdout = originalStdout })

	osStdout = io.Discard
	assert.Error(t, Run(config.Config{Paths: []string{filepath.Join(t.TempDir(), "missing.txt")}}))

	input := filepath.Join(t.TempDir(), "input.txt")
	assert.NoError(t, os.WriteFile(input, []byte("content"), 0o600))
	osStdout = failingWriter{}
	assert.Error(t, Run(config.Config{Paths: []string{input}}))
}
