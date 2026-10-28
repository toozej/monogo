# files2prompt

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/toozej/monogo)
![GitHub Actions CI Workflow Status](https://img.shields.io/github/actions/workflow/status/toozej/monogo/ci.yaml)
![Docker Pulls](https://img.shields.io/docker/pulls/toozej/files2prompt)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/toozej/monogo/total)

<img src="img/avatar.png" alt="files2prompt avatar" style="background-color: #FFFFFF;" />

This command-line tool traverses directories and outputs file contents to prepare AI prompts. It offers flexible filters and output formats. Heavily inspired by [Simon Willison's](https://simonwillison.net/) Python project [files-to-prompt](https://github.com/simonw/files-to-prompt).

## Features

- Recursive directory traversal
- Filter files by extension
- Support for .gitignore rules
- Filter hidden files or directories
- Custom ignore patterns for directories, files, or both
- Optional line numbers in output
- Optional XML output for Claude
- Optional output in Markdown with fenced code blocks
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

The tool requires at least one path argument for the main command. It processes all files at the input paths with the options you select.

### Flags

- `-e, --extension`: File extensions to include (you can repeat this option)
- `--include-hidden`: Include hidden files and folders
- `--ignore-gitignore`: Ignore .gitignore files
- `--ignore`: Patterns to ignore. Separate patterns with commas or repeat the option.

Append `/` to match directories only.
Examples: `*.test.js`, `test/`, `path/to/ignore/`, `dir1/,dir2/`.
- `-o, --output`: Output file path (defaults to stdout)
- `-c, --cxml`: Output in XML format for Claude
- `-n, --line-numbers`: Output line numbers
- `-m, --markdown`: Output in Markdown format with fenced code blocks
- `-0, --null`: Use NUL character as separator when reading from stdin
- `-d, --debug`: Enable debug-level log output

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

Configure the tool with command-line flags or environment variables through a `.env` file.
Environment variables override default values. Command-line flags override environment variables.

### Environment Variables

- `PATHS`: Comma-separated list of paths to process
- `EXTENSIONS`: Comma-separated list of file extensions to include
- `INCLUDE_HIDDEN`: Set to true to include hidden files and directories
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

## Build from Source

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
