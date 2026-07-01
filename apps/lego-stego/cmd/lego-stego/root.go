package cmd

import (
	"github.com/spf13/cobra"
	"github.com/toozej/monogo/pkg/avatar"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

var rootCmd = &cobra.Command{
	Use:   "lego-stego",
	Short: "CLI wrapped steganography tool",
}

func init() {
	rootCmd.AddCommand(
		avatar.NewCommand("lego-stego"),
		man.NewManCmd(),
		version.Command(),
	)
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
