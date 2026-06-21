// Package cmd provides command-line interface functionality for the gotts-it application.
//
// This package implements the root command and manages the command-line interface
// using the cobra library. It handles configuration, logging setup, and command
// execution for the gotts-it application.
//
// The package integrates with several components:
// - Configuration management through pkg/config
// - Article extraction through internal/article
// - TTS synthesis through internal/tts
// - Manual pages through pkg/man
// - Version information through pkg/version
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/gotts-it/internal/article"
	"github.com/toozej/monogo/apps/gotts-it/internal/slug"
	"github.com/toozej/monogo/apps/gotts-it/internal/tts"
	"github.com/toozej/monogo/pkg/gotts-it/config"
	"github.com/toozej/monogo/pkg/gotts-it/man"
	"github.com/toozej/monogo/pkg/gotts-it/version"
)

var (
	conf  config.Config
	debug bool
)

var rootCmd = &cobra.Command{
	Use:   "gotts-it",
	Short: "Extract article text from a URL or file and synthesize speech via an OpenAI-compatible TTS server",
	Long: `gotts-it extracts readable article text from a URL or local HTML
file, then synthesizes speech via an OpenAI-compatible TTS server (e.g. Speaches)
or Google Translate TTS.

Exactly one of --url or --file is required.

Examples:
gotts-it --url https://en.wikipedia.org/wiki/Readability
gotts-it --file article.html -o article.mp3
gotts-it --url https://example.com/post --format wav --speed 1.5
gotts-it --url https://example.com/post --tts-backend google --lang fr`,
	Args:             cobra.ExactArgs(0),
	PersistentPreRun: rootCmdPreRun,
	RunE:             rootCmdRunE,
}

func rootCmdRunE(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	src := article.Source{URL: conf.URL, FilePath: conf.File}
	art, err := article.FromSource(ctx, src, conf.FetchTimeout)
	if err != nil {
		return fmt.Errorf("extract article: %w", err)
	}

	log.WithFields(log.Fields{
		"title": art.Title,
		"chars": len(art.Text),
	}).Infof("extracted article %q", art.Title)

	outputPath := conf.Output
	if outputPath == "" {
		outputPath = defaultOutputPath(art, conf.TTSFormat)
	}

	if conf.Output == "" && conf.OutputDir != "" {
		outputPath = filepath.Join(conf.OutputDir, filepath.Base(outputPath))
		if err := os.MkdirAll(conf.OutputDir, 0750); err != nil {
			return fmt.Errorf("create output directory %s: %w", conf.OutputDir, err)
		}
	}

	switch conf.TTSBackend {
	case "openai":
		opts := tts.Options{
			BaseURL:      conf.OpenAIBaseURL,
			APIKey:       conf.OpenAIToken,
			Model:        conf.TTSModel,
			Voice:        conf.TTSVoice,
			Format:       conf.TTSFormat,
			Speed:        conf.TTSSpeed,
			Instructions: conf.TTSInstructions,
			Timeout:      conf.TTSTimeout,
		}
		if err := tts.Synthesize(ctx, art.Text, outputPath, opts); err != nil {
			return fmt.Errorf("synthesize speech: %w", err)
		}
	case "google":
		gopts := tts.GoogleTranslateOptions{
			Lang:    conf.GoogleTranslateLang,
			Timeout: conf.TTSTimeout,
		}
		if err := tts.SynthesizeGoogleTranslate(ctx, art.Text, outputPath, gopts); err != nil {
			return fmt.Errorf("synthesize speech: %w", err)
		}
	default:
		return fmt.Errorf("unknown TTS backend %q: use \"openai\" or \"google\"", conf.TTSBackend)
	}

	log.Infof("wrote audio to %s", outputPath)
	return nil
}

func defaultOutputPath(art article.Article, format string) string {
	var s string
	switch {
	case art.Title != "":
		s = slug.FromTitle(art.Title)
	case art.URL != "":
		if _, err := os.Stat(art.URL); err == nil {
			s = slug.FromFilePath(art.URL)
		} else {
			s = slug.FromURL(art.URL)
		}
	default:
		s = "output"
	}

	base := s + "." + format
	candidate := base
	counter := 2
	for {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d.%s", s, counter, format)
		counter++
	}
}

func rootCmdPreRun(cmd *cobra.Command, args []string) {
	if debug {
		log.SetLevel(log.DebugLevel)
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func init() {
	conf = config.GetEnvVars()

	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")

	rootCmd.Flags().StringVarP(&conf.URL, "url", "U", conf.URL, "Article URL to fetch and convert to speech")
	rootCmd.Flags().StringVarP(&conf.File, "file", "f", conf.File, "Local HTML or text file to convert to speech")
	rootCmd.Flags().StringVarP(&conf.Output, "output", "o", conf.Output, "Output audio file path (default: derived from article title)")
	rootCmd.PersistentFlags().StringVar(&conf.OutputDir, "output-dir", conf.OutputDir, "Output directory for audio files (default: current directory)")
	rootCmd.PersistentFlags().StringVar(&conf.TTSFormat, "format", conf.TTSFormat, "Output audio format (mp3, wav, flac, pcm)")
	rootCmd.PersistentFlags().StringVar(&conf.TTSVoice, "voice", conf.TTSVoice, "TTS voice ID")
	rootCmd.PersistentFlags().StringVar(&conf.TTSModel, "model", conf.TTSModel, "TTS model ID")
	rootCmd.PersistentFlags().Float64Var(&conf.TTSSpeed, "speed", conf.TTSSpeed, "Speech speed (0.25 to 4.0)")
	rootCmd.PersistentFlags().StringVar(&conf.TTSInstructions, "instructions", conf.TTSInstructions, "Model instructions (ignored by tts-1/tts-1-hd)")
	rootCmd.PersistentFlags().DurationVar(&conf.FetchTimeout, "fetch-timeout", conf.FetchTimeout, "Timeout for fetching article URL")
	rootCmd.PersistentFlags().DurationVar(&conf.TTSTimeout, "tts-timeout", conf.TTSTimeout, "Timeout per TTS chunk request")
	rootCmd.PersistentFlags().StringVar(&conf.TTSBackend, "tts-backend", conf.TTSBackend, "TTS backend: openai or google")
	rootCmd.PersistentFlags().StringVar(&conf.GoogleTranslateLang, "lang", conf.GoogleTranslateLang, "Language for Google Translate TTS (e.g. en, fr, de)")

	rootCmd.MarkFlagsOneRequired("url", "file")
	rootCmd.MarkFlagsMutuallyExclusive("url", "file")

	rootCmd.AddCommand(
		man.NewManCmd(),
		version.Command(),
		newAvatarCmd(),
		newServerCmd(),
	)
}
