// Package mcp provides an MCP (Model Context Protocol) server for exposing
// go-sort-out-gh-actions checking capabilities to AI assistants and external tooling.
//
// This package wraps the application's Cobra commands as MCP tools using the
// ophis library, allowing AI assistants and other MCP clients to invoke the
// tool's action checking capabilities programmatically.
package mcp

// Config wraps ophis configuration for the MCP server.
type Config struct {
	// Addr is the host:port address for the MCP server's SSE transport.
	Addr string
	// Transport is the transport mode for the MCP server ("stdio" or "sse").
	Transport string
	// DefaultEnv captures environment variables to pass to editor-launched MCP servers.
	DefaultEnv map[string]string
}
