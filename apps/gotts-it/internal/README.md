# `/internal`

This directory contains private application and library code that other projects must not import.
The Go compiler enforces this restriction. You can create multiple `internal` directories at any level of the project tree.

You can optionally separate shared internal code from internal code that only one app uses.
This structure is optional, especially for small projects. It gives readers visual clues about the intended package use.
Put application code in `/internal/app`, for example `/internal/app/myapp`.
Put code that these apps share in `/internal/pkg`, for example `/internal/pkg/myprivlib`.
