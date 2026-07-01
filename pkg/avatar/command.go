package avatar

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewCommand returns the hidden "avatar" command for the named application.
// appName drives the ASCII fallback label and the command's long help text.
func NewCommand(appName string) *cobra.Command {
	var avatarURL string
	var avatarPath string
	var width int
	var height int

	cmd := &cobra.Command{
		Use:   "avatar",
		Short: "Print the project avatar in the terminal",
		Long: fmt.Sprintf(`Print the %s mascot avatar as a terminal image.
Requires a terminal that supports image rendering (iTerm2, kitty, etc).
Falls back to ASCII art if rendering fails.`, appName),
		Args:   cobra.NoArgs,
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			run(appName, avatarURL, avatarPath, width, height)
		},
	}

	cmd.Flags().StringVar(&avatarURL, "url", DefaultAvatarURL, "URL of the avatar image to render")
	cmd.Flags().StringVar(&avatarPath, "path", DefaultAvatarPath, "Path to a local avatar image file (overrides --url if set)")
	cmd.Flags().IntVar(&width, "width", 40, "Width of the rendered avatar in terminal cells")
	cmd.Flags().IntVar(&height, "height", 20, "Height of the rendered avatar in terminal rows")

	return cmd
}

func run(appName, avatarURL, avatarPath string, width, height int) {
	var err error

	switch {
	case avatarPath != "":
		err = RenderFromFile(avatarPath, width, height, os.Stdout)
	case avatarURL != "":
		err = RenderFromURL(avatarURL, width, height, os.Stdout)
	default:
		PrintFallback(os.Stdout, appName)
		return
	}

	if err != nil {
		fmt.Fprint(os.Stderr, renderFailureMessage(detectCapabilities(), err))
		PrintFallback(os.Stdout, appName)
	}
}
