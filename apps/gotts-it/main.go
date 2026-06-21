// Package main provides the entry point for the gotts-it application.
//
// This application serves as a template for Go projects, demonstrating
// best practices for CLI applications using cobra, logrus, and environment
// configuration management.
package main

import cmd "github.com/toozej/monogo/apps/gotts-it/cmd/gotts-it"

// main is the entry point of the gotts-it application.
// It delegates execution to the cmd package which handles all
// command-line interface functionality.
func main() {
	cmd.Execute()
}
