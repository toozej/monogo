package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/gotts-it/internal/article"
	"github.com/toozej/monogo/apps/gotts-it/internal/tts"
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

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	scanner := bufio.NewScanner(os.Stdin)
	lineNum := 0
	processed := 0
	var processErrors []error

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := ctx.Err(); err != nil {
			processErrors = append(processErrors, err)
			break
		}
		processed++

		log.Infof("processing line %d: %s", lineNum, line)

		if err := processLine(ctx, line); err != nil {
			log.Errorf("line %d: %v", lineNum, err)
			processErrors = append(processErrors, fmt.Errorf("line %d: %w", lineNum, err))
			if ctxErr := ctx.Err(); ctxErr != nil {
				processErrors = append(processErrors, ctxErr)
				break
			}
			continue
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		processErrors = append(processErrors, fmt.Errorf("reading stdin: %w", err))
	}

	log.Infof("processed %d inputs", processed)
	return errors.Join(processErrors...)
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
	return defaultOutputPathInDir(art, format, conf.OutputDir)
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
