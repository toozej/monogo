// Package main provides diagram generation utilities for the terranotate project.
//
// This application generates architectural and component diagrams for the terranotate
// template using the go-diagrams library. It creates visual representations of the
// project structure and component relationships to aid in documentation and understanding.
//
// The generated diagrams are saved as .dot files in the docs/diagrams/go-diagrams/
// directory and can be converted to various image formats using Graphviz.
//
// Usage:
//
//	go run cmd/diagrams/main.go
//
// This will generate:
//   - architecture.dot: High-level architecture showing user interaction flow
//   - components.dot: Component relationships and dependencies
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/generic"
	"github.com/blushft/go-diagrams/nodes/programming"
)

// main is the entry point for the diagram generation utility.
//
// This function orchestrates the entire diagram generation process:
//  1. Creates the output directory structure
//  2. Changes to the appropriate working directory
//  3. Generates architecture and component diagrams
//  4. Reports successful completion
//
// The function will terminate with log.Fatal if any critical operation fails,
// such as directory creation, navigation, or diagram rendering.
func main() {
	// Ensure output directory exists
	if err := os.MkdirAll("docs/diagrams", 0750); err != nil {
		log.Fatal("Failed to create output directory:", err)
	}

	// Change to docs/diagrams directory
	if err := os.Chdir("docs/diagrams"); err != nil {
		log.Fatal("Failed to change directory:", err)
	}

	// Generate architecture diagram
	generateArchitectureDiagram()

	// Generate component diagram
	generateComponentDiagram()

	fmt.Println("Diagram .dot files generated successfully in ./docs/diagrams/go-diagrams/")
}

// generateArchitectureDiagram creates a high-level architecture diagram showing
// the interaction flow between users and the terranotate application components.
//
// The diagram illustrates:
//   - User interaction with the CLI application
//   - Configuration management flow
//   - Integration with the application logic (parser/validator/fixer)
//   - Logging system integration
//
// The diagram is rendered in top-to-bottom (TB) direction and saved as
// "architecture.dot" in the current working directory. The function will
// terminate the program with log.Fatal if diagram creation or rendering fails.
func generateArchitectureDiagram() {
	d, err := diagram.New(diagram.Filename("architecture"), diagram.Label("Terranotate Architecture"), diagram.Direction("TB"))
	if err != nil {
		log.Fatal(err)
	}

	// Define components
	user := generic.Blank.Blank(diagram.NodeLabel("User"))
	cli := programming.Language.Go(diagram.NodeLabel("CLI Application"))
	config := generic.Blank.Blank(diagram.NodeLabel("Configuration\n(env/godotenv)"))
	app := programming.Language.Go(diagram.NodeLabel("Application Logic\n(App/Parser/Validator/Fixer)"))
	logging := generic.Blank.Blank(diagram.NodeLabel("Logging\n(logrus)"))

	// Create connections
	d.Connect(user, cli, diagram.Forward())
	d.Connect(cli, config, diagram.Forward())
	d.Connect(cli, app, diagram.Forward())
	d.Connect(cli, logging, diagram.Forward())

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}

// generateComponentDiagram creates a detailed component diagram showing the
// relationships and dependencies between different packages in the terranotate project.
//
// The diagram illustrates:
//   - main.go as the entry point
//   - cmd/terranotate package handling CLI operations
//   - Integration with internal packages (app, parser, validator, fixer)
//   - Data flow between components
//
// The diagram is rendered in left-to-right (LR) direction and saved as
// "components.dot" in the current working directory. The function will
// terminate the program with log.Fatal if diagram creation or rendering fails.
func generateComponentDiagram() {
	d, err := diagram.New(diagram.Filename("components"), diagram.Label("Terranotate Components"), diagram.Direction("LR"))
	if err != nil {
		log.Fatal(err)
	}

	// Main components
	main := programming.Language.Go(diagram.NodeLabel("main.go"))
	rootCmd := programming.Language.Go(diagram.NodeLabel("cmd/terranotate\nroot.go"))
	config := programming.Language.Go(diagram.NodeLabel("pkg/config\nconfig.go"))

	// Internal packages
	app := programming.Language.Go(diagram.NodeLabel("internal/app"))
	parser := programming.Language.Go(diagram.NodeLabel("internal/parser"))
	validator := programming.Language.Go(diagram.NodeLabel("internal/validator"))
	fixer := programming.Language.Go(diagram.NodeLabel("internal/fixer"))

	// Other packages
	version := programming.Language.Go(diagram.NodeLabel("pkg/version\nversion.go"))
	man := programming.Language.Go(diagram.NodeLabel("pkg/man\nman.go"))

	// Create connections showing the flow
	d.Connect(main, rootCmd, diagram.Forward())
	d.Connect(rootCmd, config, diagram.Forward())
	d.Connect(rootCmd, app, diagram.Forward())
	d.Connect(rootCmd, version, diagram.Forward())
	d.Connect(rootCmd, man, diagram.Forward())

	// App Logic connections
	d.Connect(app, parser, diagram.Forward())
	d.Connect(app, validator, diagram.Forward())
	d.Connect(app, fixer, diagram.Forward())

	// Internal dependencies
	// Fixer uses parser and validator
	d.Connect(fixer, parser, diagram.Forward())
	d.Connect(fixer, validator, diagram.Forward())

	// Validator uses parser
	d.Connect(validator, parser, diagram.Forward())

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}
