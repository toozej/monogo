package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/toozej/lego-stego/pkg/api"
)

var hideInput, hideOutput, hideFile, hidePassword string

var hideCmd = &cobra.Command{
	Use:   "hide",
	Short: "Hide a file inside an image",
	RunE: func(cmd *cobra.Command, args []string) error {
		pw, err := readPassword(hidePassword)
		if err != nil {
			return err
		}

		// #nosec G304 -- path provided by user via CLI flag
		data, err := os.ReadFile(hideFile)
		if err != nil {
			return err
		}

		return api.EmbedFile(hideInput, hideOutput, data, pw)
	},
}

func init() {
	hideCmd.Flags().StringVarP(&hideInput, "input", "i", "", "carrier image")
	hideCmd.Flags().StringVarP(&hideOutput, "output", "o", "", "output image")
	hideCmd.Flags().StringVarP(&hideFile, "file", "f", "", "file to hide")
	hideCmd.Flags().StringVar(&hidePassword, "password", "", "password")

	_ = hideCmd.MarkFlagRequired("input")
	_ = hideCmd.MarkFlagRequired("output")
	_ = hideCmd.MarkFlagRequired("file")

	rootCmd.AddCommand(hideCmd)
}
