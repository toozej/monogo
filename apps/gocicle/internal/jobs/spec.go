// Package jobs defines portable job files. It does not import storage or credentials.
package jobs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

const APIVersion = "gocicle/v1"
const MaxDocument = 1 << 20

type Document struct {
	APIVersion string          `json:"apiVersion" yaml:"apiVersion"`
	Project    Project         `json:"project" yaml:"project"`
	Jobs       map[string]Spec `json:"jobs" yaml:"jobs"`
}
type Project struct {
	Name   string `json:"name" yaml:"name"`
	Source Source `json:"source" yaml:"source"`
}
type Source struct {
	Git   *Git   `json:"git,omitempty" yaml:"git,omitempty"`
	Local *Local `json:"local,omitempty" yaml:"local,omitempty"`
}
type Git struct {
	URL        string `json:"url" yaml:"url"`
	Branch     string `json:"branch" yaml:"branch"`
	Credential string `json:"credential,omitempty" yaml:"credential,omitempty"`
}
type Local struct {
	RunnerID      string `json:"runnerID" yaml:"runnerID"`
	HostPath      string `json:"hostPath" yaml:"hostPath"`
	ContainerPath string `json:"containerPath" yaml:"containerPath"`
	ReadOnly      bool   `json:"readOnly" yaml:"readOnly"`
}
type Spec struct {
	Schedule    string            `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	Timezone    string            `json:"timezone" yaml:"timezone"`
	Paused      bool              `json:"paused" yaml:"paused"`
	Runner      Selector          `json:"runner" yaml:"runner"`
	Container   Container         `json:"container" yaml:"container"`
	Command     Command           `json:"command" yaml:"command"`
	Environment map[string]string `json:"environment,omitempty" yaml:"environment,omitempty"`
	Secrets     map[string]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Timeout     string            `json:"timeout" yaml:"timeout"`
	PushChanges bool              `json:"pushChanges" yaml:"pushChanges"`
	Limits      Limits            `json:"limits" yaml:"limits"`
}
type Selector struct {
	Labels []string `json:"labels" yaml:"labels"`
}
type Container struct {
	Image        string `json:"image,omitempty" yaml:"image,omitempty"`
	Dockerfile   string `json:"dockerfile,omitempty" yaml:"dockerfile,omitempty"`
	BuildContext string `json:"buildContext,omitempty" yaml:"buildContext,omitempty"`
}
type Command struct {
	Path             string   `json:"path" yaml:"path"`
	Args             []string `json:"args" yaml:"args"`
	WorkingDirectory string   `json:"workingDirectory" yaml:"workingDirectory"`
}
type Limits struct {
	CPUs   float64 `json:"cpus" yaml:"cpus"`
	Memory int64   `json:"memory" yaml:"memory"`
	PIDs   int64   `json:"pids" yaml:"pids"`
}

// Pointer fields distinguish inheritance from false and an empty list.
type Preferences struct {
	CommitName     *string   `json:"commitName,omitempty" yaml:"commitName,omitempty"`
	CommitEmail    *string   `json:"commitEmail,omitempty" yaml:"commitEmail,omitempty"`
	Avatar         *string   `json:"avatar,omitempty" yaml:"avatar,omitempty"`
	NotifySuccess  *bool     `json:"notifySuccess,omitempty" yaml:"notifySuccess,omitempty"`
	NotifyFailure  *bool     `json:"notifyFailure,omitempty" yaml:"notifyFailure,omitempty"`
	NotifyRecovery *bool     `json:"notifyRecovery,omitempty" yaml:"notifyRecovery,omitempty"`
	Notifications  *[]string `json:"notifications,omitempty" yaml:"notifications,omitempty"`
}

func Resolve(owner, project Preferences) Preferences {
	name, email, yes, no := "gocicle", "gocicle@localhost", true, false
	out := Preferences{CommitName: &name, CommitEmail: &email, NotifyFailure: &yes, NotifyRecovery: &yes, NotifySuccess: &no}
	for _, p := range []Preferences{owner, project} {
		if p.CommitName != nil {
			out.CommitName = p.CommitName
		}
		if p.CommitEmail != nil {
			out.CommitEmail = p.CommitEmail
		}
		if p.Avatar != nil {
			out.Avatar = p.Avatar
		}
		if p.NotifySuccess != nil {
			out.NotifySuccess = p.NotifySuccess
		}
		if p.NotifyFailure != nil {
			out.NotifyFailure = p.NotifyFailure
		}
		if p.NotifyRecovery != nil {
			out.NotifyRecovery = p.NotifyRecovery
		}
		if p.Notifications != nil {
			out.Notifications = p.Notifications
		}
	}
	return out
}

var nameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)
var envRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func ValidName(name string) bool { return nameRE.MatchString(name) }

func Parse(data []byte) (Document, error) {
	var d Document
	if len(data) > MaxDocument {
		return d, errors.New("job file exceeds 1 MiB")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&d); err != nil {
		return d, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return d, errors.New("provide exactly one YAML document")
	}
	return d, d.Validate()
}
func (d *Document) Validate() error {
	if d.APIVersion != APIVersion {
		return fmt.Errorf("apiVersion must be %s", APIVersion)
	}
	if !nameRE.MatchString(d.Project.Name) {
		return errors.New("project name is invalid")
	}
	if err := d.Project.Source.Validate(); err != nil {
		return err
	}
	if len(d.Jobs) == 0 {
		return errors.New("provide at least one job")
	}
	for _, name := range d.Names() {
		if !nameRE.MatchString(name) {
			return fmt.Errorf("job name %q is invalid", name)
		}
		spec := d.Jobs[name]
		spec.Defaults()
		if err := spec.Validate(d.Project.Source); err != nil {
			return fmt.Errorf("job %s: %w", name, err)
		}
		d.Jobs[name] = spec
	}
	return nil
}
func (d Document) Names() []string {
	names := make([]string, 0, len(d.Jobs))
	for k := range d.Jobs {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
func (d Document) YAML() ([]byte, error) { return yaml.Marshal(d) }
func (s Source) Validate() error {
	if (s.Git == nil) == (s.Local == nil) {
		return errors.New("provide exactly one source: git or local")
	}
	if g := s.Git; g != nil {
		u, err := url.Parse(g.URL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "ssh") || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("git URL must use HTTPS or SSH without query parameters")
		}
		if u.User != nil {
			if _, ok := u.User.Password(); ok || u.Scheme != "ssh" {
				return errors.New("git URL must not contain credentials")
			}
		}
		if g.Branch == "" || strings.HasPrefix(g.Branch, "-") || strings.ContainsAny(g.Branch, " ~^:?*[\\\x00\n\r") || strings.Contains(g.Branch, "..") || strings.Contains(g.Branch, "@{") || strings.HasSuffix(g.Branch, "/") || strings.HasSuffix(g.Branch, ".") || strings.HasSuffix(g.Branch, ".lock") {
			return errors.New("git branch is invalid")
		}
	}
	if l := s.Local; l != nil {
		if l.RunnerID == "" || !absolute(l.HostPath) || !absolute(l.ContainerPath) || l.ContainerPath == "/" {
			return errors.New("local source needs a runner ID and clean absolute paths")
		}
	}
	return nil
}
func (s *Spec) Defaults() {
	if s.Timezone == "" {
		s.Timezone = "UTC"
	}
	if s.Timeout == "" {
		s.Timeout = "30m"
	}
	if s.Command.WorkingDirectory == "" {
		s.Command.WorkingDirectory = "/workspace"
	}
	if s.Limits.CPUs == 0 {
		s.Limits.CPUs = 2
	}
	if s.Limits.Memory == 0 {
		s.Limits.Memory = 2 << 30
	}
	if s.Limits.PIDs == 0 {
		s.Limits.PIDs = 256
	}
	if s.Container.Dockerfile != "" && s.Container.BuildContext == "" {
		s.Container.BuildContext = "."
	}
}
func (s Spec) Validate(source Source) error {
	if err := source.Validate(); err != nil {
		return err
	}
	if (s.Container.Image == "") == (s.Container.Dockerfile == "") {
		return errors.New("provide exactly one image or Dockerfile")
	}
	if s.Container.Dockerfile != "" && (!relative(s.Container.Dockerfile) || !relative(s.Container.BuildContext)) {
		return errors.New("the Dockerfile and build context must remain within the source")
	}
	if s.Container.Image != "" && (strings.HasPrefix(s.Container.Image, "-") || strings.ContainsAny(s.Container.Image, " \t\n\x00") || s.Container.BuildContext != "") {
		return errors.New("image reference or build context is invalid")
	}
	if !absolute(s.Command.Path) || !absolute(s.Command.WorkingDirectory) {
		return errors.New("command path and working directory must be clean absolute paths")
	}
	for _, arg := range s.Command.Args {
		if strings.ContainsRune(arg, 0) {
			return errors.New("command argument contains NUL")
		}
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil {
		return errors.New("timezone is invalid")
	}
	if s.Schedule != "" {
		if _, err := Schedule(s.Schedule, s.Timezone); err != nil {
			return err
		}
	}
	d, err := time.ParseDuration(s.Timeout)
	if err != nil || d <= 0 || d > 24*time.Hour {
		return errors.New("timeout must be between zero and 24 hours")
	}
	if math.IsNaN(s.Limits.CPUs) || math.IsInf(s.Limits.CPUs, 0) || s.Limits.CPUs <= 0 || s.Limits.CPUs > 128 || s.Limits.Memory < 16<<20 || s.Limits.Memory > 1<<40 || s.Limits.PIDs < 1 || s.Limits.PIDs > 65536 {
		return errors.New("resource limits are invalid")
	}
	if source.Local != nil && s.PushChanges {
		return errors.New("local sources cannot push changes")
	}
	for k, v := range s.Environment {
		if !envRE.MatchString(k) || strings.ContainsRune(v, 0) {
			return errors.New("environment entry is invalid")
		}
		if _, ok := s.Secrets[k]; ok {
			return errors.New("environment and secret names overlap")
		}
	}
	for k, v := range s.Secrets {
		if !envRE.MatchString(k) || v == "" {
			return errors.New("secret reference is invalid")
		}
	}
	return nil
}
func Schedule(expression, timezone string) (cron.Schedule, error) {
	if len(strings.Fields(expression)) != 5 || strings.Contains(expression, "@") {
		return nil, errors.New("schedule requires five cron fields")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, err
	}
	return parser.Parse("CRON_TZ=" + timezone + " " + expression)
}
func (s Spec) References(source Source) []string {
	refs := map[string]bool{}
	for _, v := range s.Secrets {
		refs[v] = true
	}
	if source.Git != nil && source.Git.Credential != "" {
		refs[source.Git.Credential] = true
	}
	result := make([]string, 0, len(refs))
	for v := range refs {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}
func absolute(p string) bool {
	return strings.HasPrefix(p, "/") && path.Clean(p) == p && !strings.ContainsAny(p, "\x00\n\r")
}
func relative(p string) bool {
	return p != "" && !path.IsAbs(p) && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\\\x00\n\r")
}
