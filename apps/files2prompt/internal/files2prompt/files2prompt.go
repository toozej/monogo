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
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
	log "github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/files2prompt/internal/config"
)

// Standard OS functions
var (
	osStdout           io.Writer = os.Stdout
	readIgnorePatterns           = gitignore.ReadPatterns
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
	excludedFile []os.FileInfo
}

type readErrorFilesystem struct {
	billy.Filesystem
	err error
}

func (f *readErrorFilesystem) Open(filename string) (billy.File, error) {
	file, err := f.Filesystem.Open(filename)
	if err != nil && !os.IsNotExist(err) {
		f.err = errors.Join(f.err, fmt.Errorf("open %s: %w", filename, err))
	}
	return file, err
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

func newPathFilter(path string, info os.FileInfo, cfg config.Config, excludedPaths []string, ignoreCache map[string]gitignore.Matcher) (*pathFilter, error) {
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
		info, statErr := os.Stat(canonical)
		if statErr == nil {
			filter.excludedFile = append(filter.excludedFile, info)
		} else if !os.IsNotExist(statErr) {
			return nil, statErr
		}
	}

	if !cfg.IgnoreGitignore {
		filter.ignoreRoot, err = findIgnoreRoot(path, info.IsDir())
		if err != nil {
			return nil, err
		}
		if matcher, ok := ignoreCache[filter.ignoreRoot]; ok {
			filter.ignore = matcher
		} else {
			filesystem := &readErrorFilesystem{Filesystem: osfs.New(filter.ignoreRoot)}
			patterns, patternErr := readIgnorePatterns(filesystem, nil)
			if patternErr != nil {
				return nil, fmt.Errorf("read gitignore patterns: %w", patternErr)
			}
			if filesystem.err != nil {
				return nil, fmt.Errorf("read gitignore patterns: %w", filesystem.err)
			}
			filter.ignore = gitignore.NewMatcher(patterns)
			if ignoreCache != nil {
				ignoreCache[filter.ignoreRoot] = filter.ignore
			}
		}
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
	for _, excluded := range f.excludedFile {
		if os.SameFile(info, excluded) {
			return false, nil
		}
	}

	relScan, err := filepath.Rel(f.scanRoot, absFilePath)
	if err != nil {
		return false, err
	}
	if !f.config.IncludeHidden {
		hiddenRoot := relScan == "." && strings.HasPrefix(filepath.Base(absFilePath), ".")
		if hiddenRoot || hasHiddenComponent(relScan) {
			return false, nil
		}
	}

	if f.ignore != nil {
		relIgnore, relErr := filepath.Rel(f.ignoreRoot, absFilePath)
		if relErr != nil {
			return false, relErr
		}
		if relIgnore != ".." && !strings.HasPrefix(relIgnore, ".."+string(filepath.Separator)) {
			parts := strings.Split(filepath.ToSlash(relIgnore), "/")
			if len(parts) > 0 && parts[0] == ".git" {
				return false, nil
			}
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
	return processPathWithCache(path, cfg, writer, excludedPaths, globalIndex, nil)
}

func processPathWithCache(path string, cfg config.Config, writer io.Writer, excludedPaths []string, globalIndex *int, ignoreCache map[string]gitignore.Matcher) error {
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
	filter, err := newPathFilter(path, info, cfg, excludedPaths, ignoreCache)
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
		outputInfo, outputStatErr := os.Stat(canonicalOutput)
		if outputStatErr != nil && !os.IsNotExist(outputStatErr) {
			return fmt.Errorf("stat output file: %w", outputStatErr)
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
			aliasesOutput := canonicalInput == canonicalOutput
			if outputStatErr == nil {
				inputInfo, inputStatErr := os.Stat(canonicalInput)
				if inputStatErr != nil {
					return fmt.Errorf("stat input %s: %w", input, inputStatErr)
				}
				aliasesOutput = aliasesOutput || os.SameFile(inputInfo, outputInfo)
			}
			if aliasesOutput {
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
	ignoreCache := make(map[string]gitignore.Matcher)
	for _, path := range config.Paths {
		if err := processPathWithCache(path, config, writer, excludedPaths, &globalIndex, ignoreCache); err != nil {
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
