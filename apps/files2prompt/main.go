// Package main provides the entry point for the files2prompt CLI application
// files2prompt is a command-line tool that helps prepare files for AI prompts
// by crawling directories and outputting file contents with flexible filtering and formatting options.
package main

import cmd "github.com/toozej/files2prompt/cmd/files2prompt"

func main() {
	cmd.Execute()
}
