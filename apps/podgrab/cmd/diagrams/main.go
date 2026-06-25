// Package main provides diagram generation utilities for the podgrab app.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/generic"
	"github.com/blushft/go-diagrams/nodes/programming"
)

const (
	appName     = "podgrab"
	appBinary   = "podgrab"
	appPath     = "apps/podgrab"
	appMainPath = "apps/podgrab"
	outputDir   = "docs/diagrams/podgrab"
)

var sourceRoot = "."

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal("failed to get working directory:", err)
	}
	sourceRoot = cwd

	if err := os.MkdirAll(outputDir, 0750); err != nil {
		log.Fatal("failed to create output directory:", err)
	}

	if err := os.Chdir(outputDir); err != nil {
		log.Fatal("failed to change directory:", err)
	}

	generateArchitectureDiagram()
	generateComponentDiagram()

	fmt.Printf("Diagram .dot files generated successfully in ./%s/go-diagrams/\n", outputDir)
}

func generateArchitectureDiagram() {
	d, err := diagram.New(diagram.Filename("architecture"), diagram.Label(appName+" Architecture"), diagram.Direction("TB"))
	if err != nil {
		log.Fatal(err)
	}

	user := generic.Blank.Blank(diagram.NodeLabel("User"))
	app := programming.Language.Go(diagram.NodeLabel(appBinary + "\nCLI"))
	config := generic.Blank.Blank(diagram.NodeLabel("app.yaml"))
	source := programming.Language.Go(diagram.NodeLabel(appPath))
	shared := programming.Language.Go(diagram.NodeLabel("pkg"))
	release := generic.Blank.Blank(diagram.NodeLabel("Docker\nrelease"))

	d.Connect(user, app, diagram.Forward())
	d.Connect(app, config, diagram.Forward())
	d.Connect(app, source, diagram.Forward())
	d.Connect(source, shared, diagram.Forward())
	d.Connect(config, release, diagram.Forward())

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}

func generateComponentDiagram() {
	d, err := diagram.New(diagram.Filename("components"), diagram.Label(appName+" Components"), diagram.Direction("LR"))
	if err != nil {
		log.Fatal(err)
	}

	mainNode := programming.Language.Go(diagram.NodeLabel(appMainPath + "\nmain.go"))
	cmdNodes := commandNodes()
	internalNodes := packageNodes(sourcePath(appPath, "internal"), "internal")
	sharedNodes := packageNodes(sourcePath("pkg"), "pkg")

	if len(cmdNodes) == 0 {
		cmdNodes = append(cmdNodes, programming.Language.Go(diagram.NodeLabel(filepath.Join(appPath, "cmd"))))
	}

	for _, cmdNode := range cmdNodes {
		d.Connect(mainNode, cmdNode, diagram.Forward())

		for _, internalNode := range internalNodes {
			d.Connect(cmdNode, internalNode, diagram.Forward())
		}

		for _, sharedNode := range sharedNodes {
			d.Connect(cmdNode, sharedNode, diagram.Forward())
		}
	}

	if err := d.Render(); err != nil {
		log.Fatal(err)
	}
}

func commandNodes() []*diagram.Node {
	cmdRoot := sourcePath(appPath, "cmd")
	entries, err := os.ReadDir(cmdRoot)
	if err != nil {
		return nil
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && hasGoFiles(filepath.Join(cmdRoot, entry.Name())) && entry.Name() != "diagrams" {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)

	nodes := make([]*diagram.Node, 0, len(dirs))
	for _, dir := range dirs {
		nodes = append(nodes, programming.Language.Go(diagram.NodeLabel(filepath.Join("cmd", dir))))
	}

	return nodes
}

func packageNodes(root string, labelRoot string) []*diagram.Node {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && hasGoFiles(filepath.Join(root, entry.Name())) {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)

	nodes := make([]*diagram.Node, 0, len(dirs))
	for _, dir := range dirs {
		nodes = append(nodes, programming.Language.Go(diagram.NodeLabel(filepath.Join(labelRoot, dir))))
	}

	return nodes
}

func hasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true
		}
	}

	return false
}

func sourcePath(parts ...string) string {
	pathParts := append([]string{sourceRoot}, parts...)
	return filepath.Join(pathParts...)
}
