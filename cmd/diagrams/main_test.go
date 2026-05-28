package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateArchitectureDiagram(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "diagrams")
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	generateArchitectureDiagram(outputDir)

	dotPath := filepath.Join(outputDir, "go-diagrams", "architecture.dot")
	if _, err := os.Stat(dotPath); err != nil {
		t.Errorf("expected architecture.dot at %s, got error: %v", dotPath, err)
	}
}

func TestGenerateComponentDiagram(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "diagrams")
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	generateComponentDiagram(outputDir)

	dotPath := filepath.Join(outputDir, "go-diagrams", "components.dot")
	if _, err := os.Stat(dotPath); err != nil {
		t.Errorf("expected components.dot at %s, got error: %v", dotPath, err)
	}
}
