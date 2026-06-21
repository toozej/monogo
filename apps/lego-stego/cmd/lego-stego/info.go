package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/toozej/lego-stego/internal/steg"
)

var infoInput string

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Inspect stego image",
	RunE: func(cmd *cobra.Command, args []string) error {
		// #nosec G304 -- path provided by user via CLI flag
		data, err := os.ReadFile(infoInput)
		if err != nil {
			return err
		}

		h, _, err := steg.DecodeHeader(data)
		if err != nil {
			fmt.Println("No payload detected")
			return nil
		}

		fmt.Printf("Version: %d\n", h.Version)
		fmt.Printf("Flags: %d\n", h.Flags)
		fmt.Printf("Payload length: %d\n", h.Length)

		return nil
	},
}

func init() {
	infoCmd.Flags().StringVarP(&infoInput, "input", "i", "", "image file")
	_ = infoCmd.MarkFlagRequired("input")
	rootCmd.AddCommand(infoCmd)
}
