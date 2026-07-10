package cmd

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/lego-stego/internal/api"
)

var embedInput, embedOutput, embedURL, embedPassword string

var embedCmd = &cobra.Command{
	Use:   "embed",
	Short: "Embed a QR URL into an image",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		parsed, err := url.ParseRequestURI(embedURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("url must use http or https and include a host")
		}
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
