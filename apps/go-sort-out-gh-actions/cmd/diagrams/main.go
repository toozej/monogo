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
	outputDir := "docs/diagrams"
	if err := run(outputDir); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Diagram .dot files generated successfully in ./docs/diagrams/go-diagrams/")
}

// run orchestrates diagram generation and returns any errors encountered.
func run(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := generateArchitectureDiagram(outputDir); err != nil {
		return err
	}
	if err := generateComponentDiagram(outputDir); err != nil {
		return err
	}

	return nil
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
// "architecture.dot" in the current working directory.
func generateArchitectureDiagram(outputDir string) error {
	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	if err := os.Chdir(outputDir); err != nil {
		return fmt.Errorf("failed to change to output directory: %w", err)
	}
	d, err := diagram.New(
		diagram.Filename("architecture"),
		diagram.Label("go-sort-out-gh-actions Architecture"),
		diagram.Direction("TB"),
	)
	if err != nil {
		return fmt.Errorf("failed to create architecture diagram: %w", err)
	}

	user := generic.Blank.Blank(diagram.NodeLabel("User"))
	githubAPI := generic.Blank.Blank(diagram.NodeLabel("GitHub API\n(api.github.com)"))
	notifProviders := generic.Blank.Blank(diagram.NodeLabel("Notification Providers\n(Gotify / Slack / Telegram\nDiscord / Pushover / Pushbullet)"))
	githubIssues := generic.Blank.Blank(diagram.NodeLabel("GitHub Issues\n(target repository)"))

	cli := programming.Language.Go(diagram.NodeLabel("CLI Application\n(cmd/go-sort-out-gh-actions)"))
	config := programming.Language.Go(diagram.NodeLabel("Configuration\n(pkg/config)\nenv / .env file"))
	workflowParser := programming.Language.Go(diagram.NodeLabel("Workflow Parser\n(internal/workflow)\nfinds & parses .github/workflows/**/*.yml"))
	ghClient := programming.Language.Go(diagram.NodeLabel("GitHub Client\n(internal/github)\nchecks archived status"))
	notifManager := programming.Language.Go(diagram.NodeLabel("Notification Manager\n(internal/notification)\nmulti-provider dispatch"))
	issueCreator := programming.Language.Go(diagram.NodeLabel("Issue Creator\n(internal/issue)\ncreates GitHub issues"))

	d.Connect(user, cli, diagram.Forward())
	d.Connect(cli, config, diagram.Forward())
	d.Connect(cli, workflowParser, diagram.Forward())
	d.Connect(cli, ghClient, diagram.Forward())
	d.Connect(ghClient, githubAPI, diagram.Forward())
	d.Connect(cli, notifManager, diagram.Forward())
	d.Connect(notifManager, notifProviders, diagram.Forward())
	d.Connect(cli, issueCreator, diagram.Forward())
	d.Connect(issueCreator, githubIssues, diagram.Forward())

	if err := d.Render(); err != nil {
		return fmt.Errorf("failed to render architecture diagram: %w", err)
	}

	if err := os.Chdir(origDir); err != nil {
		log.Printf("Warning: failed to change back to original directory: %v", err)
	}
	return nil
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
//
// The diagram is rendered in left-to-right (LR) direction and saved as
// "components.dot" in the current working directory.
func generateComponentDiagram(outputDir string) error {
	origDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	if err := os.Chdir(outputDir); err != nil {
		return fmt.Errorf("failed to change to output directory: %w", err)
	}
	d, err := diagram.New(
		diagram.Filename("components"),
		diagram.Label("go-sort-out-gh-actions Components"),
		diagram.Direction("LR"),
	)
	if err != nil {
		return fmt.Errorf("failed to create component diagram: %v", err)
	}

	main := programming.Language.Go(diagram.NodeLabel("main.go"))
	rootCmd := programming.Language.Go(diagram.NodeLabel("cmd/go-sort-out-gh-actions\nroot.go"))
	config := programming.Language.Go(diagram.NodeLabel("pkg/config\nconfig.go"))
	version := programming.Language.Go(diagram.NodeLabel("pkg/version\nversion.go"))
	man := programming.Language.Go(diagram.NodeLabel("pkg/man\nman.go"))
	workflowPkg := programming.Language.Go(diagram.NodeLabel("internal/workflow\nworkflow.go"))
	githubPkg := programming.Language.Go(diagram.NodeLabel("internal/github\ngithub.go"))
	notificationPkg := programming.Language.Go(diagram.NodeLabel("internal/notification\nnotification.go"))
	issuePkg := programming.Language.Go(diagram.NodeLabel("internal/issue\nissue.go"))

	ghAPI := generic.Blank.Blank(diagram.NodeLabel("GitHub REST API"))
	notifServices := generic.Blank.Blank(diagram.NodeLabel("Notification Services\n(Gotify, Slack, Telegram\nDiscord, Pushover, Pushbullet)"))

	d.Connect(main, rootCmd, diagram.Forward())
	d.Connect(rootCmd, version, diagram.Forward())
	d.Connect(rootCmd, man, diagram.Forward())
	d.Connect(rootCmd, config, diagram.Forward())
	d.Connect(rootCmd, workflowPkg, diagram.Forward())
	d.Connect(rootCmd, githubPkg, diagram.Forward())
	d.Connect(rootCmd, notificationPkg, diagram.Forward())
	d.Connect(rootCmd, issuePkg, diagram.Forward())
	d.Connect(notificationPkg, config, diagram.Forward())
	d.Connect(githubPkg, ghAPI, diagram.Forward())
	d.Connect(issuePkg, ghAPI, diagram.Forward())
	d.Connect(notificationPkg, notifServices, diagram.Forward())

	if err := d.Render(); err != nil {
		return fmt.Errorf("failed to render component diagram: %w", err)
	}

	if err := os.Chdir(origDir); err != nil {
		log.Printf("Warning: failed to change back to original directory: %v", err)
	}
	return nil
}
