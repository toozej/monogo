package cmd

import (
	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/lego-stego/internal/api"
)

var revealInput, revealOutput, revealPassword string

var revealCmd = &cobra.Command{
	Use:   "reveal",
	Short: "Reveal hidden file",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		pw, err := readPassword(revealPassword)
		if err != nil {
			return err
		}

		data, err := api.ExtractFile(revealInput, pw)
		if err != nil {
			return err
		}

		return api.WriteFileAtomic(revealOutput, data, 0600)
	},
}

func init() {
	revealCmd.Flags().StringVarP(&revealInput, "input", "i", "", "stego image")
	revealCmd.Flags().StringVarP(&revealOutput, "output", "o", "", "output file")
	revealCmd.Flags().StringVar(&revealPassword, "password", "", "password")

	_ = revealCmd.MarkFlagRequired("input")
	_ = revealCmd.MarkFlagRequired("output")

	rootCmd.AddCommand(revealCmd)
}
