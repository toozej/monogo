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

			w.Close()
			var out bytes.Buffer
			_, _ = out.ReadFrom(r)

			if out.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, out.String())
			}
		})
	}
}
