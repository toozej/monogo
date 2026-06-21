package cmd

import (
	"github.com/spf13/cobra"
	"github.com/toozej/monogo/pkg/lego-stego/man"
	"github.com/toozej/monogo/pkg/lego-stego/version"
)

var rootCmd = &cobra.Command{
	Use:   "lego-stego",
	Short: "CLI wrapped steganography tool",
}

func init() {
	rootCmd.AddCommand(
		man.NewManCmd(),
		version.Command(),
	)
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
