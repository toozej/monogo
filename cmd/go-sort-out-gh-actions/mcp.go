package cmd

import (
	"github.com/spf13/cobra"

	"github.com/toozej/go-sort-out-gh-actions/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	return mcp.NewMCPCommand(&mcp.Config{
		Addr:      conf.MCPAddr,
		Transport: conf.MCPTransport,
	})
}
