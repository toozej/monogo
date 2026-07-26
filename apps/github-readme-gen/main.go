// Command github-readme-gen generates GitHub profile README files.
package main

import (
	"fmt"
	"os"

	readmegen "github.com/toozej/monogo/apps/github-readme-gen/cmd/readme-gen"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "completion":
			// The generator has no interactive flags to complete. This command is
			// present so the monorepo release hooks can generate an empty completion
			// file consistently with the other CLI apps.
			return
		case "man":
			fmt.Print(".TH README-GEN 1\n.SH NAME\nreadme-gen \\ - generate GitHub profile README files\n")
			return
		}
	}
	readmegen.Execute()
}
