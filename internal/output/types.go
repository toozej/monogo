// Package output provides formatting and output capabilities for check results.
//
// This package supports text, JSON, and CSV output formats. CSV output is
// designed for integration with external tools like Atlassian Jira bulk import
// and MCP-based ticketing systems.
package output

// CSVConfig holds configuration for CSV output, including extra columns
// for integration with external tools like Atlassian Jira bulk import.
type CSVConfig struct {
	// ExtraColumns is a map of custom column header names to pre-populated values,
	// parsed from --csv-additional-data flag as key=value pairs.
	ExtraColumns map[string]string
}
