package jobs

import (
	"strings"
	"testing"
	"time"
)

const example = `apiVersion: gocicle/v1
project:
  name: example
  source:
    git:
      url: https://github.com/example/project.git
      branch: main
jobs:
  update:
    schedule: '0 3 * * 1'
    container:
      image: golang:1.27.1-trixie
    command:
      path: /bin/sh
      args: [-c, 'echo hello']
    pushChanges: true
`

func TestRoundTripAndDefaults(t *testing.T) {
	d, err := Parse([]byte(example))
	if err != nil {
		t.Fatal(err)
	}
	s := d.Jobs["update"]
	if s.Timeout != "30m" || s.Timezone != "UTC" || s.Limits.Memory != 2<<30 || s.Limits.CPUs != 2 {
		t.Fatalf("defaults: %+v", s)
	}
	b, err := d.YAML()
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := again.YAML()
	if string(b) != string(b2) {
		t.Fatal("YAML round trip changed the document")
	}
}
func TestRejectInvalidDocuments(t *testing.T) {
	tests := map[string]string{
		"unknown":                strings.Replace(example, "pushChanges: true", "pushChanges: true\n    runsOn: host", 1),
		"duplicate":              example + "  update:\n    paused: true\n",
		"documents":              example + "---\napiVersion: gocicle/v1\n",
		"credential URL":         strings.Replace(example, "https://github.com", "https://secret@github.com", 1),
		"shell injection branch": strings.Replace(example, "branch: main", "branch: '-main'", 1),
		"both sources":           strings.Replace(example, "  source:", "  source:\n    local: {runnerID: host, hostPath: /tmp, containerPath: /workspace}", 1),
		"path":                   strings.Replace(example, "path: /bin/sh", "path: /bin/../bin/sh", 1),
		"six fields":             strings.Replace(example, "0 3 * * 1", "0 0 3 * * 1", 1),
		"alias":                  strings.Replace(example, "image: golang:1.27.1-trixie", "image: golang:1.27.1-trixie\n      dockerfile: ../Dockerfile", 1),
	}
	for name, document := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(document)); err == nil {
				t.Fatal("invalid document was accepted")
			}
		})
	}
}
func TestPreferencesDistinguishFalseAndEmpty(t *testing.T) {
	yes, no := true, false
	inherited := []string{"destination"}
	empty := []string{}
	owner := Preferences{NotifySuccess: &yes, Notifications: &inherited}
	p := Resolve(owner, Preferences{NotifySuccess: &no, Notifications: &empty})
	if *p.NotifySuccess || len(*p.Notifications) != 0 {
		t.Fatal("explicit disabled values inherited")
	}
	p = Resolve(owner, Preferences{})
	if !*p.NotifySuccess || len(*p.Notifications) != 1 {
		t.Fatal("omitted values did not inherit")
	}
}
func TestCronTimezone(t *testing.T) {
	schedule, err := Schedule("0 9 * * *", "America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	next := schedule.Next(time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC))
	if next.UTC().Hour() != 16 {
		t.Fatalf("next=%s", next)
	}
}
func TestLocalPushAndBuildTraversal(t *testing.T) {
	d, err := Parse([]byte(example))
	if err != nil {
		t.Fatal(err)
	}
	s := d.Jobs["update"]
	source := Source{Local: &Local{RunnerID: "host", HostPath: "/srv/jobs", ContainerPath: "/workspace"}}
	if s.Validate(source) == nil {
		t.Fatal("local push accepted")
	}
	s.PushChanges = false
	s.Container = Container{Dockerfile: "../secret", BuildContext: "."}
	if s.Validate(source) == nil {
		t.Fatal("build traversal accepted")
	}
}
