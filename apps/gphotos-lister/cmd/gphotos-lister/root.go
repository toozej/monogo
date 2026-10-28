// Package cmd defines the gphotos-lister CLI and web commands.
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/gphotos-lister/internal/config"
	"github.com/toozej/monogo/apps/gphotos-lister/internal/lister"
	"github.com/toozej/monogo/apps/gphotos-lister/internal/photos"
	"github.com/toozej/monogo/apps/gphotos-lister/internal/web"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

type backend interface {
	lister.AlbumReader
}

type dependencies struct {
	load   func() (config.Config, error)
	reader func(config.Config, *slog.Logger) backend
	serve  func(context.Context, config.Config, lister.AlbumReader) error
}

func defaults() dependencies {
	return dependencies{
		load: config.Load,
		reader: func(c config.Config, logger *slog.Logger) backend {
			return &photos.Client{Timeout: c.Timeout, Logger: logger}
		},
		serve: func(ctx context.Context, c config.Config, reader lister.AlbumReader) error {
			return (&web.Server{Config: c, Reader: reader}).Serve(ctx)
		},
	}
}

// NewCommand creates a command tree with independent flag state.
func NewCommand() *cobra.Command { return newRootCommand(defaults()) }

func newRootCommand(deps dependencies) *cobra.Command {
	var flags config.Config
	root := &cobra.Command{Use: "gphotos-lister [album URL]", Short: "List original filenames of photos with album hearts", Args: cobra.MaximumNArgs(1), SilenceUsage: true, SilenceErrors: true}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		conf, err := loadConfig(cmd, flags, deps.load)
		if err != nil {
			return err
		}
		if len(args) > 0 {
			if cmd.Flags().Changed("album") {
				return fmt.Errorf("use either the album argument or --album")
			}
			conf.Album = args[0]
		}
		if err := conf.Options().Validate(); err != nil {
			return err
		}
		if err := photos.ValidateAlbumURL(conf.Album); err != nil {
			return err
		}
		if strings.ToLower(filepath.Ext(conf.Output)) != ".txt" {
			return fmt.Errorf("output path must end with .txt")
		}
		logger := commandLogger(cmd, conf)
		logger.DebugContext(cmd.Context(), "Start filename collection", "timeout", conf.Timeout, "copy", conf.Copy)
		result, runErr := lister.Run(cmd.Context(), deps.reader(conf, logger), conf.Options())
		if runErr != nil && !result.ListReady {
			logger.DebugContext(cmd.Context(), "Filename collection failed")
			return runErr
		}
		if err := lister.WriteText(conf.Output, result.Text); err != nil {
			return fmt.Errorf("write filename list: %w", err)
		}
		cmd.Printf("Wrote %d filenames to %s\n", len(result.Filenames), conf.Output)
		logger.DebugContext(cmd.Context(), "Wrote filename list", "filenames", len(result.Filenames), "bytes", len(result.Text))
		if len(result.Filenames) == 0 {
			cmd.PrintErrln("No photo filenames matched. Use --debug to inspect album pages and heart matches.")
		}
		if conf.Copy {
			cmd.Printf("Copied: %d; missing: %d; ambiguous: %d; skipped: %d\n", len(result.Copied), len(result.Missing), len(result.Ambiguous), len(result.Skipped))
			for _, name := range result.Missing {
				cmd.PrintErrf("Missing: %s\n", name)
			}
			for _, name := range result.Ambiguous {
				cmd.PrintErrf("Ambiguous: %s\n", name)
			}
		}
		return runErr
	}
	f := root.PersistentFlags()
	f.StringVar(&flags.Album, "album", "", "Google Photos album URL (GPHOTOS_ALBUM)")
	f.StringVarP(&flags.Output, "output", "o", "liked-photos.txt", "Output .txt path (GPHOTOS_OUTPUT)")
	f.StringVar(&flags.Prefix, "prefix", "", "Text before each filename (GPHOTOS_PREFIX)")
	f.StringVar(&flags.Suffix, "suffix", "", "Text after each filename (GPHOTOS_SUFFIX)")
	f.BoolVar(&flags.Copy, "copy", false, "Copy matching local files (GPHOTOS_COPY)")
	f.StringVar(&flags.Source, "source", "", "Source directory to search recursively (GPHOTOS_SOURCE)")
	f.StringVar(&flags.Destination, "destination", "", "Destination directory (GPHOTOS_DESTINATION)")
	f.BoolVar(&flags.NoOverwrite, "no-overwrite", false, "Skip existing destination files (GPHOTOS_NO_OVERWRITE)")
	f.BoolVar(&flags.Debug, "debug", false, "Log request progress and heart counts to stderr (GPHOTOS_DEBUG)")
	f.DurationVar(&flags.Timeout, "timeout", 0, "Maximum album request duration (GPHOTOS_TIMEOUT, default 15m)")
	f.StringVar(&flags.WebHost, "host", "127.0.0.1", "Web bind address (GPHOTOS_WEB_HOST)")
	f.IntVar(&flags.WebPort, "port", 8080, "Web port (GPHOTOS_WEB_PORT)")
	f.StringVar(&flags.WebUsername, "web-username", "", "Web authentication username (GPHOTOS_WEB_USERNAME)")
	f.StringVar(&flags.WebPassword, "web-password", "", "Web authentication password (GPHOTOS_WEB_PASSWORD)")
	root.AddCommand(&cobra.Command{Use: "serve", Short: "Start the web UI", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		conf, err := loadConfig(cmd, flags, deps.load)
		if err != nil {
			return err
		}
		if err := web.ValidateBind(conf.WebHost, conf.WebUsername, conf.WebPassword); err != nil {
			return err
		}
		logger := commandLogger(cmd, conf)
		logger.DebugContext(cmd.Context(), "Start web server", "host", conf.WebHost, "port", conf.WebPort)
		return deps.serve(cmd.Context(), conf, deps.reader(conf, logger))
	}})
	root.AddCommand(version.Command(), man.NewManCmd())
	return root
}

func loadConfig(cmd *cobra.Command, flags config.Config, load func() (config.Config, error)) (config.Config, error) {
	conf, err := load()
	if err != nil {
		return conf, fmt.Errorf("load configuration: %w", err)
	}
	strings := map[string]struct {
		dst   *string
		value string
	}{
		"album": {&conf.Album, flags.Album}, "output": {&conf.Output, flags.Output}, "prefix": {&conf.Prefix, flags.Prefix}, "suffix": {&conf.Suffix, flags.Suffix},
		"source": {&conf.Source, flags.Source}, "destination": {&conf.Destination, flags.Destination},
		"host": {&conf.WebHost, flags.WebHost}, "web-username": {&conf.WebUsername, flags.WebUsername}, "web-password": {&conf.WebPassword, flags.WebPassword},
	}
	for name, field := range strings {
		if cmd.Flags().Changed(name) {
			*field.dst = field.value
		}
	}
	for name, field := range map[string]struct {
		dst   *bool
		value bool
	}{"copy": {&conf.Copy, flags.Copy}, "no-overwrite": {&conf.NoOverwrite, flags.NoOverwrite}, "debug": {&conf.Debug, flags.Debug}} {
		if cmd.Flags().Changed(name) {
			*field.dst = field.value
		}
	}
	if cmd.Flags().Changed("timeout") {
		conf.Timeout = flags.Timeout
	}
	if cmd.Flags().Changed("port") {
		conf.WebPort = flags.WebPort
	}
	if conf.Timeout <= 0 {
		return conf, fmt.Errorf("timeout must be greater than zero")
	}
	return conf, nil
}

func commandLogger(cmd *cobra.Command, conf config.Config) *slog.Logger {
	level := slog.LevelInfo
	if conf.Debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: level}))
}

// Execute handles interrupt signals and returns CLI failures to the shell.
func Execute() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := NewCommand().ExecuteContext(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
