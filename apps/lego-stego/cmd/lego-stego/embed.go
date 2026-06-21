package cmd

import (
	"github.com/spf13/cobra"
	"github.com/toozej/lego-stego/pkg/api"
)

var embedInput, embedOutput, embedURL, embedPassword string

var embedCmd = &cobra.Command{
	Use:   "embed",
	Short: "Embed a QR URL into an image",
	RunE: func(cmd *cobra.Command, args []string) error {
		pw, err := readPassword(embedPassword)
		if err != nil {
			return err
		}

		return api.EmbedQR(embedInput, embedOutput, embedURL, pw)
	},
}

func init() {
	embedCmd.Flags().StringVarP(&embedInput, "input", "i", "", "carrier image")
	embedCmd.Flags().StringVarP(&embedOutput, "output", "o", "", "output image")
	embedCmd.Flags().StringVarP(&embedURL, "url", "u", "", "URL to encode")
	embedCmd.Flags().StringVar(&embedPassword, "password", "", "password")

	_ = embedCmd.MarkFlagRequired("input")
	_ = embedCmd.MarkFlagRequired("output")
	_ = embedCmd.MarkFlagRequired("url")

	rootCmd.AddCommand(embedCmd)
}
