package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/toozej/terranotate/internal/generator"
	"github.com/toozej/terranotate/internal/parser"
	"github.com/toozej/terranotate/internal/validator"
)

// Generate creates markdown documentation from Terraform resources
func Generate(fs afero.Fs, path, schemaFile, outputFile string) error {
	fmt.Println("=================================================")
	fmt.Println("Terranotate - Generate Documentation")
	fmt.Println("=================================================")
	fmt.Printf("Path: %s\n", path)
	fmt.Printf("Schema: %s\n", schemaFile)
	if outputFile != "" {
		fmt.Printf("Output: %s\n", outputFile)
	} else {
		fmt.Println("Output: stdout")
	}
	fmt.Println()

	// Get schema for documentation
	schema, err := loadSchemaForGenerator(fs, schemaFile)
	if err != nil {
		return fmt.Errorf("failed to load schema for generator: %w", err)
	}

	// Determine if path is a file or directory
	info, err := fs.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	var allResources []parser.TerraformResource
	var moduleName string

	if info.IsDir() {
		// Find all Terraform files
		tfFiles, err := findTerraformFilesForGeneration(fs, path)
		if err != nil {
			return fmt.Errorf("failed to find Terraform files: %w", err)
		}

		if len(tfFiles) == 0 {
			return fmt.Errorf("no Terraform files found in: %s", path)
		}

		fmt.Printf("Found %d Terraform file(s)\n", len(tfFiles))

		// Parse all files
		prefixes := []string{"@metadata", "@docs", "@validation", "@config"}
		p := parser.NewCommentParser(fs, prefixes)

		for _, file := range tfFiles {
			resources, err := p.ParseFile(file)
			if err != nil {
				fmt.Printf("Warning: Failed to parse %s: %v\n", file, err)
				continue
			}
			allResources = append(allResources, resources...)
		}

		moduleName = filepath.Base(path)
	} else {
		// Single file
		prefixes := []string{"@metadata", "@docs", "@validation", "@config"}
		p := parser.NewCommentParser(fs, prefixes)

		resources, err := p.ParseFile(path)
		if err != nil {
			return fmt.Errorf("failed to parse file: %w", err)
		}

		allResources = resources
		moduleName = strings.TrimSuffix(filepath.Base(path), ".tf")
	}

	fmt.Printf("Parsed %d resource(s)\n\n", len(allResources))

	if len(allResources) == 0 {
		return fmt.Errorf("no resources found to document")
	}

	// Generate markdown
	gen := generator.NewMarkdownGenerator(schema)
	markdown := gen.GenerateDocumentation(moduleName, allResources)

	// Output markdown
	if outputFile != "" {
		// Write to file
		if err := afero.WriteFile(fs, outputFile, []byte(markdown), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("✅ Documentation written to: %s\n", outputFile)
	} else {
		// Write to stdout
		fmt.Println(strings.Repeat("=", 50))
		fmt.Println(markdown)
	}

	return nil
}

func findTerraformFilesForGeneration(fs afero.Fs, root string) ([]string, error) {
	var files []string
	err := afero.Walk(fs, root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			// Skip hidden and common ignore directories
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == ".terraform" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".tf") && !strings.HasSuffix(info.Name(), "_test.tf") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func loadSchemaForGenerator(fs afero.Fs, schemaFile string) (validator.ValidationSchema, error) {
	// Defer to fix.go's loadSchema function which already handles YAML unmarshaling
	return loadSchema(fs, schemaFile)
}
