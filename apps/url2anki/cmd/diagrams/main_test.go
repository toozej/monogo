package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/generic"
	"github.com/blushft/go-diagrams/nodes/programming"
)

func TestGeneratedAppMetadata(t *testing.T) {
	if appName != "url2anki" {
		t.Fatalf("appName = %q, want url2anki", appName)
	}
	if appBinary != "url2anki" {
		t.Fatalf("appBinary = %q, want url2anki", appBinary)
	}
	if outputDir != "docs/diagrams/url2anki" {
		t.Fatalf("outputDir = %q, want docs/diagrams/url2anki", outputDir)
	}
}

func TestGenerateArchitectureDiagram(t *testing.T) {
	tmpDir := t.TempDir()
	withWorkingDir(t, tmpDir, func() {
		generateArchitectureDiagram()
		assertFileExists(t, filepath.Join(tmpDir, "go-diagrams", "architecture.dot"))
	})
}

func TestGenerateComponentDiagram(t *testing.T) {
	tmpDir := t.TempDir()
	withWorkingDir(t, tmpDir, func() {
		generateComponentDiagram()
		assertFileExists(t, filepath.Join(tmpDir, "go-diagrams", "components.dot"))
	})
}

func TestBothDiagramsGenerated(t *testing.T) {
	tmpDir := t.TempDir()
	withWorkingDir(t, tmpDir, func() {
		generateArchitectureDiagram()
		generateComponentDiagram()

		assertFileExists(t, filepath.Join(tmpDir, "go-diagrams", "architecture.dot"))
		assertFileExists(t, filepath.Join(tmpDir, "go-diagrams", "components.dot"))
	})
}

func TestDiagramCreation(t *testing.T) {
	d, err := diagram.New(diagram.Filename("test-diagram"), diagram.Label("Test Diagram"), diagram.Direction("TB"))
	if err != nil {
		t.Fatalf("failed to create diagram: %v", err)
	}
	if d == nil {
		t.Fatal("expected diagram to be created")
	}
}

func TestArchitectureDiagramComponents(t *testing.T) {
	d, err := diagram.New(diagram.Filename("arch-test"), diagram.Label("Test Architecture"), diagram.Direction("TB"))
	if err != nil {
		t.Fatalf("failed to create diagram: %v", err)
	}

	user := generic.Blank.Blank(diagram.NodeLabel("User"))
	cli := programming.Language.Go(diagram.NodeLabel(appBinary))

	d.Connect(user, cli, diagram.Forward())

	tmpDir := t.TempDir()
	withWorkingDir(t, tmpDir, func() {
		if err := d.Render(); err != nil {
			t.Fatalf("failed to render diagram: %v", err)
		}

		assertFileExists(t, filepath.Join(tmpDir, "go-diagrams", "arch-test.dot"))
	})
}

func TestComponentDiagramConnections(t *testing.T) {
	d, err := diagram.New(diagram.Filename("comp-test"), diagram.Label("Test Components"), diagram.Direction("LR"))
	if err != nil {
		t.Fatalf("failed to create diagram: %v", err)
	}

	main := programming.Language.Go(diagram.NodeLabel("main.go"))
	rootCmd := programming.Language.Go(diagram.NodeLabel("cmd/" + appBinary))

	d.Connect(main, rootCmd, diagram.Forward())

	tmpDir := t.TempDir()
	withWorkingDir(t, tmpDir, func() {
		if err := d.Render(); err != nil {
			t.Fatalf("failed to render diagram: %v", err)
		}

		assertFileExists(t, filepath.Join(tmpDir, "go-diagrams", "comp-test.dot"))
	})
}

func TestDiagramDirectionOptions(t *testing.T) {
	for _, dir := range []string{"TB", "LR"} {
		t.Run(dir, func(t *testing.T) {
			d, err := diagram.New(diagram.Filename("dir-test"), diagram.Label("Direction Test"), diagram.Direction(dir))
			if err != nil {
				t.Fatalf("failed to create diagram with direction %s: %v", dir, err)
			}
			if d == nil {
				t.Fatalf("expected diagram with direction %s to be created", dir)
			}
		})
	}
}

func TestDiagramNodeLabels(t *testing.T) {
	node := generic.Blank.Blank(diagram.NodeLabel("TestNode"))
	if node == nil {
		t.Fatal("expected node to be created with label")
	}
}

func TestDiagramGoNodeLabels(t *testing.T) {
	node := programming.Language.Go(diagram.NodeLabel("GoComponent"))
	if node == nil {
		t.Fatal("expected Go node to be created with label")
	}
}

func TestHasGoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	if hasGoFiles(tmpDir) {
		t.Fatal("empty directory should not have Go files")
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatalf("failed to write Go file: %v", err)
	}

	if !hasGoFiles(tmpDir) {
		t.Fatal("directory with a Go file should have Go files")
	}
}

func TestPackageNodesMissingRoot(t *testing.T) {
	if nodes := packageNodes(filepath.Join(t.TempDir(), "missing"), "missing"); len(nodes) != 0 {
		t.Fatalf("packageNodes returned %d nodes for missing root, want 0", len(nodes))
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	}()

	fn()
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file to exist at %s", path)
	} else if err != nil {
		t.Fatalf("failed to stat %s: %v", path, err)
	}
}
