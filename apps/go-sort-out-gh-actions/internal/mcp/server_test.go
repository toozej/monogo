package mcp

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewMCPCommand_NilConfig(t *testing.T) {
	cmd := NewMCPCommand(nil)

	if cmd == nil {
		t.Fatal("expected non-nil *cobra.Command, got nil")
	}

	if cmd.Use != "mcp" {
		t.Errorf("expected cmd.Use == %q, got %q", "mcp", cmd.Use)
	}

	if len(cmd.Commands()) == 0 {
		t.Error("expected at least one subcommand, got none")
	}
}

func TestNewMCPCommand_WithConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "config with default env",
			cfg: &Config{
				DefaultEnv: map[string]string{
					"FOO": "bar",
				},
			},
		},
		{
			name: "config with empty default env",
			cfg: &Config{
				DefaultEnv: map[string]string{},
			},
		},
		{
			name: "config with multiple env vars",
			cfg: &Config{
				DefaultEnv: map[string]string{
					"FOO":  "bar",
					"BAZ":  "qux",
					"PATH": "/custom/path",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewMCPCommand(tt.cfg)

			if cmd == nil {
				t.Fatal("expected non-nil *cobra.Command, got nil")
			}

			if cmd.Use != "mcp" {
				t.Errorf("expected cmd.Use == %q, got %q", "mcp", cmd.Use)
			}
		})
	}
}

func TestNewMCPCommand_Subcommands(t *testing.T) {
	cmd := NewMCPCommand(nil)

	expectedSubcommands := []string{"start", "stream", "tools"}

	subcmds := cmd.Commands()
	if len(subcmds) == 0 {
		t.Fatal("expected at least one subcommand, got none")
	}

	found := 0
	for _, expected := range expectedSubcommands {
		for _, sub := range subcmds {
			if sub.Name() == expected {
				found++
				break
			}
		}
	}

	if found == 0 {
		t.Errorf("expected to find at least one of %v subcommands, found none in %d total subcommands",
			expectedSubcommands, len(subcmds))
	}
}

func TestNewMCPCommand_ReturnsCobraCommand(t *testing.T) {
	cmd := NewMCPCommand(nil)

	_ = cmd
}

func TestConfig_DefaultEnv(t *testing.T) {
	tests := []struct {
		name       string
		defaultEnv map[string]string
		wantLen    int
		wantHasFOO bool
		wantFOOVal string
	}{
		{
			name:       "empty default env",
			defaultEnv: map[string]string{},
			wantLen:    0,
		},
		{
			name:       "nil default env",
			defaultEnv: nil,
			wantLen:    0,
		},
		{
			name:       "single env var",
			defaultEnv: map[string]string{"FOO": "bar"},
			wantLen:    1,
			wantHasFOO: true,
			wantFOOVal: "bar",
		},
		{
			name:       "multiple env vars",
			defaultEnv: map[string]string{"FOO": "bar", "BAZ": "qux"},
			wantLen:    2,
			wantHasFOO: true,
			wantFOOVal: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				DefaultEnv: tt.defaultEnv,
			}

			if len(cfg.DefaultEnv) != tt.wantLen {
				t.Errorf("expected DefaultEnv length %d, got %d", tt.wantLen, len(cfg.DefaultEnv))
			}

			if tt.wantHasFOO {
				if v, ok := cfg.DefaultEnv["FOO"]; !ok {
					t.Error("expected DefaultEnv to contain key FOO")
				} else if v != tt.wantFOOVal {
					t.Errorf("expected DefaultEnv[FOO] == %q, got %q", tt.wantFOOVal, v)
				}
			}
		})
	}
}

func TestNewMCPCommand_InvalidTransport(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	cfg := &Config{
		Transport: "invalid-transport",
	}
	cmd := NewMCPCommand(cfg)
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stderr: %v", err)
	}
	output := buf.String()
	if cmd == nil {
		t.Fatal("expected non-nil command even with invalid transport")
	}
	if !strings.Contains(output, "invalid MCP_TRANSPORT") {
		t.Errorf("expected warning about invalid transport, got: %s", output)
	}
}

func TestNewMCPCommand_ValidTransports(t *testing.T) {
	tests := []struct {
		name      string
		transport string
	}{
		{name: "stdio transport", transport: "stdio"},
		{name: "sse transport", transport: "sse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Transport: tt.transport,
			}
			cmd := NewMCPCommand(cfg)
			if cmd == nil {
				t.Fatal("expected non-nil command")
			}
		})
	}
}

func TestSanitizePATH(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "standard paths only",
			path: "/usr/bin:/usr/sbin:/bin:/sbin",
			want: "/usr/bin:/usr/sbin:/bin:/sbin",
		},
		{
			name: "filters out non-standard dirs",
			path: "/usr/bin:/home/user/.local/bin:/opt/custom/bin",
			want: "/usr/bin",
		},
		{
			name: "empty path gets fallback",
			path: "",
			want: "/usr/bin:/bin",
		},
		{
			name: "no standard dirs gets fallback",
			path: "/home/user/.local/bin:/opt/custom",
			want: "/usr/bin:/bin",
		},
		{
			name: "preserves order of allowed dirs",
			path: "/bin:/usr/local/bin:/usr/bin:/home/user/bin",
			want: "/bin:/usr/local/bin:/usr/bin",
		},
		{
			name: "all five standard paths",
			path: "/usr/bin:/usr/sbin:/usr/local/bin:/bin:/sbin",
			want: "/usr/bin:/usr/sbin:/usr/local/bin:/bin:/sbin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePATH(tt.path)
			if got != tt.want {
				t.Errorf("sanitizePATH(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
