package avatar

import (
	"fmt"
	"strings"

	termimg "github.com/blacktop/go-termimg"
)

// imageCapabilities describes which terminal inline-image protocols the current
// terminal supports, as reported by go-termimg.
type imageCapabilities struct {
	Kitty      bool
	ITerm2     bool
	Sixel      bool
	Halfblocks bool
}

// hasRichProtocol reports whether a high-fidelity inline image protocol
// (anything better than Unicode half-blocks) is available.
func (c imageCapabilities) hasRichProtocol() bool {
	return c.Kitty || c.ITerm2 || c.Sixel
}

// names returns the human-readable names of the supported rich protocols.
func (c imageCapabilities) names() []string {
	var n []string
	if c.Kitty {
		n = append(n, "Kitty graphics")
	}
	if c.ITerm2 {
		n = append(n, "iTerm2 inline images")
	}
	if c.Sixel {
		n = append(n, "Sixel")
	}
	return n
}

// detectCapabilities probes the current terminal via go-termimg. It is a
// package variable so tests can stub it without querying a real terminal.
var detectCapabilities = func() imageCapabilities {
	return imageCapabilities{
		Kitty:      termimg.KittySupported(),
		ITerm2:     termimg.ITerm2Supported(),
		Sixel:      termimg.SixelSupported(),
		Halfblocks: termimg.HalfblocksSupported(),
	}
}

// terminalSuggestions lists terminals that implement an inline image protocol
// supported by go-termimg (github.com/blacktop/go-termimg).
const terminalSuggestions = `To view the avatar, use a terminal that supports an inline image protocol:
  - kitty     https://sw.kovidgoyal.net/kitty/   (Kitty graphics protocol)
  - Ghostty   https://ghostty.org/               (Kitty graphics protocol)
  - WezTerm   https://wezterm.org/               (Kitty / iTerm2 protocols)
  - iTerm2    https://iterm2.com/                (iTerm2 inline images, macOS)
  - Sixel-capable: foot, xterm -ti vt340, mlterm, Konsole
Note: multiplexers like tmux or screen can block image passthrough.`

// renderFailureMessage builds a diagnostic, tailored to the terminal's detected
// image capabilities, explaining why the avatar image could not be shown. When
// the terminal supports a rich image protocol the failure is attributed to the
// image source; otherwise it explains the terminal limitation and suggests
// terminals that can render images.
func renderFailureMessage(caps imageCapabilities, err error) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[WARN] Failed to render avatar image: %v\n", err)

	if caps.hasRichProtocol() {
		fmt.Fprintf(&b, "Your terminal reports support for %s image rendering, so this is\n", strings.Join(caps.names(), " / "))
		b.WriteString("most likely a problem with the image itself (missing, unreadable, or not\n")
		b.WriteString("a valid image) rather than the terminal.\n")
	} else {
		b.WriteString("Your terminal does not support an inline image protocol (Kitty graphics,\n")
		b.WriteString("iTerm2 inline images, or Sixel), so the avatar image cannot be displayed.\n")
		b.WriteString(terminalSuggestions)
		b.WriteString("\n")
	}

	b.WriteString("Falling back to ASCII art avatar:\n")
	return b.String()
}
