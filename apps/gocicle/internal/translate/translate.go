// Package translate parses supported workflow syntax without executing source code.
package translate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"gopkg.in/yaml.v3"
)

type Diagnostic struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}
type Options struct {
	Project     jobs.Project
	Image       string
	Labels      map[string][]string
	Credentials map[string]string
	Draft       bool
}
type Result struct {
	Document    jobs.Document     `json:"document"`
	Scripts     map[string]string `json:"scripts"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
}

var ErrIncomplete = errors.New("workflow conversion is incomplete")

func Convert(kind, file string, data []byte, options Options) (Result, error) {
	r := Result{Document: jobs.Document{APIVersion: jobs.APIVersion, Project: options.Project, Jobs: map[string]jobs.Spec{}}, Scripts: map[string]string{}}
	if len(data) > jobs.MaxDocument {
		return r, errors.New("workflow exceeds 1 MiB")
	}
	switch kind {
	case "gha":
		r.gha(file, data, options)
	case "jenkins":
		r.jenkins(file, string(data), options)
	default:
		return r, errors.New("translator must be gha or jenkins")
	}
	if len(r.Diagnostics) == 0 {
		if err := r.Document.Validate(); err != nil {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{File: file, Line: 1, Column: 1, Message: err.Error()})
		}
	}
	if len(r.Diagnostics) > 0 {
		for name, spec := range r.Document.Jobs {
			spec.Paused = true
			r.Document.Jobs[name] = spec
		}
		return r, ErrIncomplete
	}
	return r, nil
}
func (r *Result) diagnostic(file string, n *yaml.Node, message string) {
	r.Diagnostics = append(r.Diagnostics, Diagnostic{File: file, Line: n.Line, Column: n.Column, Message: message})
}
func (r *Result) mapping(file string, n *yaml.Node, allowed ...string) map[string]*yaml.Node {
	out := map[string]*yaml.Node{}
	if n == nil {
		return out
	}
	if n.Kind != yaml.MappingNode {
		r.diagnostic(file, n, "expected a mapping")
		return out
	}
	for i := 0; i < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Tag != "!!str" {
			r.diagnostic(file, k, "mapping key must be a string")
			continue
		}
		if _, ok := out[k.Value]; ok {
			r.diagnostic(file, k, "duplicate field")
		}
		out[k.Value] = v
		ok := false
		for _, name := range allowed {
			if name == k.Value {
				ok = true
			}
		}
		if !ok {
			r.diagnostic(file, k, "unsupported field: "+k.Value)
		}
	}
	return out
}
func (r *Result) literal(file string, n *yaml.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind != yaml.ScalarNode || strings.Contains(n.Value, "${{") {
		r.diagnostic(file, n, "dynamic values require explicit conversion")
		return ""
	}
	return n.Value
}
func (r *Result) gha(file string, data []byte, o Options) {
	var root yaml.Node
	d := yaml.NewDecoder(bytes.NewReader(data))
	if err := d.Decode(&root); err != nil {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{File: file, Line: 1, Column: 1, Message: err.Error()})
		return
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		r.diagnostic(file, &root, "provide one workflow document")
		return
	}
	if len(root.Content) != 1 {
		return
	}
	top := r.mapping(file, root.Content[0], "name", "on", "env", "jobs")
	schedules := []string{}
	if on := top["on"]; on != nil {
		if on.Kind == yaml.MappingNode {
			triggers := r.mapping(file, on, "schedule", "workflow_dispatch")
			if dispatch := triggers["workflow_dispatch"]; dispatch != nil && len(dispatch.Content) > 0 {
				r.diagnostic(file, dispatch, "workflow dispatch inputs are unsupported")
			}
			if list := triggers["schedule"]; list != nil {
				if list.Kind != yaml.SequenceNode {
					r.diagnostic(file, list, "schedule must be a sequence")
				}
				for _, item := range list.Content {
					value := r.mapping(file, item, "cron")
					schedules = append(schedules, r.literal(file, value["cron"]))
				}
			}
		} else if on.Value != "workflow_dispatch" {
			r.diagnostic(file, on, "only schedule and workflow_dispatch triggers are supported")
		}
	}
	if len(schedules) > 1 {
		r.diagnostic(file, top["on"], "multiple schedules require separate jobs")
	}
	workflowEnv := r.environment(file, top["env"], o)
	list := top["jobs"]
	if list == nil {
		r.diagnostic(file, root.Content[0], "workflow has no jobs")
		return
	}
	if list.Kind != yaml.MappingNode {
		r.diagnostic(file, list, "jobs must be a mapping")
		return
	}
	seen := map[string]bool{}
	for i := 0; i < len(list.Content); i += 2 {
		name, node := list.Content[i].Value, list.Content[i+1]
		if seen[name] {
			r.diagnostic(file, list.Content[i], "duplicate job name")
			continue
		}
		seen[name] = true
		fields := r.mapping(file, node, "name", "runs-on", "container", "env", "steps", "timeout-minutes")
		spec := jobs.Spec{Timezone: "UTC", Container: jobs.Container{Image: o.Image}, Environment: map[string]string{}, Secrets: map[string]string{}, Command: jobs.Command{Path: "/bin/sh", WorkingDirectory: "/workspace"}}
		if len(schedules) > 0 {
			spec.Schedule = schedules[0]
		}
		for k, v := range workflowEnv.values {
			spec.Environment[k] = v
		}
		for k, v := range workflowEnv.secrets {
			spec.Secrets[k] = v
		}
		environment := r.environment(file, fields["env"], o)
		for k, v := range environment.values {
			spec.Environment[k] = v
		}
		for k, v := range environment.secrets {
			spec.Secrets[k] = v
		}
		if container := fields["container"]; container != nil {
			if container.Kind == yaml.ScalarNode {
				spec.Container.Image = r.literal(file, container)
			} else {
				definition := r.mapping(file, container, "image")
				spec.Container.Image = r.literal(file, definition["image"])
			}
		}
		if spec.Container.Image == "" {
			r.diagnostic(file, node, "provide an explicit container image mapping")
		}
		if runner := fields["runs-on"]; runner != nil {
			values := []*yaml.Node{runner}
			if runner.Kind == yaml.SequenceNode {
				values = runner.Content
			}
			for _, value := range values {
				label := r.literal(file, value)
				mapped, ok := o.Labels[label]
				if !ok {
					r.diagnostic(file, value, "map runner label explicitly: "+label)
				} else {
					spec.Runner.Labels = append(spec.Runner.Labels, mapped...)
				}
			}
		}
		if timeout := fields["timeout-minutes"]; timeout != nil {
			spec.Timeout = r.literal(file, timeout) + "m"
		}
		commands := []string{}
		steps := fields["steps"]
		checkout := false
		if steps == nil || steps.Kind != yaml.SequenceNode {
			r.diagnostic(file, node, "job must have sequential steps")
		} else {
			for _, step := range steps.Content {
				f := r.mapping(file, step, "name", "uses", "with", "run", "shell", "working-directory")
				if uses := f["uses"]; uses != nil {
					action := r.literal(file, uses)
					if !strings.HasPrefix(action, "actions/checkout@") {
						r.diagnostic(file, uses, "only actions/checkout is supported")
						continue
					}
					if checkout || len(commands) > 0 {
						r.diagnostic(file, uses, "checkout must occur once before commands")
					}
					checkout = true
					settings := r.mapping(file, f["with"], "ref", "persist-credentials", "fetch-depth")
					for k, v := range settings {
						value := r.literal(file, v)
						switch k {
						case "ref":
							if o.Project.Source.Git == nil || value != o.Project.Source.Git.Branch {
								r.diagnostic(file, v, "checkout ref must match the project branch")
							}
						case "persist-credentials":
							if value != "false" {
								r.diagnostic(file, v, "persisted checkout credentials are unsupported")
							}
						case "fetch-depth":
							if value != "0" {
								r.diagnostic(file, v, "only a full checkout is supported")
							}
						}
					}
					continue
				}
				command := r.literal(file, f["run"])
				if command == "" {
					r.diagnostic(file, step, "step must contain a literal run command")
					continue
				}
				shell := "/bin/sh"
				if sh := f["shell"]; sh != nil {
					switch sh.Value {
					case "bash":
						shell = "/bin/bash"
					case "sh":
					default:
						r.diagnostic(file, sh, "shell template is unsupported")
					}
				}
				working := "/workspace"
				if wd := f["working-directory"]; wd != nil {
					value := r.literal(file, wd)
					if strings.HasPrefix(value, "/") || strings.Contains(value, "..") {
						r.diagnostic(file, wd, "working directory must remain within the source")
					} else {
						working += "/" + value
					}
				}
				flags := " -e -c "
				if shell == "/bin/bash" {
					flags = " --noprofile --norc -eo pipefail -c "
				}
				commands = append(commands, "(cd "+shellQuote(working)+" && "+shell+flags+shellQuote(command)+")")
			}
		}
		if !checkout {
			r.diagnostic(file, node, "explicit checkout is required for the project source")
		}
		r.script(name, &spec, commands)
		r.Document.Jobs[name] = spec
	}
}

type environment struct{ values, secrets map[string]string }

var secretExpression = regexp.MustCompile(`^\$\{\{\s*secrets\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}$`)

func (r *Result) environment(file string, node *yaml.Node, o Options) environment {
	result := environment{values: map[string]string{}, secrets: map[string]string{}}
	if node == nil {
		return result
	}
	if node.Kind != yaml.MappingNode {
		r.diagnostic(file, node, "environment must be a literal mapping")
		return result
	}
	for i := 0; i < len(node.Content); i += 2 {
		name, value := node.Content[i].Value, node.Content[i+1]
		if match := secretExpression.FindStringSubmatch(value.Value); match != nil {
			ref, ok := o.Credentials[match[1]]
			if !ok {
				r.diagnostic(file, value, "map credential explicitly: "+match[1])
			} else {
				result.secrets[name] = ref
			}
		} else {
			result.values[name] = r.literal(file, value)
		}
	}
	return result
}
func (r *Result) script(name string, spec *jobs.Spec, commands []string) {
	path := ".gocicle/scripts/" + name + ".sh"
	r.Scripts[path] = "#!/bin/sh\nset -eu\n" + strings.Join(commands, "\n") + "\n"
	spec.Command.Args = []string{"/workspace/" + path}
	spec.Defaults()
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func (d Diagnostic) String() string {
	return fmt.Sprintf("%s:%d:%d: %s", d.File, d.Line, d.Column, d.Message)
}
