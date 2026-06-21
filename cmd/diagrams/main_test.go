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

	if err := generateArchitectureDiagram(outputDir); err != nil {
		t.Fatalf("generateArchitectureDiagram returned error: %v", err)
	}

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

	if err := generateComponentDiagram(outputDir); err != nil {
		t.Fatalf("generateComponentDiagram returned error: %v", err)
	}

	dotPath := filepath.Join(outputDir, "go-diagrams", "components.dot")
	if _, err := os.Stat(dotPath); err != nil {
		t.Errorf("expected components.dot at %s, got error: %v", dotPath, err)
	}
}

func TestRun(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "diagrams")

	if err := run(outputDir); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	archDot := filepath.Join(outputDir, "go-diagrams", "architecture.dot")
	compDot := filepath.Join(outputDir, "go-diagrams", "components.dot")
	if _, err := os.Stat(archDot); err != nil {
		t.Errorf("expected architecture.dot at %s, got error: %v", archDot, err)
	}
	if _, err := os.Stat(compDot); err != nil {
		t.Errorf("expected components.dot at %s, got error: %v", compDot, err)
	}
}

func TestRun_MkdirAllError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test running as root")
	}
	err := run("/dev/null/impossible/path")
	if err == nil {
		t.Error("expected error from run with invalid output dir")
	}
}

func TestRun_ArchitectureDiagramError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test running as root")
	}
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "diagrams")
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	readOnlyDir := filepath.Join(outputDir, "go-diagrams")
	if err := os.MkdirAll(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to chmod readonly dir: %v", err)
	}

	err := generateArchitectureDiagram(outputDir)
	if err == nil {
		t.Error("expected error from generateArchitectureDiagram with readonly dir")
	}
}

func TestRun_ComponentDiagramError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test running as root")
	}
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "diagrams")
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}
	readOnlyDir := filepath.Join(outputDir, "go-diagrams")
	if err := os.MkdirAll(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to chmod readonly dir: %v", err)
	}

	err := generateComponentDiagram(outputDir)
	if err == nil {
		t.Error("expected error from generateComponentDiagram with readonly dir")
	}
}
