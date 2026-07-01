package avatar

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewCommand_ReturnsCommand(t *testing.T) {
	cmd := NewCommand("example-app")

	if cmd == nil {
		t.Fatal("Expected non-nil command, got nil")
	}

	if cmd.Use != "avatar" {
		t.Errorf("Expected Use %q, got %q", "avatar", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short to be set, got empty string")
	}

	if !strings.Contains(cmd.Long, "example-app") {
		t.Errorf("Expected Long to mention the app name, got %q", cmd.Long)
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

func TestNewCommand_Flags(t *testing.T) {
	cmd := NewCommand("example-app")

	tests := []struct {
		name        string
		wantDefault string
	}{
		{name: "url", wantDefault: DefaultAvatarURL},
		{name: "path", wantDefault: DefaultAvatarPath},
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

func TestNewCommand_Hidden(t *testing.T) {
	cmd := NewCommand("example-app")

	if !cmd.Hidden {
		t.Error("Expected avatar command to be hidden")
	}
}

func TestNewCommand_ExecuteWithNoArgs(t *testing.T) {
	cmd := NewCommand("example-app")

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Expected avatar command to execute without error, got: %v", err)
	}
}

func TestRun_FallbackWhenNoURLOrPath(t *testing.T) {
	// Capture stdout to verify fallback is printed
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	run("example-app", "", "", 40, 20)

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	output := buf.String()
	if !strings.Contains(output, "example-app") {
		t.Errorf("expected fallback ASCII art in output when no URL or path provided, got: %q", output)
	}
}

func TestRun_DefaultPathFallsBack(t *testing.T) {
	origCaps := detectCapabilities
	defer func() { detectCapabilities = origCaps }()
	detectCapabilities = func() imageCapabilities { return imageCapabilities{Halfblocks: true} }

	origStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	origStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	run("example-app", "", "./img/nonexistent-avatar.png", 40, 20)

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var stdout bytes.Buffer
	_, _ = stdout.ReadFrom(rOut)

	var stderr bytes.Buffer
	_, _ = stderr.ReadFrom(rErr)

	if !strings.Contains(stdout.String(), "example-app") {
		t.Errorf("expected fallback ASCII art in stdout, got: %q", stdout.String())
	}

	if !strings.Contains(stderr.String(), "Failed to render avatar image") {
		t.Errorf("expected error message in stderr, got: %q", stderr.String())
	}
}

func TestRun_InvalidPathFallsBack(t *testing.T) {
	// When path is given but invalid, it should fall back to ASCII
	origCaps := detectCapabilities
	defer func() { detectCapabilities = origCaps }()
	detectCapabilities = func() imageCapabilities { return imageCapabilities{Halfblocks: true} }

	origStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	origStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	run("example-app", "", "/nonexistent/path.png", 40, 20)

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var stdout bytes.Buffer
	_, _ = stdout.ReadFrom(rOut)

	var stderr bytes.Buffer
	_, _ = stderr.ReadFrom(rErr)

	// Should print fallback to stdout
	if !strings.Contains(stdout.String(), "example-app") {
		t.Errorf("expected fallback ASCII art in stdout, got: %q", stdout.String())
	}

	// Should print error to stderr
	if !strings.Contains(stderr.String(), "Failed to render avatar image") {
		t.Errorf("expected error message in stderr, got: %q", stderr.String())
	}
}

func TestRun_URLPathWhenNoLocalPath(t *testing.T) {
	// With an empty path but a bogus URL, run should attempt the URL branch
	// and fall back to ASCII art on failure.
	origCaps := detectCapabilities
	defer func() { detectCapabilities = origCaps }()
	detectCapabilities = func() imageCapabilities { return imageCapabilities{Halfblocks: true} }

	origStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	origStderr := os.Stderr
	rErr, wErr, _ := os.Pipe()
	os.Stderr = wErr

	run("example-app", "http://insecure.example.com/a.png", "", 40, 20)

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var stdout bytes.Buffer
	_, _ = stdout.ReadFrom(rOut)

	var stderr bytes.Buffer
	_, _ = stderr.ReadFrom(rErr)

	if !strings.Contains(stdout.String(), "example-app") {
		t.Errorf("expected fallback ASCII art in stdout, got: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Failed to render avatar image") {
		t.Errorf("expected error message in stderr, got: %q", stderr.String())
	}
}
