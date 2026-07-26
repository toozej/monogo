// Command readme-gen renders templates/README.md.tpl into README.md
// using data fetched from the GitHub API.
// Package readmegen implements the GitHub profile README generator command.
package readmegen

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/toozej/monogo/apps/github-readme-gen/internal/readme"
)

type fetcher interface {
	Fetch(context.Context, string) (*readme.Data, error)
}

// Execute runs the README generator using the process command-line arguments.
func Execute() {
	if err := run(os.Args[1:], os.Stdout, func(ctx context.Context, token string) fetcher {
		return readme.NewClient(ctx, token)
	}); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, stdout io.Writer, newClient func(context.Context, string) fetcher) error {
	flags := flag.NewFlagSet("readme-gen", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	templatePath := flags.String("template", "templates/README.md.tpl", "path to README template file")
	outputPath := flags.String("output", "README.md", "path to write generated README")
	githubUser := flags.String("user", "toozej", "GitHub username to fetch data for")
	token := flags.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub token (defaults to GITHUB_TOKEN env var)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	data, err := newClient(ctx, *token).Fetch(ctx, *githubUser)
	if err != nil {
		return fmt.Errorf("fetch GitHub data: %w", err)
	}

	if err := readme.Render(*templatePath, *outputPath, data); err != nil {
		return fmt.Errorf("render README: %w", err)
	}

	_, err = fmt.Fprintf(stdout, "Wrote %s from %s\n", *outputPath, *templatePath)
	return err
}
