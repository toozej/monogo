package cmd

import (
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
	original := conf
	defer func() { conf = original }()
	conf = config.Config{
		URL: "https://example.com", AnswerSelector: ".answer", OutputFile: "cards.csv",
		HTTPTimeout: time.Second, MaxResponseBytes: 1024,
	}

	if err := rootCmdPreRunE(&cobra.Command{Use: "url2anki"}, nil); err == nil {
		t.Fatal("expected missing question selector to fail")
	}
}
