package cmd

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/go-sort-out-gh-actions/pkg/config"
)

func TestCommandStructure(t *testing.T) {
	cmd := rootCmd

	subCommands := cmd.Commands()
	commandNames := make(map[string]bool)
	for _, subCmd := range subCommands {
		commandNames[subCmd.Name()] = true
	}

	expectedCommands := []string{"archived", "eol", "outdated", "check", "version", "man"}
	for _, name := range expectedCommands {
		if !commandNames[name] {
			t.Errorf("Expected subcommand %q not found", name)
		}
	}
}

func TestGlobalFlags(t *testing.T) {
	expectedPersistentFlags := []string{"debug", "verbose", "token", "notify", "create-issue", "workflow", "workflows-dir", "repos-dir", "output-format"}
	for _, flagName := range expectedPersistentFlags {
		if rootCmd.PersistentFlags().Lookup(flagName) == nil {
			t.Errorf("Expected persistent flag --%s on root command", flagName)
		}
	}
}

func TestRootCommandProperties(t *testing.T) {
	if rootCmd.Use != "go-sort-out-gh-actions" {
		t.Errorf("Expected Use %q, got %q", "go-sort-out-gh-actions", rootCmd.Use)
	}
	if rootCmd.Short != "Detect archived GitHub Actions in repository workflows" {
		t.Errorf("Expected Short %q, got %q", "Detect archived GitHub Actions in repository workflows", rootCmd.Short)
	}
	if rootCmd.PersistentPreRun == nil {
		t.Error("Expected PersistentPreRun to be set on root command")
	}
}

func TestPersistentFlagShorthandsAndDefaults(t *testing.T) {
	tests := []struct {
		name        string
		shorthand   string
		wantDefault string
	}{
		{name: "debug", shorthand: "d", wantDefault: "false"},
		{name: "verbose", shorthand: "v", wantDefault: "false"},
		{name: "token", shorthand: "t", wantDefault: ""},
		{name: "output-format", shorthand: "o", wantDefault: "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := rootCmd.PersistentFlags().Lookup(tt.name)
			if flag == nil {
				t.Fatalf("Flag --%s not found", tt.name)
			}
			if flag.Shorthand != tt.shorthand {
				t.Errorf("Expected shorthand %q for --%s, got %q", tt.shorthand, tt.name, flag.Shorthand)
			}
			if flag.DefValue != tt.wantDefault {
				t.Errorf("Expected default %q for --%s, got %q", tt.wantDefault, tt.name, flag.DefValue)
			}
		})
	}

	noShorthandFlags := []string{"notify", "create-issue", "workflow", "workflows-dir", "repos-dir"}
	for _, name := range noShorthandFlags {
		flag := rootCmd.PersistentFlags().Lookup(name)
		if flag == nil {
			t.Fatalf("Flag --%s not found", name)
		}
		if flag.Shorthand != "" {
			t.Errorf("Expected no shorthand for --%s, got %q", name, flag.Shorthand)
		}
	}
}

func TestRootCmdPreRun(t *testing.T) {
	// NOTE: This test mutates the global log level and the package-level
	// 'debug' variable. It must NOT use t.Parallel() to avoid data races
	// with other tests that read or write the same globals.
	originalLevel := log.GetLevel()
	defer log.SetLevel(originalLevel)

	t.Run("debug true sets DebugLevel", func(t *testing.T) {
		log.SetLevel(log.InfoLevel)
		origDebug := debug
		debug = true
		defer func() { debug = origDebug }()

		rootCmdPreRun(nil, nil)

		if log.GetLevel() != log.DebugLevel {
			t.Errorf("Expected DebugLevel, got %v", log.GetLevel())
		}
	})

	t.Run("debug false leaves level unchanged", func(t *testing.T) {
		log.SetLevel(log.InfoLevel)
		origDebug := debug
		debug = false
		defer func() { debug = origDebug }()

		rootCmdPreRun(nil, nil)

		if log.GetLevel() != log.InfoLevel {
			t.Errorf("Expected InfoLevel, got %v", log.GetLevel())
		}
	})
}

func TestResolveToken(t *testing.T) {
	// NOTE: This test mutates the package-level 'conf' and 'githubToken'
	// variables. It must NOT use t.Parallel() to avoid data races with
	// other tests that read or write the same globals.
	origConf := conf
	origGithubToken := githubToken
	defer func() {
		conf = origConf
		githubToken = origGithubToken
	}()

	t.Run("from conf.GitHubToken", func(t *testing.T) {
		conf = config.Config{GitHubToken: "gh_token_value"}
		githubToken = ""
		token := resolveToken()
		if token != "gh_token_value" {
			t.Errorf("Expected %q, got %q", "gh_token_value", token)
		}
	})

	t.Run("fallback to conf.GitHubTokenFallback", func(t *testing.T) {
		conf = config.Config{GitHubToken: "", GitHubTokenFallback: "github_token_value"}
		githubToken = ""
		token := resolveToken()
		if token != "github_token_value" {
			t.Errorf("Expected %q, got %q", "github_token_value", token)
		}
	})

	t.Run("flag overrides env vars", func(t *testing.T) {
		conf = config.Config{GitHubToken: "env_token", GitHubTokenFallback: "fallback_token"}
		githubToken = "flag_token"
		token := resolveToken()
		if token != "flag_token" {
			t.Errorf("Expected %q, got %q", "flag_token", token)
		}
	})

	t.Run("flag overrides when env is empty", func(t *testing.T) {
		conf = config.Config{GitHubToken: "", GitHubTokenFallback: ""}
		githubToken = "flag_only_token"
		token := resolveToken()
		if token != "flag_only_token" {
			t.Errorf("Expected %q, got %q", "flag_only_token", token)
		}
	})

	t.Run("GH_TOKEN takes priority over GITHUB_TOKEN", func(t *testing.T) {
		conf = config.Config{GitHubToken: "primary", GitHubTokenFallback: "fallback"}
		githubToken = ""
		token := resolveToken()
		if token != "primary" {
			t.Errorf("Expected %q, got %q", "primary", token)
		}
	})
}

func TestResolveOutputFormat(t *testing.T) {
	// NOTE: This test mutates the package-level 'outputFormat' variable.
	// It must NOT use t.Parallel() to avoid data races with other tests
	// that read or write the same globals.
	origOutputFormat := outputFormat
	defer func() { outputFormat = origOutputFormat }()

	t.Run("text format", func(t *testing.T) {
		outputFormat = "text"
		f := resolveOutputFormat()
		if f != output.FormatText {
			t.Errorf("Expected FormatText, got %v", f)
		}
	})

	t.Run("json format", func(t *testing.T) {
		outputFormat = "json"
		f := resolveOutputFormat()
		if f != output.FormatJSON {
			t.Errorf("Expected FormatJSON, got %v", f)
		}
	})
}
