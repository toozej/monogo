package main

import (
	"os"

	cmd "github.com/toozej/go-find-liquor/cmd/go-find-liquor"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
