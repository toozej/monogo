package main

import (
	"os"
	"strings"
	"testing"
)

func TestDemoScriptUsesSupportedCommands(t *testing.T) {
	data, err := os.ReadFile("demo.sh")
	if err != nil {
		t.Fatalf("reading demo script: %v", err)
	}
	script := string(data)
	if strings.Contains(script, "--username") {
		t.Fatal("demo script uses the unsupported --username flag")
	}
	if !strings.Contains(script, `"${BIN}" version`) {
		t.Fatal("demo script does not run the version command")
	}
}
