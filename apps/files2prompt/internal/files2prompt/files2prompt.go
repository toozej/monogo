// Package files2prompt provides functionality for crawling directories,
// filtering files, and preparing content suitable for AI prompt ingestion.
package files2prompt

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-git/go-billy/v5/osfs"
	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
	log "github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/files2prompt/internal/config"
)

// Standard OS functions
var (
	osStdout io.Writer = os.Stdout
)

var extToLang = map[string]string{
	"py":   "python",
	"c":    "c",
	"cpp":  "cpp",
	"java": "java",
	"js":   "javascript",
	"ts":   "typescript",
	"html": "html",
	"css":  "css",
	"xml":  "xml",
	"json": "json",
	"yaml": "yaml",
	"yml":  "yaml",
	"sh":   "bash",
	"rb":   "ruby",
	"go":   "go",
}

func getBackticks(content string) string {
	backticks := "```"
	for strings.Contains(content, backticks) {
		backticks += "`"
	}
	return backticks
}

type pathFilter struct {
	config       config.Config
	scanRoot     string
	ignoreRoot   string
	ignore       gitignore.Matcher
	excludedPath map[string]struct{}
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}

	parent, parentErr := filepath.EvalSymlinks(filepath.Dir(abs))
	if parentErr != nil {
		if os.IsNotExist(parentErr) {
			return filepath.Clean(abs), nil
		}
		return "", parentErr
	}
	return filepath.Join(parent, filepath.Base(abs)), nil
}

func findIgnoreRoot(path string, isDir bool) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	start := abs
	if !isDir {
		start = filepath.Dir(abs)
	}

	for current := start; ; current = filepath.Dir(current) {
		if _, statErr := os.Stat(filepath.Join(current, ".git")); statErr == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return start, nil
}

func newPathFilter(path string, info os.FileInfo, cfg config.Config, excludedPaths []string) (*pathFilter, error) {
	scanRoot := path
	if !info.IsDir() {
		scanRoot = filepath.Dir(path)
	}
	absScanRoot, err := filepath.Abs(scanRoot)
	if err != nil {
		return nil, err
	}

	filter := &pathFilter{
		config:       cfg,
		scanRoot:     absScanRoot,
		excludedPath: make(map[string]struct{}, len(excludedPaths)),
	}
	for _, excluded := range excludedPaths {
		canonical, canonicalErr := canonicalPath(excluded)
		if canonicalErr != nil {
			return nil, canonicalErr
		}
		filter.excludedPath[canonical] = struct{}{}
	}

	if !cfg.IgnoreGitignore {
		filter.ignoreRoot, err = findIgnoreRoot(path, info.IsDir())
		if err != nil {
			return nil, err
		}
		patterns, patternErr := gitignore.ReadPatterns(osfs.New(filter.ignoreRoot), nil)
		if patternErr != nil {
			return nil, fmt.Errorf("read gitignore patterns: %w", patternErr)
		}
		filter.ignore = gitignore.NewMatcher(patterns)
	}

	return filter, nil
}

func hasHiddenComponent(path string) bool {
	for _, component := range strings.Split(filepath.ToSlash(path), "/") {
		if component != "." && component != ".." && strings.HasPrefix(component, ".") {
			return true
		}
	}
	return false
}

func (f *pathFilter) matchesCustomIgnore(filePath string, isDir bool) bool {
	relPath, err := filepath.Rel(f.scanRoot, filePath)
	if err != nil {
		relPath = filePath
	}
	relPath = filepath.ToSlash(relPath)
	base := filepath.Base(filePath)

	for _, patternGroup := range f.config.IgnorePatterns {
		for _, pattern := range strings.Split(patternGroup, ",") {
			pattern = strings.TrimSpace(filepath.ToSlash(pattern))
			if pattern == "" {
				continue
			}
			directoryOnly := strings.HasSuffix(pattern, "/")
			pattern = strings.TrimSuffix(pattern, "/")
			if directoryOnly && !isDir {
				if matched, _ := doublestar.Match(pattern+"/**", relPath); matched {
					return true
				}
				continue
			}
			baseMatch, _ := doublestar.Match(pattern, base)
			pathMatch, _ := doublestar.Match(pattern, relPath)
			if baseMatch || pathMatch {
				return true
			}
		}
	}
	return false
}

func (f *pathFilter) include(filePath string, info os.FileInfo) (bool, error) {
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return false, nil
	}
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return false, err
	}

	canonical, err := canonicalPath(absFilePath)
	if err != nil {
		return false, err
	}
	if _, excluded := f.excludedPath[canonical]; excluded {
		return false, nil
	}

	relScan, err := filepath.Rel(f.scanRoot, absFilePath)
	if err != nil {
		return false, err
	}
	if !f.config.IncludeHidden && hasHiddenComponent(relScan) {
		return false, nil
	}

	if f.ignore != nil {
		relIgnore, relErr := filepath.Rel(f.ignoreRoot, absFilePath)
		if relErr != nil {
			return false, relErr
		}
		if relIgnore != ".." && !strings.HasPrefix(relIgnore, ".."+string(filepath.Separator)) {
			parts := strings.Split(filepath.ToSlash(relIgnore), "/")
			if f.ignore.Match(parts, info.IsDir()) {
				return false, nil
			}
		}
	}

	if f.matchesCustomIgnore(absFilePath, info.IsDir()) {
		return false, nil
	}

	if !info.IsDir() && len(f.config.Extensions) > 0 {
		ext := filepath.Ext(filePath)
		for _, allowed := range f.config.Extensions {
			if ext == allowed {
				return true, nil
			}
		}
		return false, nil
	}

	return true, nil
}

func processPath(path string, cfg config.Config, writer io.Writer, excludedPaths []string, globalIndex *int) error {
	// Handle current directory case
	if path == "." {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %v", err)
		}
	}

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	filter, err := newPathFilter(path, info, cfg, excludedPaths)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		include, includeErr := filter.include(path, info)
		if includeErr != nil {
			return includeErr
		}
		if !include {
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return fmt.Errorf("refusing non-regular input file %s", path)
			}
			return nil
		}
		return processFile(path, cfg, writer, globalIndex)
	}

	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		include, includeErr := filter.include(filePath, info)
		if includeErr != nil {
			return includeErr
		}
		if !include {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() {
			return processFile(filePath, cfg, writer, globalIndex)
		}
		return nil
	})
}

func processFile(filePath string, config config.Config, writer io.Writer, globalIndex *int) error {
	content, err := os.ReadFile(filePath) // #nosec G304
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	lines := strings.Split(string(content), "\n")
	var processedContent strings.Builder

	// Process content with line numbers if enabled
	if config.LineNumbers {
		// Calculate padding for line numbers based on total lines
		padding := len(fmt.Sprintf("%d", len(lines)))
		format := fmt.Sprintf("%% %dd │ %%s\n", padding)

		for i, line := range lines {
			fmt.Fprintf(&processedContent, format, i+1, line)
		}
	} else {
		processedContent.WriteString(string(content))
	}

	switch {
	case config.Markdown:
		ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
		lang := extToLang[ext]
		contentStr := processedContent.String()
		backticks := getBackticks(contentStr)
		if contentStr != "" && !strings.HasSuffix(contentStr, "\n") {
			contentStr += "\n"
		}
		markdownOutput := fmt.Sprintf("%s\n%s%s\n%s%s\n", filePath, backticks, lang, contentStr, backticks)
		_, err = writer.Write([]byte(markdownOutput))
	case config.ClaudeXML:
		if _, err = fmt.Fprintf(writer, "<document index=\"%d\">\n<source>", *globalIndex); err != nil {
			return err
		}
		if err = xml.EscapeText(writer, []byte(filePath)); err != nil {
			return err
		}
		if _, err = io.WriteString(writer, "</source>\n<document_content>\n"); err != nil {
			return err
		}
		if err = xml.EscapeText(writer, []byte(processedContent.String())); err != nil {
			return err
		}
		_, err = io.WriteString(writer, "</document_content>\n</document>\n")
		(*globalIndex)++
	default:
		contentStr := processedContent.String()
		if contentStr != "" && !strings.HasSuffix(contentStr, "\n") {
			contentStr += "\n"
		}
		output := fmt.Sprintf("%s\n---\n%s---\n\n", filePath, contentStr)
		_, err = writer.Write([]byte(output))
	}

	return err
}

// Run executes the files2prompt logic using the provided config.
// It walks through each path, reads applicable files, and writes output
// either to stdout or a file depending on config.
func Run(config config.Config) error {
	log.Debugf("files2prompt pkg Run config config struct contains: %v\n", config)
	if config.Markdown && config.ClaudeXML {
		return errors.New("markdown and Claude XML output are mutually exclusive")
	}

	writer := osStdout
	var outputFile *os.File
	var temporaryPath string
	var finalOutputPath string
	excludedPaths := make([]string, 0, 2)

	if config.OutputFile != "" {
		canonicalOutput, err := canonicalPath(config.OutputFile)
		if err != nil {
			return fmt.Errorf("resolve output file: %w", err)
		}
		for _, input := range config.Paths {
			info, statErr := os.Lstat(input)
			if statErr != nil {
				continue
			}
			if info.IsDir() {
				continue
			}
			canonicalInput, canonicalErr := canonicalPath(input)
			if canonicalErr != nil {
				return fmt.Errorf("resolve input %s: %w", input, canonicalErr)
			}
			if canonicalInput == canonicalOutput {
				return fmt.Errorf("output file %s aliases input %s", config.OutputFile, input)
			}
		}

		outputFile, err = os.CreateTemp(filepath.Dir(canonicalOutput), ".files2prompt-*")
		if err != nil {
			return fmt.Errorf("create temporary output: %w", err)
		}
		temporaryPath = outputFile.Name()
		finalOutputPath = canonicalOutput
		excludedPaths = append(excludedPaths, canonicalOutput, temporaryPath)
		writer = outputFile
		defer func() {
			_ = outputFile.Close()
			_ = os.Remove(temporaryPath)
		}()
	}

	globalIndex := 1

	if config.ClaudeXML {
		if _, err := io.WriteString(writer, "<documents>\n"); err != nil {
			return fmt.Errorf("write XML header: %w", err)
		}
	}

	var processingErrors []error
	for _, path := range config.Paths {
		if err := processPath(path, config, writer, excludedPaths, &globalIndex); err != nil {
			processingErrors = append(processingErrors, fmt.Errorf("process %s: %w", path, err))
		}
	}

	if config.ClaudeXML {
		if _, err := io.WriteString(writer, "</documents>\n"); err != nil {
			processingErrors = append(processingErrors, fmt.Errorf("write XML footer: %w", err))
		}
	}
	if err := errors.Join(processingErrors...); err != nil {
		return err
	}

	if outputFile != nil {
		if err := outputFile.Sync(); err != nil {
			return fmt.Errorf("sync output: %w", err)
		}
		if err := outputFile.Close(); err != nil {
			return fmt.Errorf("close output: %w", err)
		}
		if err := os.Rename(temporaryPath, finalOutputPath); err != nil {
			return fmt.Errorf("replace output: %w", err)
		}
		temporaryPath = ""
	}
	return nil
}
