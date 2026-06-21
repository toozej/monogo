package app

import (
	"fmt"

	"github.com/spf13/afero"
	"github.com/toozej/terranotate/internal/parser"
)

// Parse implements the parse command logic
func Parse(fs afero.Fs, filename string) error {
	fmt.Println("=================================================")
	fmt.Println("Terranotate - Terraform Comment Parser")
	fmt.Println("\n=================================================")

	// Define the comment prefixes you want to parse
	prefixes := []string{"@metadata", "@docs", "@validation", "@config"}

	p := parser.NewCommentParser(fs, prefixes)

	// Parse the Terraform file
	resources, err := p.ParseFile(filename)
	if err != nil {
		return fmt.Errorf("error parsing file: %w", err)
	}

	fmt.Printf("Found %d resources in %s\n\n", len(resources), filename)

	for _, resource := range resources {
		fmt.Printf("\n📦 Resource: %s.%s (lines %d-%d)\n",
			resource.Type, resource.Name, resource.StartLine, resource.EndLine)

		if len(resource.PrecedingComments) > 0 {
			fmt.Println("\n  📝 Preceding Comments:")
			for _, comment := range resource.PrecedingComments {
				fmt.Printf("    [Lines %d-%d] %s\n", comment.Line, comment.EndLine, comment.Prefix)
				printFields(comment.Fields, "      ")
			}
		}

		if len(resource.InlineComments) > 0 {
			fmt.Println("\n  💬 Inline Comments:")
			for _, comment := range resource.InlineComments {
				fmt.Printf("    [Lines %d-%d] %s\n", comment.Line, comment.EndLine, comment.Prefix)
				printFields(comment.Fields, "      ")
			}
		}
	}

	return nil
}

// printFields recursively prints nested field structures
func printFields(fields map[string]interface{}, indent string) {
	for k, v := range fields {
		if k == "_content" {
			continue // Skip the raw content field in detailed output
		}

		switch val := v.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indent, k)
			printFields(val, indent+"  ")
		case []interface{}:
			fmt.Printf("%s%s: %v\n", indent, k, val)
		default:
			fmt.Printf("%s%s: %v\n", indent, k, val)
		}
	}
}
