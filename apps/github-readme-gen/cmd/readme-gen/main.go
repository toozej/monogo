// Command readme-gen renders templates/README.md.tpl into README.md
// using data fetched from the GitHub API.
// Package readmegen implements the GitHub profile README generator command.
package readmegen

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/github-readme-gen/internal/readme"
	"github.com/toozej/monogo/pkg/avatar"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

type fetcher interface {
	Fetch(context.Context, string) (*readme.Data, error)
}

var rootCmd *cobra.Command

func newRootCommand(newClient func(context.Context, string) fetcher) *cobra.Command {
	var templatePath string
	var outputPath string
	var githubUser string
	var token string

	cmd := &cobra.Command{
		Use:           "readme-gen",
		Short:         "Generate a GitHub profile README",
		Long:          "Generate a GitHub profile README from recent public GitHub activity.",
		Args:          cobra.ExactArgs(0),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("token") {
				token = os.Getenv("GITHUB_TOKEN")
			}
			return run(cmd.Context(), cmd.OutOrStdout(), templatePath, outputPath, githubUser, token, newClient)
		},
	}
	cmd.Flags().StringVar(&templatePath, "template", "templates/README.md.tpl", "Path to README template file")
	cmd.Flags().StringVar(&outputPath, "output", "README.md", "Path to write generated README")
	cmd.Flags().StringVar(&githubUser, "user", "toozej", "GitHub username to fetch data for")
	cmd.Flags().StringVar(&token, "token", "", "GitHub token (defaults to GITHUB_TOKEN)")
	cmd.AddCommand(
		avatar.NewCommand("github-readme-gen"),
		man.NewManCmd(),
		version.Command(),
	)
	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultHelpCmd()
	return cmd
}

func run(ctx context.Context, stdout io.Writer, templatePath, outputPath, githubUser, token string, newClient func(context.Context, string) fetcher) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	data, err := newClient(ctx, token).Fetch(ctx, githubUser)
	if err != nil {
		return fmt.Errorf("fetch GitHub data: %w", err)
	}

	if err := readme.Render(templatePath, outputPath, data); err != nil {
		return fmt.Errorf("render README: %w", err)
	}

	_, err = fmt.Fprintf(stdout, "Wrote %s from %s\n", outputPath, templatePath)
	return err
}

// Execute runs the README generator command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd = newRootCommand(func(ctx context.Context, token string) fetcher {
		return readme.NewClient(ctx, token)
	})
}
