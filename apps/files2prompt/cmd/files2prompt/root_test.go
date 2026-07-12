package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergePathsPreservesConfiguredPaths(t *testing.T) {
	assert.Equal(t,
		[]string{"configured", "argument", "stdin"},
		mergePaths([]string{"configured"}, []string{"argument"}, []string{"stdin"}),
	)
}

func TestAllConfigurationFlagsAreRegistered(t *testing.T) {
	for _, name := range []string{
		"extension",
		"include-hidden",
		"ignore-gitignore",
		"ignore",
		"output",
		"cxml",
		"line-numbers",
		"markdown",
		"null",
	} {
		assert.NotNilf(t, rootCmd.Flags().Lookup(name), "flag %q should always be registered", name)
	}
}
