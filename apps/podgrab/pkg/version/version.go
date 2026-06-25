// Package version provides build and version information for the podgrab application.
//
// This package manages version metadata that is typically injected during the build process
// using ldflags. It provides both programmatic access to version information and JSON
// serialization for the /version HTTP endpoint.
//
// The version information includes:
//   - Version: Semantic version number (e.g., "v1.0.0")
//   - Commit: Git commit hash from build
//   - Branch: Git branch name from build
//   - BuiltAt: Build timestamp
//   - Builder: Build environment or CI system identifier
//
// Build-time injection example:
//
//	go build -ldflags "-X github.com/toozej/podgrab/pkg/version.Version=v1.0.0 \
//	  -X github.com/toozej/podgrab/pkg/version.Commit=abc123 \
//	  -X github.com/toozej/podgrab/pkg/version.Branch=main"
//
// Example usage:
//
//	import "github.com/toozej/podgrab/pkg/version"
//
//	// Get version info programmatically
//	info, err := version.Get()
//	if err == nil {
//		fmt.Printf("Version: %s\n", info.Version)
//	}
//
//	// Return JSON from HTTP handler
//	data, err := version.JSON()
package version

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// Version information variables that are populated by the build system.
//
// These variables are intended to be set during build time using Go's ldflags
// mechanism. If not set, they default to development-friendly values.
//
// Variables:
//   - Version: The semantic version of the application (default: "local")
//   - Commit: Git commit hash of the build (default: empty string)
//   - Branch: Git branch name of the build (default: empty string)
//   - BuiltAt: ISO timestamp of when the binary was built (default: empty string)
//   - Builder: Identifier of the build system/CI (default: empty string)
//
// Build-time injection example:
//
//	go build -ldflags "-X github.com/toozej/podgrab/pkg/version.Version=v1.2.3"
var (
	// Version represents the semantic version of the application.
	// Defaults to "local" for development builds.
	Version = "local"

	// Commit holds the Git commit hash from which the binary was built.
	// Empty by default, populated via build-time ldflags injection.
	Commit = ""

	// Branch contains the Git branch name from which the binary was built.
	// Empty by default, populated via build-time ldflags injection.
	Branch = ""

	// BuiltAt stores the timestamp when the binary was built.
	// Empty by default, typically populated with ISO format timestamp.
	BuiltAt = ""

	// Builder identifies the build environment or CI system.
	// Empty by default, can identify local, CI system, or builder.
	Builder = ""
)

// Info represents structured build and version information.
//
// This struct provides a structured way to access version metadata and
// is used for both programmatic access and JSON serialization for the
// version endpoint output.
//
// Fields:
//   - Version: Semantic version string
//   - Commit: Git commit hash
//   - Branch: Git branch name
//   - BuiltAt: Build timestamp
//   - Builder: Build environment identifier
//
// Example:
//
//	info := Info{
//		Version: "v1.0.0",
//		Commit:  "abc123def",
//		Branch:  "main",
//		BuiltAt: "2023-10-15T10:30:00Z",
//		Builder: "github-actions",
//	}
type Info struct {
	// Commit holds the Git commit hash from the build.
	Commit string `json:"Commit"`

	// Version contains the semantic version string.
	Version string `json:"Version"`

	// Branch specifies the Git branch used for the build.
	Branch string `json:"Branch"`

	// BuiltAt stores the build timestamp in ISO format.
	BuiltAt string `json:"BuiltAt"`

	// Builder identifies the build environment or CI system.
	Builder string `json:"Builder"`
}

// Get creates and returns an Info struct populated with current version information.
//
// This function collects all version metadata from the package-level variables
// and returns them in a structured Info object. It provides a programmatic
// way to access version information within the application.
//
// The returned Info struct contains the same data that would be displayed
// by the version endpoint, making it suitable for internal version checks,
// logging, telemetry, or other programmatic uses.
//
// Returns:
//   - Info: Populated version information struct
//   - error: Always nil in current implementation (reserved for future use)
//
// Example:
//
//	info, err := version.Get()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Running %s version %s\n", os.Args[0], info.Version)
//	if info.Commit != "" {
//		fmt.Printf("Built from commit %s\n", info.Commit)
//	}
func Get() (Info, error) {
	return Info{
		Commit:  Commit,
		Version: Version,
		Branch:  Branch,
		BuiltAt: BuiltAt,
		Builder: Builder,
	}, nil
}

// Command creates and returns a cobra command for displaying version information.
//
// This function constructs a "version" subcommand that outputs detailed build
// and version information in JSON format. The command is designed to be added
// to a root cobra command to provide standard version query functionality.
//
// Command characteristics:
//   - Use: "version" - command name for invocation
//   - Output: JSON-formatted version information
//   - Args: No arguments accepted
//   - Errors: Returns error if JSON marshaling or Info retrieval fails
//
// The JSON output includes all available version fields and follows a consistent
// format that can be parsed by scripts or other automated tools.
//
// Returns:
//   - *cobra.Command: Configured version command ready to be added to parent command
//
// Example:
//
//	// Add version command to root command
//	rootCmd.AddCommand(version.Command())
//
//	// Command line usage:
//	// ./podgrab version
//	// Output: {"Commit":"abc123","Version":"v1.0.0","Branch":"main",...}
func Command() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version.",
		Long:  `Print the version and build information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := Get()
			if err != nil {
				return err
			}
			jsonBytes, err := json.Marshal(info)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
			return nil
		},
	}
}

// JSON returns the version information as a JSON-encoded byte slice.
//
// This helper is used by the /version HTTP endpoint to return consistent
// JSON-formatted version metadata.
//
// Returns:
//   - []byte: JSON-encoded Info struct
//   - error: Error if JSON marshaling or Info retrieval fails
func JSON() ([]byte, error) {
	info, err := Get()
	if err != nil {
		return nil, err
	}
	bytes, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal version info: %w", err)
	}
	return bytes, nil
}
