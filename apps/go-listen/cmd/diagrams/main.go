// Package main provides diagram generation utilities for the go-listen project.
//
// This application generates architectural and component diagrams for the go-listen
// Spotify playlist management application using the go-diagrams library. It creates
// visual representations of the project structure and component relationships to aid
// in documentation and understanding.
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
	if err := os.MkdirAll("docs/diagrams", 0o750); err != nil {
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
// the interaction flow between users and the go-listen application components.
//
// The diagram illustrates:
//   - User interaction with the web interface
//   - HTTP server handling requests
//   - Spotify API integration for playlist management
//   - Artist search and playlist creation flow
//   - Configuration and logging systems
//
// The diagram is rendered in top-to-bottom (TB) direction and saved as
// "architecture.dot" in the current working directory. The function will
// terminate the program with log.Fatal if diagram creation or rendering fails.
func generateArchitectureDiagram() {
	d, err := diagram.New(diagram.Filename("architecture"), diagram.Label("Go-Listen Architecture"), diagram.Direction("TB"))
	if err != nil {
		log.Fatal(err)
	}

	// Define components
	user := generic.Blank.Blank(diagram.NodeLabel("User"))
	webUI := generic.Blank.Blank(diagram.NodeLabel("Web Interface\n(HTML/JS)"))
	httpServer := programming.Language.Go(diagram.NodeLabel("HTTP Server\n(Gin/Echo)"))
	searchService := programming.Language.Go(diagram.NodeLabel("Search Service\n(Fuzzy Artist Search)"))
	playlistService := programming.Language.Go(diagram.NodeLabel("Playlist Service"))
	spotifyAPI := generic.Blank.Blank(diagram.NodeLabel("Spotify Web API"))
	config := generic.Blank.Blank(diagram.NodeLabel("Configuration\n(env/godotenv)"))
	logging := generic.Blank.Blank(diagram.NodeLabel("Logging\n(logrus)"))

	// Create connections
	d.Connect(user, webUI, diagram.Forward())
	d.Connect(webUI, httpServer, diagram.Forward())
	d.Connect(httpServer, searchService, diagram.Forward())
	d.Connect(httpServer, playlistService, diagram.Forward())
	d.Connect(playlistService, spotifyAPI, diagram.Forward())
	d.Connect(httpServer, config, diagram.Forward())
	d.Connect(httpServer, logging, diagram.Forward())

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}

// generateComponentDiagram creates a detailed component diagram showing the
// relationships and dependencies between different packages in the go-listen project.
//
// The diagram illustrates:
//   - main.go as the entry point
//   - cmd/go-listen package handling CLI operations and server startup
//   - Internal services for search, playlist management, and Spotify integration
//   - Integration with configuration, version, and man packages
//   - Data flow between components
//
// The diagram is rendered in left-to-right (LR) direction and saved as
// "components.dot" in the current working directory. The function will
// terminate the program with log.Fatal if diagram creation or rendering fails.
func generateComponentDiagram() {
	d, err := diagram.New(diagram.Filename("components"), diagram.Label("Go-Listen Components"), diagram.Direction("LR"))
	if err != nil {
		log.Fatal(err)
	}

	// Main components
	main := programming.Language.Go(diagram.NodeLabel("main.go"))
	rootCmd := programming.Language.Go(diagram.NodeLabel("cmd/go-listen\nroot.go"))
	serveCmd := programming.Language.Go(diagram.NodeLabel("cmd/go-listen\nserve.go"))
	server := programming.Language.Go(diagram.NodeLabel("internal/server\nserver.go"))

	// Services
	searchService := programming.Language.Go(diagram.NodeLabel("internal/services/search\nfuzzy_artist_searcher.go"))
	playlistService := programming.Language.Go(diagram.NodeLabel("internal/services/playlist\nplaylist.go"))
	spotifyService := programming.Language.Go(diagram.NodeLabel("internal/services/spotify\nservice.go"))
	duplicateService := programming.Language.Go(diagram.NodeLabel("internal/services/duplicate\nduplicate.go"))

	// Middleware
	middleware := programming.Language.Go(diagram.NodeLabel("internal/middleware\nlogging, security, ratelimit"))

	// Packages
	config := programming.Language.Go(diagram.NodeLabel("pkg/config\nconfig.go"))
	version := programming.Language.Go(diagram.NodeLabel("pkg/version\nversion.go"))
	man := programming.Language.Go(diagram.NodeLabel("pkg/man\nman.go"))
	logging := programming.Language.Go(diagram.NodeLabel("pkg/logging\nlogger.go"))

	// Create connections showing the flow
	d.Connect(main, rootCmd, diagram.Forward())
	d.Connect(rootCmd, serveCmd, diagram.Forward())
	d.Connect(serveCmd, server, diagram.Forward())
	d.Connect(server, middleware, diagram.Forward())
	d.Connect(server, searchService, diagram.Forward())
	d.Connect(server, playlistService, diagram.Forward())
	d.Connect(server, spotifyService, diagram.Forward())
	d.Connect(server, duplicateService, diagram.Forward())
	d.Connect(rootCmd, config, diagram.Forward())
	d.Connect(rootCmd, version, diagram.Forward())
	d.Connect(rootCmd, man, diagram.Forward())
	d.Connect(server, logging, diagram.Forward())

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}
