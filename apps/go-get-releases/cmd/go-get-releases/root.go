// Package cmd provides the go-get-releases command line.
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/go-get-releases/internal/releases"
	"github.com/toozej/monogo/pkg/avatar"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

func newRootCommand() *cobra.Command {
	return newCommand(downloadReleases)
}

func newCommand(run func(context.Context, releases.Config, io.Writer, io.Writer) error) *cobra.Command {
	defaults := releases.Defaults()
	var configPath, directory string
	var repositories, targets []string
	var keep, retries int
	var requireAll bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:           "go-get-releases [owner/repo ...]",
		Short:         "Store the latest stable GitHub release packages",
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := defaults
			if configPath != "" {
				var err error
				c, err = releases.Load(configPath)
				if err != nil {
					return err
				}
			}
			for _, name := range append(repositories, args...) {
				c.Repositories = append(c.Repositories, releases.Repository{Name: name})
			}
			if cmd.Flags().Changed("directory") {
				c.Directory = directory
			}
			if cmd.Flags().Changed("keep") {
				c.Keep = keep
			}
			if cmd.Flags().Changed("retries") {
				c.Retries = retries
			}
			if cmd.Flags().Changed("require-all") {
				c.RequireAll = requireAll
			}
			if cmd.Flags().Changed("target") {
				c.Targets = nil
				for _, value := range targets {
					t, err := releases.ParseTarget(value)
					if err != nil {
						return err
					}
					c.Targets = append(c.Targets, t)
				}
			}
			if timeout <= 0 {
				return fmt.Errorf("timeout must be greater than zero")
			}
			if err := c.Validate(); err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			return run(ctx, c, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "Read a YAML configuration file")
	cmd.Flags().StringSliceVar(&repositories, "repo", nil, "Add GitHub repositories (owner/repo or HTTPS URL)")
	cmd.Flags().StringVar(&directory, "directory", defaults.Directory, "Store releases in this directory")
	cmd.Flags().IntVar(&keep, "keep", defaults.Keep, "Keep this many latest stable releases per repository")
	cmd.Flags().IntVar(&retries, "retries", defaults.Retries, "Retry each failed download at least once (1 to 10)")
	cmd.Flags().StringSliceVar(&targets, "target", nil, "Select OS/ARCH:FORMAT+FORMAT (repeat this option)")
	cmd.Flags().BoolVar(&requireAll, "require-all", false, "Require an asset for every target before cleanup")
	cmd.Flags().DurationVar(&timeout, "timeout", 6*time.Hour, "Stop the run after this duration")
	cmd.AddCommand(avatar.NewCommand("go-get-releases"), man.NewManCmd(), version.Command())
	return cmd
}

func downloadReleases(ctx context.Context, c releases.Config, out, stderr io.Writer) error {
	token := releases.DiscoverToken(ctx)
	if token == "" {
		_, _ = fmt.Fprintln(stderr, "No GitHub token is available. Use the unauthenticated API rate limit.")
	}
	source, err := releases.NewGitHub(token, c.Retries)
	if err != nil {
		return err
	}
	runner := releases.Runner{Source: source, Log: out}
	return runner.Run(ctx, c)
}

// Execute stops downloads when the process receives an interrupt.
func Execute() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	cmd := newRootCommand()
	err := cmd.ExecuteContext(ctx)
	cancel()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
