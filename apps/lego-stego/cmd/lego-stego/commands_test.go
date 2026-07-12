package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandsRejectPositionalArguments(t *testing.T) {
	commands := []*cobra.Command{embedCmd, extractCmd, hideCmd, revealCmd, infoCmd}
	for _, command := range commands {
		t.Run(command.Name(), func(t *testing.T) {
			if err := command.Args(command, []string{"unexpected"}); err == nil {
				t.Fatal("unexpected positional argument was accepted")
			}
		})
	}
}

func TestValidateEmbedURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "https", value: "https://example.com/path?query=value#fragment"},
		{name: "http", value: "http://example.com"},
		{name: "missing scheme", value: "example.com", wantErr: true},
		{name: "missing host", value: "https:///path", wantErr: true},
		{name: "unsupported scheme", value: "file:///tmp/data", wantErr: true},
		{name: "invalid port", value: "https://example.com:not-a-port", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateEmbedURL(tt.value); (err != nil) != tt.wantErr {
				t.Fatalf("validateEmbedURL(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestExtractOutputFlagIsOptional(t *testing.T) {
	flag := extractCmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("output flag is missing")
	}
	if _, required := flag.Annotations[cobra.BashCompOneRequiredFlag]; required {
		t.Fatal("extract output flag must remain optional")
	}
}
