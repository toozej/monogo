# files2prompt

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/files2prompt)
[![Go Report Card](https://goreportcard.com/badge/github.com/toozej/files2prompt)](https://goreportcard.com/report/github.com/toozej/files2prompt)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/files2prompt/cicd.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/files2prompt)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/files2prompt/total)

A command-line tool that helps prepare files for AI prompts by crawling directories and outputting file contents with flexible filtering and formatting options. Heavily inspired by [Simon Willison's](https://simonwillison.net/) Python project [files-to-prompt](https://github.com/simonw/files-to-prompt).

## Features

- Recursive directory traversal
- File filtering by extension
- Support for .gitignore rules
- Hidden file/directory filtering
- Custom ignore patterns including for directories and/or files
- Optional line numbers in output
- Optional Claude-specific XML output format
- Optional Markdown output format with fenced code blocks
- Configurable via environment variables or command-line flags
- Output to file or stdout
- Sub-commands for version information and manual page generation

## Installation

```bash
make install
```

## Usage

Basic usage:
```bash
files2prompt [command] [flags] [paths...]
```

The tool requires at least one path argument for the main command. It will process all files in the specified paths according to the provided options.

### Flags

- `-e, --extension`: File extensions to include (can be specified multiple times)
- `--include-hidden`: Include hidden files and folders
- `--ignore-gitignore`: Ignore .gitignore files
- `--ignore`: Patterns to ignore (can be comma-separated or specified multiple times). Use '/' suffix to match directories only. Examples: '*.test.js', 'test/', 'path/to/ignore/', 'dir1/,dir2/'
- `-o, --output`: Output file path (defaults to stdout)
- `-c, --cxml`: Output in XML format for Claude
- `-n, --line-numbers`: Output line numbers
- `-m, --markdown`: Output in Markdown format with fenced code blocks
- `-0, --null`: Use NUL character as separator when reading from stdin
- `-d, --debug`: Enable debug-level logging

### Sub-commands

- `version`: Print version and build information in JSON format
- `man`: Generate Unix manual pages (hidden command)

### Examples

Process all files in the current directory:
```bash
files2prompt .
```

Process only Python files:
```bash
files2prompt -e .py ./src
```

Include hidden files and generate Claude-compatible XML:
```bash
files2prompt --include-hidden -c ./project
```

Output to a file instead of stdout:
```bash
files2prompt -o output.txt ./src ./tests
```

Ignore specific patterns:
```bash
files2prompt --ignore "*.test.js" --ignore "*.spec.js" ./src
```

Multiple ignore patterns:
```bash
files2prompt --ignore "*.test.js" --ignore "node_modules/" ./src
```

Comma-separated ignore patterns:
```bash
files2prompt --ignore "*.test.js,node_modules/,dist/" ./src
```

Ignore specific directories:
```bash
files2prompt --ignore "test/,build/" .
```

Output in Markdown format:
```bash
files2prompt --markdown ./src
```

Use NUL separator when reading from stdin:
```bash
echo -e "path1\x00path2" | files2prompt --null
```

## Configuration

The tool can be configured using either command-line flags or environment variables through a `.env` file. Environment variables take precedence over default values but can be overridden by command-line flags.

### Environment Variables

- `PATHS`: Comma-separated list of paths to process
- `EXTENSIONS`: Comma-separated list of file extensions to include
- `INCLUDE_HIDDEN`: Set to true to include hidden files/directories
- `IGNORE_GITIGNORE`: Set to true to ignore .gitignore rules
- `IGNORE_PATTERNS`: Comma-separated list of patterns to ignore
- `OUTPUT_FILE`: Path for the output file
- `CLAUDE_XML`: Set to true to output in Claude XML format
- `LINE_NUMBERS`: Set to true to display line numbers in output
- `MARKDOWN`: Set to true to output in Markdown format
- `NULL`: Set to true to use NUL character as separator when reading from stdin

## Output Formats

### Standard Format
```
 /path/to/file1
---
[file contents]
---

/path/to/file2
---
[file contents]
---
```

### Markdown Format (-m/--markdown)
```
/path/to/file1
```language
[file contents]
```

/path/to/file2
```language
[file contents]
```
```

### Claude XML Format (-c/--cxml)
```xml
<documents>
<document index="1">
<source>/path/to/file1</source>
<document_content>
[file contents]
</document_content>
</document>
<document index="2">
<source>/path/to/file2</source>
<document_content>
[file contents]
</document_content>
</document>
</documents>
```

## Building from Source

1. Clone the repository:
```bash
git clone https://github.com/toozej/files2prompt.git
```

2. Build the binary:
```bash
cd files2prompt
make local-build
```


## changes required to update golang version
- `make update-golang-version` 
