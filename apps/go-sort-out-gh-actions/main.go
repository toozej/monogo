// Package main provides the entry point for the go-sort-out-gh-actions application.
//
// This application serves as a template for Go projects, demonstrating
// best practices for CLI applications using cobra, logrus, and environment
// configuration management.
package main

import cmd "github.com/toozej/monogo/apps/go-sort-out-gh-actions/cmd/go-sort-out-gh-actions"

// main is the entry point of the go-sort-out-gh-actions application.
// It delegates execution to the cmd package which handles all
// command-line interface functionality.
func main() {
	cmd.Execute()
}
