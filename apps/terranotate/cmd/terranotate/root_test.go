package cmd

import "testing"

func TestTerraformVersionFlagIsInheritedBySubcommands(t *testing.T) {
	if flag := validateCmd.Flag("terraform-version"); flag == nil {
		t.Fatal("validate command does not inherit --terraform-version")
	}
}
