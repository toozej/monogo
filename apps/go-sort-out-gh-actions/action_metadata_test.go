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

	type actionSpec struct {
		command string
		flags   []string
	}
	actions := map[string]actionSpec{
		"action.yml": {
			command: "archived",
			flags:   []string{"verbose", "debug", "workflow", "workflows-dir", "repos-dir", "notify", "create-issue", "stale-days"},
		},
		"check/action.yml": {
			command: "check",
			flags:   []string{"verbose", "debug", "workflow", "workflows-dir", "repos-dir", "notify", "create-issue", "write", "semver", "stale-days"},
		},
		"check-archived/action.yml": {
			command: "archived",
			flags:   []string{"verbose", "debug", "workflow", "workflows-dir", "repos-dir", "notify", "create-issue", "stale-days"},
		},
		"check-outdated/action.yml": {
			command: "outdated",
			flags:   []string{"verbose", "debug", "workflow", "workflows-dir", "repos-dir", "update", "pin", "semver"},
		},
		"eol/action.yml": {
			command: "eol",
			flags:   []string{"verbose", "debug", "workflow", "workflows-dir", "repos-dir", "notify", "update", "stale-days"},
		},
	}
	inputExpression := regexp.MustCompile(`^--[^=]+=\$[{][{] inputs\.([a-z0-9-]+) [}][}]$`)

	for path, spec := range actions {
		path, spec := path, spec
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
			if len(metadata.Runs.Steps) != 0 {
				t.Fatal("Docker action metadata must not contain composite-action steps")
			}
			if got := metadata.Runs.Env["GH_TOKEN"]; got != "${{ inputs.token }}" {
				t.Errorf("runs.env.GH_TOKEN = %q, want token input expression", got)
			}
			if _, ok := metadata.Inputs["token"]; !ok {
				t.Error("token input is not declared")
			}
			if len(metadata.Runs.Env) != 1 {
				t.Errorf("runs.env = %v, want only GH_TOKEN", metadata.Runs.Env)
			}
			if len(metadata.Runs.Args) == 0 || metadata.Runs.Args[0] != spec.command {
				t.Fatalf("first argument = %q, want %q", metadata.Runs.Args, spec.command)
			}

			forwarded := make(map[string]bool)
			for _, argument := range metadata.Runs.Args[1:] {
				match := inputExpression.FindStringSubmatch(argument)
				if match == nil {
					t.Errorf("argument %q is not a non-empty --flag=${{ inputs.name }} value", argument)
					continue
				}
				if _, ok := metadata.Inputs[match[1]]; !ok {
					t.Errorf("argument %q references undeclared input %q", argument, match[1])
				}
				if match[1] == "token" {
					t.Errorf("token must be passed through runs.env, not command-line argument %q", argument)
				}
				if forwarded[match[1]] {
					t.Errorf("input %q is forwarded more than once", match[1])
				}
				forwarded[match[1]] = true
			}
			for _, flag := range spec.flags {
				if _, ok := metadata.Inputs[flag]; !ok {
					t.Errorf("required action input %q is not declared", flag)
				}
				if !forwarded[flag] {
					t.Errorf("declared action input %q is not forwarded", flag)
				}
			}
			for input := range metadata.Inputs {
				if input != "token" && !forwarded[input] {
					t.Errorf("declared action input %q is not forwarded", input)
				}
			}
			if len(forwarded) != len(spec.flags) {
				t.Errorf("forwarded inputs = %v, want exactly %v", forwarded, spec.flags)
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

func TestReleasedActionImageRunsAsRoot(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("Dockerfile.goreleaser.distroless")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FROM ") && strings.HasSuffix(line, ":nonroot") {
			t.Errorf("GitHub Docker action runtime must use the base image's default root user, got %q", line)
		}
		if strings.HasPrefix(line, "USER ") {
			t.Errorf("GitHub Docker action runtime must not select a non-root user, got %q", line)
		}
	}
}
