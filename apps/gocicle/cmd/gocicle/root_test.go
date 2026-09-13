package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestUtilityCommandsDoNotLoadDatabase(t *testing.T) {
	t.Setenv("GOCICLE_DATABASE_URL", "invalid")
	t.Setenv("GOCICLE_KEY_FILE", "/missing")
	for _, args := range [][]string{{"version"}, {"--help"}, {"jobs", "--help"}, {"translate", "--help"}} {
		cmd := NewCommand()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if output.Len() == 0 {
			t.Fatal("utility command produced no output")
		}
	}
}
func TestKeygenDoesNotOverwriteKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	cmd := NewCommand()
	cmd.SetArgs([]string{"keygen", "--output", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("key file is not private")
	}
	cmd = NewCommand()
	cmd.SetArgs([]string{"keygen", "--output", path})
	if err := cmd.Execute(); err == nil {
		t.Fatal("keygen overwrote an existing key")
	}
}
