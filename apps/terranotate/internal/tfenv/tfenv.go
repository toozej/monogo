package tfenv

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// EnsureVersion ensures that the specified Terraform version is installed and selected using tfenv.
func EnsureVersion(version string) error {
	if version == "" {
		return nil
	}

	// Check if tfenv is installed
	if _, err := exec.LookPath("tfenv"); err != nil {
		return fmt.Errorf("tfenv is not installed or not in PATH: %w", err)
	}

	fmt.Printf("Ensuring Terraform version %s is installed...\n", version)

	// Install the version
	installCmd := exec.Command("tfenv", "install", version) // #nosec G204 -- version comes from trusted configuration
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("failed to install terraform version %s: %w", version, err)
	}

	// Use the version
	useCmd := exec.Command("tfenv", "use", version) // #nosec G204 -- version comes from trusted configuration
	useCmd.Stdout = os.Stdout
	useCmd.Stderr = os.Stderr
	if err := useCmd.Run(); err != nil {
		return fmt.Errorf("failed to use terraform version %s: %w", version, err)
	}

	// Modify PATH to prioritize the version selected by tfenv if necessary
	// Typically tfenv manages symlinks, so just running `terraform` should work if tfenv init is done.
	// However, usually `tfenv use` writes to .terraform-version or updates the shim.
	// We can verify by checking `terraform version`.

	// #nosec G204 - version comes from trusted configuration source
	verCmd := exec.Command("terraform", "version")
	output, err := verCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check terraform version: %w", err)
	}

	if !strings.Contains(string(output), version) {
		return fmt.Errorf("terraform version mismatch after tfenv use. Output: %s", string(output))
	}

	fmt.Printf("Successfully switched to Terraform %s\n", version)
	return nil
}
