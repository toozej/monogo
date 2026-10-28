# Resolve Problems

## Issue: "package not found"
```bash
go mod tidy
go get github.com/hashicorp/hcl/v2
```

## Issue: "cannot find parser functions"
Keep `parser.go` and `main.go` in the same package and directory.
If you use the package structure, import them correctly.

## Issue: "file not found"
Ensure `example.tf` is in the correct directory (for example `examples/example.tf`).
