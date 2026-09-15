package translate

import (
	"errors"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
)

const gha = `name: update
on:
  schedule:
    - cron: '0 3 * * 1'
  workflow_dispatch:
jobs:
  update:
    runs-on: ubuntu-latest
    container: alpine:3.22
    env:
      GREETING: hello
      TOKEN: '${{ secrets.WRITE_TOKEN }}'
    steps:
      - uses: actions/checkout@v4
        with:
          persist-credentials: false
          fetch-depth: 0
      - run: echo first
      - run: echo second
        shell: bash
`

func options() Options {
	return Options{Project: jobs.Project{Name: "example", Source: jobs.Source{Git: &jobs.Git{URL: "https://example.com/repo.git", Branch: "main"}}}, Labels: map[string][]string{"ubuntu-latest": {"linux"}, "jenkins": {"linux"}}, Credentials: map[string]string{"WRITE_TOKEN": "secret-id"}}
}
func TestGHA(t *testing.T) {
	result, err := Convert("gha", "ci.yaml", []byte(gha), options())
	if err != nil {
		t.Fatalf("%v: %+v", err, result.Diagnostics)
	}
	spec := result.Document.Jobs["update"]
	if spec.Secrets["TOKEN"] != "secret-id" || spec.Schedule != "0 3 * * 1" {
		t.Fatalf("spec=%+v", spec)
	}
	script := result.Scripts[".gocicle/scripts/update.sh"]
	if strings.Index(script, "echo first") > strings.Index(script, "echo second") || !strings.Contains(script, "pipefail") {
		t.Fatalf("script=%s", script)
	}
}
func TestGHARejectsUnsupportedSemantics(t *testing.T) {
	for _, field := range []string{"strategy: {matrix: {go: [1, 2]}}", "needs: other", "services: {}", "if: true", "uses: example/reusable@main"} {
		t.Run(field, func(t *testing.T) {
			source := strings.Replace(gha, "    runs-on:", "    "+field+"\n    runs-on:", 1)
			result, err := Convert("gha", "ci.yaml", []byte(source), options())
			if !errors.Is(err, ErrIncomplete) || len(result.Diagnostics) == 0 {
				t.Fatal("unsupported behavior was omitted")
			}
			if !result.Document.Jobs["update"].Paused {
				t.Fatal("draft is enabled")
			}
			if result.Diagnostics[0].Line == 0 {
				t.Fatal("diagnostic omitted location")
			}
		})
	}
}
func TestJenkins(t *testing.T) {
	input := `pipeline { agent { docker { image 'alpine:3.22' } } triggers { cron('0 3 * * 1') } environment { NAME = 'hello' } stages { stage('one') { steps { sh 'echo one' } } stage('two') { steps { sh '''echo two
echo three''' } } } }`
	result, err := Convert("jenkins", "Jenkinsfile", []byte(input), options())
	if err != nil {
		t.Fatalf("%v: %+v", err, result.Diagnostics)
	}
	if len(result.Scripts) != 1 || result.Document.Jobs["pipeline"].Environment["NAME"] != "hello" {
		t.Fatal("pipeline did not convert")
	}
	for _, source := range []string{strings.Replace(input, "0 3 * * 1", "H 3 * * 1", 1), strings.Replace(input, "sh 'echo one'", "script { sh 'echo one' }", 1), input + " malicious()"} {
		result, err := Convert("jenkins", "Jenkinsfile", []byte(source), options())
		if !errors.Is(err, ErrIncomplete) || !result.Document.Jobs["pipeline"].Paused {
			t.Fatal("unsupported Groovy was accepted")
		}
	}
}
