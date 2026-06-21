package cmd

import (
	"github.com/spf13/cobra"
	"github.com/toozej/lego-stego/pkg/version"
)

var rootCmd = &cobra.Command{
	Use:   "lego-stego",
	Short: "CLI wrapped steganography tool",
}

func init() {
	rootCmd.AddCommand(version.Command())
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
