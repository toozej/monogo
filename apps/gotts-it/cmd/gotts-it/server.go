package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/gotts-it/internal/article"
	"github.com/toozej/gotts-it/internal/slug"
	"github.com/toozej/gotts-it/internal/tts"
)

func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run in server mode, processing URLs or files from stdin",
		Long: `Run gotts-it in server mode. Reads URLs or file paths from stdin,
one per line, and processes each one sequentially. Output files are written
to the directory specified by --output-dir (or the current directory).

Examples:
  echo "https://example.com/article1" | gotts-it server --output-dir ./out
  cat urls.txt | gotts-it server --output-dir ./out --tts-backend google --lang en`,
		Args: cobra.NoArgs,
		RunE: serverCmdRunE,
	}

	return cmd
}

func serverCmdRunE(cmd *cobra.Command, args []string) error {
	if conf.OutputDir == "" {
		conf.OutputDir = "."
	}
	if err := os.MkdirAll(conf.OutputDir, 0750); err != nil {
		return fmt.Errorf("create output directory %s: %w", conf.OutputDir, err)
	}

	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)
	lineNum := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lineNum++

		log.Infof("processing line %d: %s", lineNum, line)

		if err := processLine(ctx, line); err != nil {
			log.Errorf("line %d: %v", lineNum, err)
			continue
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("reading stdin: %w", err)
	}

	log.Infof("processed %d inputs", lineNum)
	return nil
}

func processLine(ctx context.Context, line string) error {
	var src article.Source
	if isURL(line) {
		src.URL = line
	} else {
		src.FilePath = line
	}

	art, err := article.FromSource(ctx, src, conf.FetchTimeout)
	if err != nil {
		return fmt.Errorf("extract article from %s: %w", line, err)
	}

	log.WithFields(log.Fields{
		"title": art.Title,
		"chars": len(art.Text),
	}).Infof("extracted article %q", art.Title)

	outputPath := serverOutputPath(art, conf.TTSFormat)

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

func serverOutputPath(art article.Article, format string) string {
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
	return filepath.Join(conf.OutputDir, base)
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
