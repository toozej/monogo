# sanitize [![GoDoc](https://godoc.org/github.com/kennygrant/sanitize?status.svg)](https://godoc.org/github.com/kennygrant/sanitize) [![Go Report Card](https://goreportcard.com/badge/github.com/kennygrant/sanitize)](https://goreportcard.com/report/github.com/kennygrant/sanitize)

Package sanitize provides functions to sanitize html and paths with go (golang).

FUNCTIONS

```go
sanitize.Accents(s string) string
```

Accents replaces a set of accented characters with ascii equivalents.

```go
sanitize.BaseName(s string) string
```

`BaseName` sanitizes a string for use as a filename.
It replaces `.` or `/` with `-` in the basename.
Unlike `Name`, it does not attempt to normalize text as a path.

```go
sanitize.HTML(s string) string
```

`HTML` strips HTML tags with a simple parser, replaces common entities, and escapes `<` and `>` in the result.
Use the result as plain text.

```go
sanitize.HTMLAllowing(s string, args...[]string) (string, error)
```

`HTMLAllowing` parses HTML and permits tags and attributes from optional lists in `args`.
The `args[0]` list specifies allowed tags. The `args[1]` list specifies allowed attributes.
If either list is absent, the function uses default sets.

```go
sanitize.Name(s string) string
```

`Name` sanitizes a string for use as a filename.
It finds the path basename and replaces non-ASCII characters.

```go
sanitize.Path(s string) string
```

Path makes a string safe to use as an url path.

## Changes

Version 1.2

- Adjusted the HTML function to avoid a linter diagnostic.
- Added more tests from https://githubengineering.com/githubs-post-csp-journey/
- Changed the license filename.
- Added badges and a changelog to the README.

Version 1.1 Fixed type in comments. Merge pull request from Povilas
Balzaravicius Pawka

- replace br tags with newline even when they contain a space

Version 1.0 First release
