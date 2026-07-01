package avatar

import (
	"errors"
	"strings"
	"testing"
)

func TestImageCapabilities_HasRichProtocol(t *testing.T) {
	tests := []struct {
		name string
		caps imageCapabilities
		want bool
	}{
		{name: "none but halfblocks", caps: imageCapabilities{Halfblocks: true}, want: false},
		{name: "kitty", caps: imageCapabilities{Kitty: true}, want: true},
		{name: "iterm2", caps: imageCapabilities{ITerm2: true}, want: true},
		{name: "sixel", caps: imageCapabilities{Sixel: true}, want: true},
		{name: "empty", caps: imageCapabilities{}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.caps.hasRichProtocol(); got != tt.want {
				t.Errorf("hasRichProtocol() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageCapabilities_Names(t *testing.T) {
	caps := imageCapabilities{Kitty: true, Sixel: true}
	names := caps.names()
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "Kitty") || !strings.Contains(joined, "Sixel") {
		t.Errorf("names() = %v, want Kitty and Sixel", names)
	}
	if strings.Contains(joined, "iTerm2") {
		t.Errorf("names() = %v, should not include iTerm2", names)
	}
}

func TestRenderFailureMessage_NoRichProtocol(t *testing.T) {
	msg := renderFailureMessage(imageCapabilities{Halfblocks: true}, errors.New("boom"))

	wantSubstrings := []string{
		"Failed to render avatar image: boom", // underlying error surfaced
		"does not support an inline image protocol",
		"Kitty graphics",
		"iTerm2",
		"Sixel",
		"kitty",   // terminal suggestion
		"Ghostty", // terminal suggestion
		"WezTerm", // terminal suggestion
		"tmux",    // multiplexer caveat
		"Falling back to ASCII art avatar",
	}
	for _, sub := range wantSubstrings {
		if !strings.Contains(msg, sub) {
			t.Errorf("message missing %q; got:\n%s", sub, msg)
		}
	}
}

func TestRenderFailureMessage_WithRichProtocol(t *testing.T) {
	msg := renderFailureMessage(imageCapabilities{ITerm2: true, Halfblocks: true}, errors.New("open ./img/avatar.png: no such file"))

	// When a rich protocol is present, blame the image, not the terminal, and
	// do not spam terminal suggestions.
	if !strings.Contains(msg, "problem with the image") {
		t.Errorf("expected image-source explanation; got:\n%s", msg)
	}
	if !strings.Contains(msg, "iTerm2 inline images") {
		t.Errorf("expected detected protocol to be named; got:\n%s", msg)
	}
	if strings.Contains(msg, "https://ghostty.org") {
		t.Errorf("should not suggest other terminals when a rich protocol is available; got:\n%s", msg)
	}
	if !strings.Contains(msg, "open ./img/avatar.png: no such file") {
		t.Errorf("expected underlying error surfaced; got:\n%s", msg)
	}
	if !strings.Contains(msg, "Falling back to ASCII art avatar") {
		t.Errorf("expected ASCII fallback notice; got:\n%s", msg)
	}
}
