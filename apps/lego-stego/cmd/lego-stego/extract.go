package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toozej/lego-stego/pkg/api"
)

var extractInput, extractOutput, extractPassword string

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract and decode QR from image",
	RunE: func(cmd *cobra.Command, args []string) error {
		pw, err := readPassword(extractPassword)
		if err != nil {
			return err
		}

		decoded, err := api.ExtractQR(extractInput, extractOutput, pw)
		if err != nil {
			return err
		}

		fmt.Println(decoded)
		return nil
	},
}

func init() {
	extractCmd.Flags().StringVarP(&extractInput, "input", "i", "", "stego image")
	extractCmd.Flags().StringVarP(&extractOutput, "output", "o", "", "output QR image")
	extractCmd.Flags().StringVar(&extractPassword, "password", "", "password")

	_ = extractCmd.MarkFlagRequired("input")
	_ = extractCmd.MarkFlagRequired("output")

	rootCmd.AddCommand(extractCmd)
}
