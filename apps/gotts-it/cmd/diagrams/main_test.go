package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/generic"
	"github.com/blushft/go-diagrams/nodes/programming"
)

func TestGenerateArchitectureDiagram(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	generateArchitectureDiagram()

	expectedFile := filepath.Join(tmpDir, "go-diagrams", "architecture.dot")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		entries, _ := os.ReadDir(tmpDir)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected architecture.dot at %s; dir contents: %v", expectedFile, names)
	}
}

func TestGenerateComponentDiagram(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	generateComponentDiagram()

	expectedFile := filepath.Join(tmpDir, "go-diagrams", "components.dot")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		entries, _ := os.ReadDir(tmpDir)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected components.dot at %s; dir contents: %v", expectedFile, names)
	}
}

func TestDiagramCreation(t *testing.T) {
	d, err := diagram.New(diagram.Filename("test-diagram"), diagram.Label("Test Diagram"), diagram.Direction("TB"))
	if err != nil {
		t.Fatalf("Failed to create diagram: %v", err)
	}
	if d == nil {
		t.Error("expected diagram to be created, got nil")
	}
}

func TestArchitectureDiagramComponents(t *testing.T) {
	d, err := diagram.New(diagram.Filename("arch-test"), diagram.Label("Test Architecture"), diagram.Direction("TB"))
	if err != nil {
		t.Fatalf("Failed to create diagram: %v", err)
	}

	user := generic.Blank.Blank(diagram.NodeLabel("User"))
	cli := programming.Language.Go(diagram.NodeLabel("CLI Application"))

	d.Connect(user, cli, diagram.Forward())

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	if err := d.Render(); err != nil {
		t.Fatalf("Failed to render diagram: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "go-diagrams", "arch-test.dot")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected arch-test.dot at %s", expectedFile)
	}
}

func TestComponentDiagramConnections(t *testing.T) {
	d, err := diagram.New(diagram.Filename("comp-test"), diagram.Label("Test Components"), diagram.Direction("LR"))
	if err != nil {
		t.Fatalf("Failed to create diagram: %v", err)
	}

	main := programming.Language.Go(diagram.NodeLabel("main.go"))
	rootCmd := programming.Language.Go(diagram.NodeLabel("cmd/gotts-it"))

	d.Connect(main, rootCmd, diagram.Forward())

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	if err := d.Render(); err != nil {
		t.Fatalf("Failed to render diagram: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "go-diagrams", "comp-test.dot")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected comp-test.dot at %s", expectedFile)
	}
}

func TestDiagramDirectionOptions(t *testing.T) {
	directions := []string{"TB", "LR"}
	for _, dir := range directions {
		t.Run(dir, func(t *testing.T) {
			d, err := diagram.New(diagram.Filename("dir-test"), diagram.Label("Direction Test"), diagram.Direction(dir))
			if err != nil {
				t.Fatalf("Failed to create diagram with direction %s: %v", dir, err)
			}
			if d == nil {
				t.Errorf("expected diagram with direction %s to be created, got nil", dir)
			}
		})
	}
}

func TestDiagramNodeLabels(t *testing.T) {
	node := generic.Blank.Blank(diagram.NodeLabel("TestNode"))
	if node == nil {
		t.Error("expected node to be created with label, got nil")
	}
}

func TestDiagramGoNodeLabels(t *testing.T) {
	node := programming.Language.Go(diagram.NodeLabel("GoComponent"))
	if node == nil {
		t.Error("expected Go node to be created with label, got nil")
	}
}

func TestBothDiagramsGenerated(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	generateArchitectureDiagram()
	generateComponentDiagram()

	archFile := filepath.Join(tmpDir, "go-diagrams", "architecture.dot")
	compFile := filepath.Join(tmpDir, "go-diagrams", "components.dot")

	if _, err := os.Stat(archFile); os.IsNotExist(err) {
		t.Error("expected architecture.dot to be generated")
	}
	if _, err := os.Stat(compFile); os.IsNotExist(err) {
		t.Error("expected components.dot to be generated")
	}
}

func TestMkdirAllForOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := fmt.Sprintf("%s/docs/diagrams", tmpDir)
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Errorf("expected output directory to exist at %s", outputDir)
	}
}

func TestRun(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(workDir, 0750); err != nil {
		t.Fatalf("Failed to create work directory: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	if err := run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	archFile := filepath.Join(workDir, "docs", "diagrams", "go-diagrams", "architecture.dot")
	if _, err := os.Stat(archFile); os.IsNotExist(err) {
		t.Error("expected architecture.dot to be generated")
	}
	compFile := filepath.Join(workDir, "docs", "diagrams", "go-diagrams", "components.dot")
	if _, err := os.Stat(compFile); os.IsNotExist(err) {
		t.Error("expected components.dot to be generated")
	}
}

func TestRun_MkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "docs")
	if err := os.WriteFile(filePath, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	err = run()
	if err == nil {
		t.Error("expected error when MkdirAll fails due to file blocking path")
	}
}

func TestMain_MkdirAll(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	subDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatalf("Failed to create sub directory: %v", err)
	}

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	if err := os.MkdirAll("docs/diagrams", 0750); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}
	if err := os.Chdir("docs/diagrams"); err != nil {
		t.Fatalf("Failed to change to diagrams directory: %v", err)
	}

	generateArchitectureDiagram()
	generateComponentDiagram()

	archFile := filepath.Join(subDir, "docs", "diagrams", "go-diagrams", "architecture.dot")
	if _, err := os.Stat(archFile); os.IsNotExist(err) {
		t.Errorf("expected architecture.dot to be generated at %s", archFile)
	}
	compFile := filepath.Join(subDir, "docs", "diagrams", "go-diagrams", "components.dot")
	if _, err := os.Stat(compFile); os.IsNotExist(err) {
		t.Errorf("expected components.dot to be generated at %s", compFile)
	}
}

func TestArchitectureDiagram_Connections(t *testing.T) {
	d, err := diagram.New(diagram.Filename("arch-conn-test"), diagram.Label("Arch Connections"), diagram.Direction("TB"))
	if err != nil {
		t.Fatalf("Failed to create diagram: %v", err)
	}

	user := generic.Blank.Blank(diagram.NodeLabel("User"))
	cli := programming.Language.Go(diagram.NodeLabel("CLI Application"))
	config := generic.Blank.Blank(diagram.NodeLabel("Config"))
	starter := programming.Language.Go(diagram.NodeLabel("Starter"))
	logging := generic.Blank.Blank(diagram.NodeLabel("Logging"))

	d.Connect(user, cli, diagram.Forward())
	d.Connect(cli, config, diagram.Forward())
	d.Connect(cli, starter, diagram.Forward())
	d.Connect(cli, logging, diagram.Forward())

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	if err := d.Render(); err != nil {
		t.Fatalf("Failed to render diagram: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "go-diagrams", "arch-conn-test.dot")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected arch-conn-test.dot at %s", expectedFile)
	}
}

func TestComponentDiagram_Connections(t *testing.T) {
	d, err := diagram.New(diagram.Filename("comp-conn-test"), diagram.Label("Comp Connections"), diagram.Direction("LR"))
	if err != nil {
		t.Fatalf("Failed to create diagram: %v", err)
	}

	main := programming.Language.Go(diagram.NodeLabel("main.go"))
	rootCmd := programming.Language.Go(diagram.NodeLabel("cmd/gotts-it"))
	config := programming.Language.Go(diagram.NodeLabel("pkg/config"))
	starter := programming.Language.Go(diagram.NodeLabel("internal/starter"))
	version := programming.Language.Go(diagram.NodeLabel("pkg/version"))
	man := programming.Language.Go(diagram.NodeLabel("pkg/man"))

	d.Connect(main, rootCmd, diagram.Forward())
	d.Connect(rootCmd, config, diagram.Forward())
	d.Connect(rootCmd, starter, diagram.Forward())
	d.Connect(rootCmd, version, diagram.Forward())
	d.Connect(rootCmd, man, diagram.Forward())

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	if err := d.Render(); err != nil {
		t.Fatalf("Failed to render diagram: %v", err)
	}

	expectedFile := filepath.Join(tmpDir, "go-diagrams", "comp-conn-test.dot")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected comp-conn-test.dot at %s", expectedFile)
	}
}
