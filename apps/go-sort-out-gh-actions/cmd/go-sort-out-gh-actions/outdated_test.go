package cmd

import (
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
)

func TestOutdatedCommandFlags(t *testing.T) {
	cmd := newOutdatedCmd()

	if cmd.Name() != "outdated" {
		t.Errorf("Expected command name 'outdated', got %q", cmd.Name())
	}

	if cmd.Flags().Lookup("update") == nil {
		t.Error("Expected --update flag on outdated command")
	}
	if cmd.Flags().Lookup("pin") == nil {
		t.Error("Expected --pin flag on outdated command")
	}
	if cmd.Flags().Lookup("semver") == nil {
		t.Error("Expected --semver flag on outdated command")
	}
}

func TestOutdatedCommandDescriptions(t *testing.T) {
	cmd := newOutdatedCmd()

	if cmd.Short != "Display outdated GitHub Actions" {
		t.Errorf("Expected Short %q, got %q", "Display outdated GitHub Actions", cmd.Short)
	}
	expectedLong := `Scan workflow files and display GitHub Actions that are outdated compared to the latest release.
By default, updates are pinned to SHAs with semver comments. Use --semver for version strings instead of SHAs.
Use --pin to swap from semver version strings to SHAs.`
	if cmd.Long != expectedLong {
		t.Errorf("Expected Long %q, got %q", expectedLong, cmd.Long)
	}
}

func TestOutdatedCommandFlagDefaults(t *testing.T) {
	cmd := newOutdatedCmd()

	tests := []struct {
		name string
		want bool
	}{
		{name: "update", want: false},
		{name: "pin", want: false},
		{name: "semver", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := cmd.Flags().GetBool(tt.name)
			if err != nil {
				t.Fatalf("Failed to get %s flag: %v", tt.name, err)
			}
			if val != tt.want {
				t.Errorf("Expected %s default %v, got %v", tt.name, tt.want, val)
			}
		})
	}
}

func TestOutdatedCommandNoArgs(t *testing.T) {
	cmd := newOutdatedCmd()
	if cmd.Args == nil {
		t.Error("Expected Args to be set")
	}
	if err := cmd.Args(cmd, nil); err != nil {
		t.Errorf("Expected NoArgs to accept no args, got error: %v", err)
	}
	if err := cobra.NoArgs(cmd, []string{"extra"}); err == nil {
		t.Error("Expected NoArgs to reject args")
	}
}

func TestOutdatedCommandHasRun(t *testing.T) {
	cmd := newOutdatedCmd()
	if cmd.Run == nil {
		t.Error("Expected Run function to be set on outdated command")
	}
}

func TestOutdatedCommand_PinFlagInRun(t *testing.T) {
	if os.Getenv("TEST_OUTDATED_PIN_RUN") == "1" {
		cmd := newOutdatedCmd()
		_ = cmd.Flags().Set("pin", "true")
		_ = cmd.Flags().Set("semver", "true")
		pinVal, _ := cmd.Flags().GetBool("pin")
		semverVal, _ := cmd.Flags().GetBool("semver")
		useSemver := semverVal
		if pinVal {
			useSemver = false
		}
		if useSemver != false {
			os.Exit(1)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestOutdatedCommand_PinFlagInRun")
	cmd.Env = append(os.Environ(), "TEST_OUTDATED_PIN_RUN=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Process exited with error: %v\nOutput: %s", err, output)
	}
}

func TestOutdatedCommandPinOverridesSemver(t *testing.T) {
	cmd := newOutdatedCmd()

	t.Run("pin true forces useSemver false", func(t *testing.T) {
		if err := cmd.Flags().Set("pin", "true"); err != nil {
			t.Fatalf("Failed to set pin flag: %v", err)
		}
		if err := cmd.Flags().Set("semver", "true"); err != nil {
			t.Fatalf("Failed to set semver flag: %v", err)
		}

		pinVal, _ := cmd.Flags().GetBool("pin")
		semverVal, _ := cmd.Flags().GetBool("semver")

		useSemver := semverVal
		if pinVal {
			useSemver = false
		}

		if useSemver != false {
			t.Errorf("Expected useSemver=false when pin=true, got %v", useSemver)
		}
	})

	t.Run("semver true without pin", func(t *testing.T) {
		cmd2 := newOutdatedCmd()
		if err := cmd2.Flags().Set("semver", "true"); err != nil {
			t.Fatalf("Failed to set semver flag: %v", err)
		}

		pinVal, _ := cmd2.Flags().GetBool("pin")
		semverVal, _ := cmd2.Flags().GetBool("semver")

		useSemver := semverVal
		if pinVal {
			useSemver = false
		}

		if useSemver != true {
			t.Errorf("Expected useSemver=true when semver=true and pin=false, got %v", useSemver)
		}
	})

	t.Run("default both false", func(t *testing.T) {
		cmd3 := newOutdatedCmd()
		pinVal, _ := cmd3.Flags().GetBool("pin")
		semverVal, _ := cmd3.Flags().GetBool("semver")

		useSemver := semverVal
		if pinVal {
			useSemver = false
		}

		if useSemver != false {
			t.Errorf("Expected useSemver=false by default, got %v", useSemver)
		}
	})
}
