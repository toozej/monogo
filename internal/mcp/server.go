package mcp

import (
	"fmt"
	"os"

	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
)

// NewMCPCommand creates an MCP command tree using ophis that wraps the
// application's Cobra commands as MCP tools.
//
// The command tree includes:
// - mcp start: Start MCP server on stdio
// - mcp stream: Stream MCP server over HTTP
// - mcp tools: Export available MCP tools as JSON
// - mcp claude: Claude Desktop integration (enable/disable/list)
// - mcp vscode: VSCode integration (enable/disable/list)
// - mcp cursor: Cursor integration (enable/disable/list)
//
// Only the archived, outdated, eol, and check commands are exposed as MCP
// tools. The token flag is excluded to prevent sensitive credential exposure.
func NewMCPCommand(cfg *Config) *cobra.Command {
	if cfg != nil && cfg.Transport != "" {
		if cfg.Transport != "stdio" && cfg.Transport != "sse" {
			fmt.Fprintf(os.Stderr, "Warning: invalid MCP_TRANSPORT %q, falling back to stdio (valid: stdio, sse)\n", cfg.Transport)
		}
	}

	defaultEnv := map[string]string{
		"PATH": sanitizePATH(os.Getenv("PATH")),
	}

	if cfg != nil && len(cfg.DefaultEnv) > 0 {
		for k, v := range cfg.DefaultEnv {
			defaultEnv[k] = v
		}
	}

	ophisCfg := &ophis.Config{
		Selectors: []ophis.Selector{
			{
				CmdSelector:           ophis.AllowCmdsContaining("archived", "outdated", "eol", "check"),
				LocalFlagSelector:     ophis.ExcludeFlags("token"),
				InheritedFlagSelector: ophis.ExcludeFlags("token"),
			},
		},
		DefaultEnv: defaultEnv,
	}

	return ophis.Command(ophisCfg)
}

// sanitizePATH strips unusual or sensitive directory entries from PATH,
// keeping only standard system directories to avoid leaking host
// environment topology to MCP clients.
func sanitizePATH(path string) string {
	allowed := map[string]bool{
		"/usr/bin":       true,
		"/usr/sbin":      true,
		"/usr/local/bin": true,
		"/bin":           true,
		"/sbin":          true,
	}

	var filtered []string
	for _, dir := range splitPath(path) {
		if allowed[dir] {
			filtered = append(filtered, dir)
		}
	}
	if len(filtered) == 0 {
		return "/usr/bin:/bin"
	}
	return joinPath(filtered)
}

func splitPath(p string) []string {
	if p == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == os.PathListSeparator {
			result = append(result, p[start:i])
			start = i + 1
		}
	}
	result = append(result, p[start:])
	return result
}

func joinPath(parts []string) string {
	sep := string(os.PathListSeparator)
	result := parts[0]
	for _, p := range parts[1:] {
		result += sep + p
	}
	return result
}
