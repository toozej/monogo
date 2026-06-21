package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/avatar"
)

func TestNewAvatarCmd_ReturnsCommand(t *testing.T) {
	cmd := newAvatarCmd()

	if cmd == nil {
		t.Fatal("Expected non-nil command, got nil")
	}

	if cmd.Use != "avatar" {
		t.Errorf("Expected Use %q, got %q", "avatar", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short to be set, got empty string")
	}

	if !cmd.Hidden {
		t.Error("Expected Hidden == true, got false")
	}

	if cmd.Args == nil {
		t.Error("Expected Args to be set")
	}

	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("Expected NoArgs to accept no args, got error: %v", err)
	}

	if err := cobra.NoArgs(cmd, []string{"extra"}); err == nil {
		t.Error("Expected NoArgs to reject args")
	}
}

func TestNewAvatarCmd_Flags(t *testing.T) {
	cmd := newAvatarCmd()

	tests := []struct {
		name        string
		wantDefault string
	}{
		{name: "url", wantDefault: avatar.DefaultAvatarURL},
		{name: "path", wantDefault: avatar.DefaultAvatarPath},
		{name: "width", wantDefault: "40"},
		{name: "height", wantDefault: "20"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.name)
			if flag == nil {
				t.Fatalf("Expected --%s flag on avatar command", tt.name)
			}
			if flag.DefValue != tt.wantDefault {
				t.Errorf("Expected default %q for --%s, got %q", tt.wantDefault, tt.name, flag.DefValue)
			}
		})
	}
}

func TestNewAvatarCmd_Hidden(t *testing.T) {
	cmd := newAvatarCmd()

	if !cmd.Hidden {
		t.Error("Expected avatar command to be hidden")
	}
}

func TestNewAvatarCmd_ExecuteWithNoArgs(t *testing.T) {
	cmd := newAvatarCmd()

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Expected avatar command to execute without error, got: %v", err)
	}
}

func TestRunAvatar_FallbackWhenNoURLOrPath(t *testing.T) {
	// Capture stdout to verify fallback is printed
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runAvatar("", "", 40, 20)

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	io := &buf
	_, _ = io.ReadFrom(r)

	output := buf.String()
	if !strings.Contains(output, "go-sort-out-gh-actions") {
		t.Errorf("expected fallback ASCII art in output when no URL or path provided, got: %q", output)
	}
}

func TestRunAvatar_DefaultPathFallsBack(t *testing.T) {
	origStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	origStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	runAvatar("", "./img/nonexistent-avatar.png", 40, 20)

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var stdout bytes.Buffer
	_, _ = stdout.ReadFrom(rOut)

	var stderr bytes.Buffer
	_, _ = stderr.ReadFrom(rErr)

	if !strings.Contains(stdout.String(), "go-sort-out-gh-actions") {
		t.Errorf("expected fallback ASCII art in stdout, got: %q", stdout.String())
	}

	if !strings.Contains(stderr.String(), "Failed to render avatar image") {
		t.Errorf("expected error message in stderr, got: %q", stderr.String())
	}
}

func TestRunAvatar_InvalidPathFallsBack(t *testing.T) {
	// When path is given but invalid, it should fall back to ASCII
	origStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	origStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	runAvatar("", "/nonexistent/path.png", 40, 20)

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var stdout bytes.Buffer
	_, _ = stdout.ReadFrom(rOut)

	var stderr bytes.Buffer
	_, _ = stderr.ReadFrom(rErr)

	// Should print fallback to stdout
	if !strings.Contains(stdout.String(), "go-sort-out-gh-actions") {
		t.Errorf("expected fallback ASCII art in stdout, got: %q", stdout.String())
	}

	// Should print error to stderr
	if !strings.Contains(stderr.String(), "Failed to render avatar image") {
		t.Errorf("expected error message in stderr, got: %q", stderr.String())
	}
}
