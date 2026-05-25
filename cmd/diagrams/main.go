// Package main provides diagram generation utilities for the go-sort-out-gh-actions project.
//
// This application generates architectural and component diagrams for the go-sort-out-gh-actions
// project using the go-diagrams library. It creates visual representations of the
// project structure and component relationships to aid in documentation and understanding.
//
// The tool scans GitHub Actions workflow files, checks whether referenced actions are archived
// via the GitHub API, optionally sends notifications through multiple channels (Gotify, Slack,
// Telegram, Discord, Pushover, Pushbullet), and can create GitHub issues summarising the findings.
//
// The generated diagrams are saved as .dot files in the docs/diagrams/go-diagrams/
// directory and can be converted to various image formats using Graphviz.
//
// Usage:
//
//	go run cmd/diagrams/main.go
//
// This will generate:
//   - architecture.dot: High-level architecture showing the end-to-end detection flow
//   - components.dot:   Package-level component relationships and dependencies
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
// the end-to-end flow of the go-sort-out-gh-actions tool.
//
// The diagram illustrates:
//   - User invoking the CLI application
//   - Configuration loading from environment variables / .env file
//   - Workflow file discovery and YAML parsing (.github/workflows/**/*.yml|yaml)
//   - GitHub API calls to check archived status of each referenced action
//   - Optional notification dispatch to one or more providers
//     (Gotify, Slack, Telegram, Discord, Pushover, Pushbullet)
//   - Optional GitHub issue creation summarising archived findings
//
// The diagram is rendered in top-to-bottom (TB) direction and saved as
// "architecture.dot" in the current working directory. The function will
// terminate the program with log.Fatal if diagram creation or rendering fails.
func generateArchitectureDiagram() {
	d, err := diagram.New(
		diagram.Filename("architecture"),
		diagram.Label("go-sort-out-gh-actions Architecture"),
		diagram.Direction("TB"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Actors / external systems
	user := generic.Blank.Blank(diagram.NodeLabel("User"))
	githubAPI := generic.Blank.Blank(diagram.NodeLabel("GitHub API\n(api.github.com)"))
	notifProviders := generic.Blank.Blank(diagram.NodeLabel("Notification Providers\n(Gotify / Slack / Telegram\nDiscord / Pushover / Pushbullet)"))
	githubIssues := generic.Blank.Blank(diagram.NodeLabel("GitHub Issues\n(target repository)"))

	// Application components
	cli := programming.Language.Go(diagram.NodeLabel("CLI Application\n(cmd/go-sort-out-gh-actions)"))
	config := programming.Language.Go(diagram.NodeLabel("Configuration\n(pkg/config)\nenv / .env file"))
	workflowParser := programming.Language.Go(diagram.NodeLabel("Workflow Parser\n(internal/workflow)\nfinds & parses .github/workflows/**/*.yml"))
	ghClient := programming.Language.Go(diagram.NodeLabel("GitHub Client\n(internal/github)\nchecks archived status"))
	notifManager := programming.Language.Go(diagram.NodeLabel("Notification Manager\n(internal/notification)\nmulti-provider dispatch"))
	issueCreator := programming.Language.Go(diagram.NodeLabel("Issue Creator\n(internal/issue)\ncreates GitHub issues"))

	// User → CLI
	d.Connect(user, cli, diagram.Forward())

	// CLI reads config
	d.Connect(cli, config, diagram.Forward())

	// CLI drives workflow discovery and parsing
	d.Connect(cli, workflowParser, diagram.Forward())

	// CLI calls GitHub API client
	d.Connect(cli, ghClient, diagram.Forward())
	d.Connect(ghClient, githubAPI, diagram.Forward())

	// CLI optionally sends notifications
	d.Connect(cli, notifManager, diagram.Forward())
	d.Connect(notifManager, notifProviders, diagram.Forward())

	// CLI optionally creates a GitHub issue
	d.Connect(cli, issueCreator, diagram.Forward())
	d.Connect(issueCreator, githubIssues, diagram.Forward())

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}

// generateComponentDiagram creates a detailed component diagram showing the
// package-level relationships and dependencies within the go-sort-out-gh-actions project.
//
// The diagram illustrates:
//   - main.go as the binary entry point
//   - cmd/go-sort-out-gh-actions/root.go — cobra CLI root command, orchestration logic
//   - pkg/config/config.go         — environment-variable / .env configuration loading
//   - pkg/version/version.go       — version sub-command
//   - pkg/man/man.go               — man-page sub-command
//   - internal/workflow/workflow.go — workflow file discovery and YAML parsing
//   - internal/github/github.go    — GitHub REST API client (archived status checks)
//   - internal/notification/notification.go — multi-provider notification manager
//   - internal/issue/issue.go      — GitHub issue creation for archived findings
//   - internal/starter/starter.go  — application bootstrap / startup helper
//
// The diagram is rendered in left-to-right (LR) direction and saved as
// "components.dot" in the current working directory. The function will
// terminate the program with log.Fatal if diagram creation or rendering fails.
func generateComponentDiagram() {
	d, err := diagram.New(
		diagram.Filename("components"),
		diagram.Label("go-sort-out-gh-actions Components"),
		diagram.Direction("LR"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Entry point
	main := programming.Language.Go(diagram.NodeLabel("main.go"))

	// CLI layer
	rootCmd := programming.Language.Go(diagram.NodeLabel("cmd/go-sort-out-gh-actions\nroot.go"))

	// pkg layer
	config := programming.Language.Go(diagram.NodeLabel("pkg/config\nconfig.go"))
	version := programming.Language.Go(diagram.NodeLabel("pkg/version\nversion.go"))
	man := programming.Language.Go(diagram.NodeLabel("pkg/man\nman.go"))

	// internal layer
	workflowPkg := programming.Language.Go(diagram.NodeLabel("internal/workflow\nworkflow.go"))
	githubPkg := programming.Language.Go(diagram.NodeLabel("internal/github\ngithub.go"))
	notificationPkg := programming.Language.Go(diagram.NodeLabel("internal/notification\nnotification.go"))
	issuePkg := programming.Language.Go(diagram.NodeLabel("internal/issue\nissue.go"))

	// External dependency labels
	ghAPI := generic.Blank.Blank(diagram.NodeLabel("GitHub REST API"))
	notifServices := generic.Blank.Blank(diagram.NodeLabel("Notification Services\n(Gotify, Slack, Telegram\nDiscord, Pushover, Pushbullet)"))

	// main → root command
	d.Connect(main, rootCmd, diagram.Forward())

	// root command → pkg sub-commands
	d.Connect(rootCmd, version, diagram.Forward())
	d.Connect(rootCmd, man, diagram.Forward())

	// root command → config
	d.Connect(rootCmd, config, diagram.Forward())

	// root command → internal packages
	d.Connect(rootCmd, workflowPkg, diagram.Forward())
	d.Connect(rootCmd, githubPkg, diagram.Forward())
	d.Connect(rootCmd, notificationPkg, diagram.Forward())
	d.Connect(rootCmd, issuePkg, diagram.Forward())

	// notification manager uses config
	d.Connect(notificationPkg, config, diagram.Forward())

	// github client → GitHub API
	d.Connect(githubPkg, ghAPI, diagram.Forward())

	// issue creator → GitHub API (issues endpoint)
	d.Connect(issuePkg, ghAPI, diagram.Forward())

	// notification manager → external services
	d.Connect(notificationPkg, notifServices, diagram.Forward())

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}
