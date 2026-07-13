package starter

import (
	"bytes"
	"os"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid username", "Alice", "Hello from Alice\n"},
		{"empty username", "", "Hello from \n"},
		{"whitespace username", " ", "Hello from  \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := os.Stdout
			defer func() { os.Stdout = old }()

			r, w, _ := os.Pipe()
			os.Stdout = w

			Run(tt.input)

			_ = w.Close()
			var out bytes.Buffer
			_, _ = out.ReadFrom(r)

			if out.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, out.String())
			}
		})
	}
}

// TestRun_WritesToWriter exercises the run seam directly against an isolated
// buffer, avoiding os.Stdout redirection so the test carries no global state.
func TestRun_WritesToWriter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid username", "Alice", "Hello from Alice\n"},
		{"empty username", "", "Hello from \n"},
		{"whitespace username", " ", "Hello from  \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			run(&buf, tt.input)
			if buf.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, buf.String())
			}
		})
	}
}
