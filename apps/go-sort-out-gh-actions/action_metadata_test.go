package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type actionMetadata struct {
	Inputs  map[string]map[string]any `yaml:"inputs"`
	Outputs map[string]struct {
		Description string `yaml:"description"`
		Value       string `yaml:"value"`
	} `yaml:"outputs"`
	Runs struct {
		Using string         `yaml:"using"`
		Image string         `yaml:"image"`
		Args  []string       `yaml:"args"`
		Env   map[string]any `yaml:"env"`
		Steps []any          `yaml:"steps"`
	} `yaml:"runs"`
}

func TestContainerActionMetadata(t *testing.T) {
	t.Parallel()

	actions := map[string]string{
		"action.yml":                "archived",
		"check/action.yml":          "check",
		"check-archived/action.yml": "archived",
		"check-outdated/action.yml": "outdated",
		"eol/action.yml":            "eol",
	}
	inputExpression := regexp.MustCompile(`^--[^=]+=\$[{][{] inputs\.([a-z0-9-]+) [}][}]$`)

	for path, command := range actions {
		path, command := path, command
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var metadata actionMetadata
			if err := yaml.Unmarshal(contents, &metadata); err != nil {
				t.Fatalf("parse action metadata: %v", err)
			}

			if metadata.Runs.Using != "docker" {
				t.Fatalf("runs.using = %q, want docker", metadata.Runs.Using)
			}
			if metadata.Runs.Image == "" {
				t.Fatal("container action has no image")
			}
			if len(metadata.Runs.Env) != 0 || len(metadata.Runs.Steps) != 0 {
				t.Fatal("Docker action metadata must not contain composite-action env or steps")
			}
			if len(metadata.Runs.Args) == 0 || metadata.Runs.Args[0] != command {
				t.Fatalf("first argument = %q, want %q", metadata.Runs.Args, command)
			}

			for _, argument := range metadata.Runs.Args[1:] {
				match := inputExpression.FindStringSubmatch(argument)
				if match == nil {
					t.Errorf("argument %q is not a non-empty --flag=${{ inputs.name }} value", argument)
					continue
				}
				if _, ok := metadata.Inputs[match[1]]; !ok {
					t.Errorf("argument %q references undeclared input %q", argument, match[1])
				}
			}
			for name, output := range metadata.Outputs {
				if output.Description == "" {
					t.Errorf("output %q has no description", name)
				}
				if output.Value != "" {
					t.Errorf("Docker action output %q has composite-only value %q", name, output.Value)
				}
			}
		})
	}
}

func TestExamplesUseMonorepoActionPaths(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("examples/workflows/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, "find-archived-repos-workflow-example.yaml")
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(contents), "\n") {
			if strings.Contains(line, "uses: toozej/") && !strings.Contains(line, "uses: toozej/monogo/apps/go-sort-out-gh-actions") {
				t.Errorf("%s contains stale action reference %q", path, strings.TrimSpace(line))
			}
		}
	}
}
