package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/url2anki/internal/config"
)

func TestRootCommandUsesRunE(t *testing.T) {
	if rootCmd.RunE == nil {
		t.Fatal("root command must propagate operational errors through RunE")
	}
}

func TestRootPreRunAcceptsEnvironmentConfiguration(t *testing.T) {
	original := conf
	defer func() { conf = original }()
	conf = config.Config{
		URL: "https://example.com", QuestionSelector: ".question", AnswerSelector: ".answer",
		OutputFile: "cards.csv", HTTPTimeout: time.Second, MaxResponseBytes: 1024,
	}

	if err := rootCmdPreRunE(&cobra.Command{Use: "url2anki"}, nil); err != nil {
		t.Fatalf("effective environment configuration was rejected: %v", err)
	}
}

func TestRootPreRunRejectsMissingEffectiveValue(t *testing.T) {
	valid := config.Config{
		URL: "https://example.com", AnswerSelector: ".answer", OutputFile: "cards.csv",
		QuestionSelector: ".question", HTTPTimeout: time.Second, MaxResponseBytes: 1024,
	}
	tests := []struct {
		name   string
		mutate func(*config.Config)
		want   string
	}{
		{name: "blank URL", mutate: func(c *config.Config) { c.URL = " \t" }, want: "url is required"},
		{name: "blank question selector", mutate: func(c *config.Config) { c.QuestionSelector = " \t" }, want: "question selector is required"},
		{name: "blank answer selector", mutate: func(c *config.Config) { c.AnswerSelector = " \t" }, want: "answer selector is required"},
		{name: "blank output file", mutate: func(c *config.Config) { c.OutputFile = " \t" }, want: "output file is required"},
		{name: "zero timeout", mutate: func(c *config.Config) { c.HTTPTimeout = 0 }, want: "http timeout"},
		{name: "zero response limit", mutate: func(c *config.Config) { c.MaxResponseBytes = 0 }, want: "maximum response size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := conf
			t.Cleanup(func() { conf = original })
			conf = valid
			tt.mutate(&conf)
			err := rootCmdPreRunE(&cobra.Command{Use: "url2anki"}, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("rootCmdPreRunE() error = %v, want %q", err, tt.want)
			}
		})
	}
}
