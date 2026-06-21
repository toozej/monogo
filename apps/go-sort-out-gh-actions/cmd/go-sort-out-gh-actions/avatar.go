package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/avatar"
)

func newAvatarCmd() *cobra.Command {
	var avatarURL string
	var avatarPath string
	var width int
	var height int

	cmd := &cobra.Command{
		Use:   "avatar",
		Short: "Print the project avatar in the terminal",
		Long: `Print the go-sort-out-gh-actions mascot avatar as a terminal image.
Requires a terminal that supports image rendering (iTerm2, kitty, etc).
Falls back to ASCII art if rendering fails.`,
		Args:   cobra.NoArgs,
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			runAvatar(avatarURL, avatarPath, width, height)
		},
	}

	cmd.Flags().StringVar(&avatarURL, "url", avatar.DefaultAvatarURL, "URL of the avatar image to render")
	cmd.Flags().StringVar(&avatarPath, "path", avatar.DefaultAvatarPath, "Path to a local avatar image file (overrides --url if set)")
	cmd.Flags().IntVar(&width, "width", 40, "Width of the rendered avatar in terminal cells")
	cmd.Flags().IntVar(&height, "height", 20, "Height of the rendered avatar in terminal rows")

	return cmd
}

func runAvatar(avatarURL, avatarPath string, width, height int) {
	var err error

	switch {
	case avatarPath != "":
		err = avatar.RenderFromFile(avatarPath, width, height, os.Stdout)
	case avatarURL != "":
		err = avatar.RenderFromURL(avatarURL, width, height, os.Stdout)
	default:
		avatar.PrintFallback(os.Stdout)
		return
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s Failed to render avatar image: %v\n", actioninfo.Emoji("⚠️ ", "[WARN] "), err)
		fmt.Fprintln(os.Stderr, "Falling back to ASCII art avatar:")
		avatar.PrintFallback(os.Stdout)
	}
}
