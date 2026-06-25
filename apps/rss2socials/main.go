// Package main contains the entry point for the rss2socials application.
// It imports and executes the command-line interface from the cmd package.
package main

import cmd "github.com/toozej/rss2socials/cmd/rss2socials"

func main() {
	cmd.Execute()
}
