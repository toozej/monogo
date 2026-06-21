# Troubleshooting

## Issue: "package not found"
```bash
go mod tidy
go get github.com/hashicorp/hcl/v2
```

## Issue: "cannot find parser functions"
Make sure both `parser.go` and `main.go` are in the same package and directory, or properly imported if using the package structure.

## Issue: "file not found"
Ensure `example.tf` is in the correct directory (e.g., `examples/example.tf`).
