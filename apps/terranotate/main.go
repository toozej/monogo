// Package main provides the entry point for the terranotate application.
//
// This application serves as a template for Go projects, demonstrating
// best practices for CLI applications using cobra, logrus, and environment
// configuration management.
package main

import cmd "github.com/toozej/terranotate/cmd/terranotate"

// main is the entry point of the terranotate application.
// It delegates execution to the cmd package which handles all
// command-line interface functionality.
func main() {
	cmd.Execute()
}
