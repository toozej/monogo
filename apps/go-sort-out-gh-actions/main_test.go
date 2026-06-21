package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestMain_VersionSubprocess(t *testing.T) {
	if os.Getenv("TEST_MAIN_VERSION") == "1" {
		os.Args = []string{"go-sort-out-gh-actions", "version"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMain_VersionSubprocess")
	cmd.Env = append(os.Environ(), "TEST_MAIN_VERSION=1", "GH_TOKEN=test-token")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected version command to succeed, got error: %v\noutput: %s", err, output)
	}
}
