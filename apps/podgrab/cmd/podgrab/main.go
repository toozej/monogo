// Package main provides the CLI entry point for the podgrab application.
//
// This package wraps the web application into a minimal cobra command structure
// to support CLI features like completions and man page generation.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/toozej/podgrab/pkg/man"
	"github.com/toozej/podgrab/pkg/version"
)

var rootCmd = &cobra.Command{
	Use:   "podgrab",
	Short: "Self-hosted podcast manager",
	Long:  `Podgrab is a self-hosted podcast manager that automatically downloads podcast episodes.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Run the web application
		fmt.Println("Starting podgrab web server...")
		// The actual web server logic is in main.go; this is a minimal wrapper
		// for CLI tooling support (completions, man pages).
		fmt.Println("Use 'go run main.go' to start the web server directly.")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func main() {
	rootCmd.AddCommand(version.Command())
	rootCmd.AddCommand(man.NewManCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
