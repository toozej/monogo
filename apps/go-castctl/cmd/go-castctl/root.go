// Package cmd implements the go-castctl command-line interface.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/go-castctl/internal/castor"
	"github.com/toozej/monogo/apps/go-castctl/internal/config"
	"github.com/toozej/monogo/apps/go-castctl/internal/server"
	"github.com/toozej/monogo/pkg/avatar"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

// NewRootCommand constructs a new command tree.
func NewRootCommand() *cobra.Command {
	run := func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load configuration: %w", err)
		}
		if err := cfg.Validate(); err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		client := castor.New(cfg.Castor.Binary, cfg.Castor.ConfigPath, cfg.Castor.Timeout)
		srv := server.New(cfg.Server, client)
		return srv.Run(ctx)
	}

	root := &cobra.Command{
		Use:          "go-castctl",
		Short:        "Cast web video to a TV or watch it locally",
		Args:         cobra.NoArgs,
		RunE:         run,
		SilenceUsage: true,
	}
	root.SetContext(context.Background())
	root.AddCommand(
		&cobra.Command{Use: "serve", Short: "Start the web server", Args: cobra.NoArgs, RunE: run},
		avatar.NewCommand("go-castctl"),
		man.NewManCmd(),
		version.Command(),
	)
	return root
}

// Execute runs the command and exits nonzero on failure.
func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
