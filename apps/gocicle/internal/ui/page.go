// Package ui contains browser models and screens. It does not import database or secret storage packages.
package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"gopkg.in/yaml.v3"
)

var once sync.Once

func RegisterRoutes() { once.Do(func() { app.Route("/", func() app.Composer { return &Page{} }) }) }

type project struct {
	ID, Name, OwnerID string
	Version           int64
	Source            jobs.Source
	Preferences       jobs.Preferences
}
type job struct {
	ID, Name, ProjectID, RevisionID string
	Version                         int64
	Enabled                         bool
}
type run struct {
	ID, JobID, Status, PushResult string
	CreatedAt                     time.Time
	Result                        json.RawMessage
}
type user struct {
	ID, Name               string
	Administrator, Enabled bool
	Preferences            jobs.Preferences
	Version                int64
}
type provider struct{ ID, Name, Kind string }
type Page struct {
	app.Compo
	User                                             user
	Providers                                        []provider
	Provider, Hint, Invitation, Message              string
	Projects                                         []project
	Project                                          string
	Jobs                                             []job
	Runs                                             []run
	Tab                                              string
	InspectorRunnerID, InspectionID                  string
	YAML, Preview, Mapping                           string
	Document                                         *jobs.Document
	Selected                                         job
	Editor                                           string
	Enabled                                          bool
	NewName, NewURL, NewBranch                       string
	RunID, Logs, Metrics                             string
	stream                                           app.Value
	listeners                                        []app.Func
	Settings, SettingKind, SettingID, SettingVersion string
	MemberID, MemberRole                             string
}

func (p *Page) OnMount(ctx app.Context) {
	p.Tab = "jobs"
	p.NewBranch = "main"
	p.Mapping = "{}"
	p.MemberRole = "viewer"
	p.SettingKind = "preferences"
	p.Settings = "{}"
	p.load(ctx)
}
func (p *Page) OnDismount() { p.closeStream() }
func (p *Page) closeStream() {
	if p.stream != nil && p.stream.Truthy() {
		p.stream.Call("close")
		p.stream = app.ValueOf(nil)
	}
	for _, fn := range p.listeners {
		fn.Release()
	}
	p.listeners = nil
}
func csrf() string {
	for _, entry := range strings.Split(app.Window().Get("document").Get("cookie").String(), ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if ok && name == "__Host-gocicle-csrf" {
			return value
		}
	}
	return ""
}
func request(method, path string, in, out any, version int64) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf())
	req.Header.Set("If-Match", fmt.Sprint(version))
	req.Header.Set("Idempotency-Key", fmt.Sprintf("browser-%d", time.Now().UnixNano()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to the server")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct{ Error string }
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error == "" {
			e.Error = "request failed"
		}
		return fmt.Errorf("%s", e.Error)
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out)
	}
	return nil
}
func (p *Page) load(ctx app.Context) {
	ctx.Async(func() {
		var u user
		var providers []provider
		_ = request("GET", "/api/v1/providers", nil, &providers, 0)
		err := request("GET", "/api/v1/me", nil, &u, 0)
		var projects []project
		if err == nil {
			err = request("GET", "/api/v1/projects", nil, &projects, 0)
		}
		ctx.Dispatch(func(ctx app.Context) {
			if err != nil && u.Enabled {
				p.Message = err.Error()
			}
			p.Providers = providers
			p.User = u
			p.Projects = projects
			if p.Project == "" && len(projects) > 0 {
				p.Project = projects[0].ID
			}
			if u.Enabled {
				p.refresh(ctx)
			}
		})
	})
}
func (p *Page) refresh(ctx app.Context) {
	if p.Project == "" {
		return
	}
	id := p.Project
	ctx.Async(func() {
		var allJobs []job
		var runs []run
		err := request("GET", "/api/v1/projects/"+id+"/jobs", nil, &allJobs, 0)
		if err == nil {
			err = request("GET", "/api/v1/projects/"+id+"/runs", nil, &runs, 0)
		}
		ctx.Dispatch(func(ctx app.Context) {
			if err != nil {
				p.Message = err.Error()
				return
			}
			p.Jobs = allJobs
			p.Runs = runs
		})
	})
}
func (p *Page) action(ctx app.Context, f func() error) {
	ctx.Async(func() {
		err := f()
		ctx.Dispatch(func(ctx app.Context) {
			if err != nil {
				p.Message = err.Error()
			} else {
				p.Message = "Saved."
				p.refresh(ctx)
			}
		})
	})
}
func (p *Page) Render() app.UI {
	if !p.User.Enabled {
		return p.login()
	}
	options := []app.UI{app.Option().Value("").Text("Select a project")}
	for _, project := range p.Projects {
		options = append(options, app.Option().Value(project.ID).Selected(p.Project == project.ID).Text(project.Name))
	}
	tabs := []app.UI{}
	for _, tab := range []string{"jobs", "import", "runs", "members", "settings"} {
		name := tab
		tabs = append(tabs, app.Button().Text(strings.ToUpper(name[:1])+name[1:]).OnClick(func(ctx app.Context, e app.Event) { p.Tab = name }))
	}
	content := app.Div().Body(p.jobList())
	switch p.Tab {
	case "import":
		content = app.Div().Body(p.importForm())
	case "runs":
		content = app.Div().Body(p.runList())
	case "members":
		content = app.Div().Body(p.memberForm())
	case "settings":
		content = app.Div().Body(p.settingsForm())
	}
	return app.Main().Body(app.Style().Text(styles), app.Header().Body(app.H1().Text("gocicle"), app.Span().Text(p.User.Name)), app.Nav().Body(app.Select().OnChange(func(ctx app.Context, e app.Event) { p.Project = ctx.JSSrc().Get("value").String(); p.refresh(ctx) }).Body(options...), app.Div().Body(tabs...)), app.P().Role("status").Class("message").Text(p.Message), content,
		app.Details().Body(app.Summary().Text("Create a project"), p.field("Project name", &p.NewName), p.field("Git URL", &p.NewURL), p.field("Branch", &p.NewBranch), app.Button().Text("Create project").OnClick(func(ctx app.Context, e app.Event) {
			name, url, branch := p.NewName, p.NewURL, p.NewBranch
			p.action(ctx, func() error {
				var project project
				err := request("POST", "/api/v1/projects", map[string]any{"name": name, "source": jobs.Source{Git: &jobs.Git{URL: url, Branch: branch}}}, &project, 0)
				if err == nil {
					ctx.Dispatch(func(ctx app.Context) { p.Project = project.ID; p.load(ctx) })
				}
				return err
			})
		})))
}
func (p *Page) field(label string, value *string) app.UI {
	return app.Label().Body(app.Span().Text(label), app.Input().Value(*value).OnInput(p.ValueTo(value)))
}
func (p *Page) login() app.UI {
	options := []app.UI{app.Option().Value("").Text("Select a login provider")}
	for _, provider := range p.Providers {
		if provider.Kind == "git" {
			continue
		}
		options = append(options, app.Option().Value(provider.ID).Text(provider.Name))
	}
	return app.Main().Body(app.Style().Text(styles), app.H1().Text("gocicle"), app.P().Text("Sign in to manage container jobs."), app.P().Text(p.Message), app.Select().OnChange(p.ValueTo(&p.Provider)).Body(options...), p.field("AT Protocol handle (Tangled only)", &p.Hint), p.field("Invitation (first sign-in only)", &p.Invitation), app.Button().Text("Sign in").OnClick(func(ctx app.Context, e app.Event) {
		id, hint, invite := p.Provider, p.Hint, p.Invitation
		p.action(ctx, func() error {
			var response struct{ URL string }
			if err := request("POST", "/auth/"+id+"/begin", map[string]any{"hint": hint, "invitation": invite}, &response, 0); err != nil {
				return err
			}
			ctx.Dispatch(func(ctx app.Context) { app.Window().Get("location").Set("href", response.URL) })
			return nil
		})
	}))
}
func (p *Page) jobList() app.UI {
	rows := []app.UI{}
	for _, item := range p.Jobs {
		j := item
		rows = append(rows, app.Tr().Body(app.Td().Text(j.Name), app.Td().Text(map[bool]string{true: "Enabled", false: "Disabled"}[j.Enabled]), app.Td().Body(app.Button().Text("Edit").OnClick(func(ctx app.Context, e app.Event) { p.edit(ctx, j) }), app.Button().Text("Run now").Disabled(!j.Enabled).OnClick(func(ctx app.Context, e app.Event) {
			p.action(ctx, func() error { return request("POST", "/api/v1/jobs/"+j.ID+"/runs", nil, nil, 0) })
		}))))
	}
	return app.Section().Body(app.H2().Text("Jobs"), app.Table().Body(app.TBody().Body(rows...)), app.If(p.Selected.ID != "", func() app.UI {
		return app.Div().Body(app.H3().Text(p.Selected.Name), app.Label().Text("Job YAML"), app.Textarea().Rows(22).Text(p.Editor).OnInput(p.ValueTo(&p.Editor)), app.Label().Body(app.Input().Type("checkbox").Checked(p.Enabled).OnChange(func(ctx app.Context, e app.Event) { p.Enabled = ctx.JSSrc().Get("checked").Bool() }), app.Text("Enable this validated job")), app.Button().Text("Save job").OnClick(func(ctx app.Context, e app.Event) {
			text, id, version, enabled := p.Editor, p.Selected.ID, p.Selected.Version, p.Enabled
			p.action(ctx, func() error {
				var spec jobs.Spec
				d := yaml.NewDecoder(strings.NewReader(text))
				d.KnownFields(true)
				if err := d.Decode(&spec); err != nil {
					return err
				}
				var saved job
				if err := request("PUT", "/api/v1/jobs/"+id, map[string]any{"spec": spec, "enabled": enabled}, &saved, version); err != nil {
					return err
				}
				ctx.Dispatch(func(ctx app.Context) { p.Selected = saved })
				return nil
			})
		}), app.Button().Text("Preview schedule").OnClick(func(ctx app.Context, e app.Event) {
			text := p.Editor
			p.action(ctx, func() error {
				var spec jobs.Spec
				if err := yaml.Unmarshal([]byte(text), &spec); err != nil {
					return err
				}
				var times []time.Time
				if err := request("POST", "/api/v1/schedules/preview", map[string]string{"schedule": spec.Schedule, "timezone": spec.Timezone}, &times, 0); err != nil {
					return err
				}
				ctx.Dispatch(func(ctx app.Context) { p.Preview = fmt.Sprint(times) })
				return nil
			})
		}), app.Pre().Text(p.Preview))
	}))
}
func (p *Page) edit(ctx app.Context, j job) {
	ctx.Async(func() {
		var result struct {
			Job      job
			Revision struct{ Spec jobs.Spec }
		}
		err := request("GET", "/api/v1/jobs/"+j.ID, nil, &result, 0)
		text, _ := yaml.Marshal(result.Revision.Spec)
		ctx.Dispatch(func(ctx app.Context) {
			if err != nil {
				p.Message = err.Error()
				return
			}
			p.Selected = result.Job
			p.Enabled = result.Job.Enabled
			p.Editor = string(text)
		})
	})
}
func (p *Page) importForm() app.UI {
	return app.Section().Body(app.H2().Text("Import repository jobs"), p.inspectForm(), app.P().Text("Paste .gocicle.yaml. Review the preview before importing. Imported jobs remain disabled."), app.Textarea().Rows(18).Text(p.YAML).OnInput(func(ctx app.Context, e app.Event) { p.YAML = ctx.JSSrc().Get("value").String(); p.Document = nil }), app.Button().Text("Preview import").OnClick(func(ctx app.Context, e app.Event) {
		text := p.YAML
		p.action(ctx, func() error {
			var d jobs.Document
			if err := request("POST", "/api/v1/jobs/import-preview", map[string]string{"yaml": text}, &d, 0); err != nil {
				return err
			}
			b, _ := d.YAML()
			ctx.Dispatch(func(ctx app.Context) { p.Document = &d; p.Preview = string(b) })
			return nil
		})
	}), app.Pre().Text(p.Preview), app.Label().Text("Credential mapping and conflicting job versions"), app.Textarea().Rows(5).Text(p.Mapping).OnInput(p.ValueTo(&p.Mapping)), app.Button().Text("Import disabled jobs").Disabled(p.Document == nil || p.Project == "").OnClick(func(ctx app.Context, e app.Event) {
		d, mapping, id := p.Document, p.Mapping, p.Project
		p.action(ctx, func() error {
			var values struct {
				Credentials map[string]string `json:"credentials"`
				Versions    map[string]int64  `json:"versions"`
			}
			if err := json.Unmarshal([]byte(mapping), &values); err != nil {
				return err
			}
			return request("POST", "/api/v1/projects/"+id+"/jobs/import", map[string]any{"document": d, "credentials": values.Credentials, "versions": values.Versions}, nil, 0)
		})
	}), app.A().Href("/api/v1/projects/"+p.Project+"/jobs/export").Download(".gocicle.yaml").Text("Export jobs"))
}
func (p *Page) runList() app.UI {
	rows := []app.UI{}
	for _, item := range p.Runs {
		run := item
		rows = append(rows, app.Tr().Body(app.Td().Text(run.CreatedAt.Format(time.RFC3339)), app.Td().Text(run.Status), app.Td().Text(run.PushResult), app.Td().Body(app.Button().Text("Inspect").OnClick(func(ctx app.Context, e app.Event) { p.watch(ctx, run.ID) }), app.Button().Text("Cancel").Disabled(run.Status != "running" && run.Status != "queued").OnClick(func(ctx app.Context, e app.Event) {
			p.action(ctx, func() error { return request("POST", "/api/v1/runs/"+run.ID+"/cancel", nil, nil, 0) })
		}))))
	}
	return app.Section().Body(app.H2().Text("Run history"), app.Button().Text("Refresh").OnClick(func(ctx app.Context, e app.Event) { p.refresh(ctx) }), app.Table().Body(app.TBody().Body(rows...)), app.H3().Text(p.RunID), app.Pre().Class("logs").Text(p.Logs), app.H3().Text("Metrics"), app.Pre().Text(p.Metrics))
}
func (p *Page) watch(ctx app.Context, id string) {
	p.closeStream()
	p.RunID = id
	p.Logs = ""
	p.Metrics = ""
	p.stream = app.Window().Get("EventSource").New("/api/v1/runs/" + id + "/events")
	for _, event := range []string{"log", "metrics", "complete"} {
		kind := event
		callback := app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			data := args[0].Get("data").String()
			ctx.Dispatch(func(ctx app.Context) {
				switch kind {
				case "log":
					var line string
					if json.Unmarshal([]byte(data), &line) == nil {
						p.Logs += line
						if len(p.Logs) > 1<<20 {
							p.Logs = p.Logs[len(p.Logs)-(1<<20):]
						}
					}
				case "metrics":
					p.Metrics = data
				case "complete":
					p.Message = "Run completed."
					p.refresh(ctx)
				}
			})
			return nil
		})
		p.listeners = append(p.listeners, callback)
		p.stream.Call("addEventListener", kind, callback)
	}
}
func (p *Page) memberForm() app.UI {
	return app.Section().Body(app.H2().Text("Project members"), p.field("User ID", &p.MemberID), app.Select().OnChange(p.ValueTo(&p.MemberRole)).Body(app.Option().Value("viewer").Text("Viewer"), app.Option().Value("operator").Text("Operator"), app.Option().Value("owner").Text("Owner"), app.Option().Value("").Text("Remove access")), app.Button().Text("Save membership").OnClick(func(ctx app.Context, e app.Event) {
		id, member, role := p.Project, p.MemberID, p.MemberRole
		p.action(ctx, func() error {
			return request("PUT", "/api/v1/projects/"+id+"/memberships/"+member, map[string]string{"role": role}, nil, 0)
		})
	}))
}
func (p *Page) settingsForm() app.UI {
	choices := []app.UI{}
	for _, kind := range []string{"preferences", "project-preferences", "connections", "ssh-keys", "notifications", "secrets"} {
		choices = append(choices, app.Option().Value(kind).Selected(p.SettingKind == kind).Text(kind))
	}
	if p.User.Administrator {
		for _, kind := range []string{"admin/runners", "admin/users", "admin/providers", "admin/enrollments"} {
			choices = append(choices, app.Option().Value(kind).Selected(p.SettingKind == kind).Text(kind))
		}
	}
	return app.Section().Body(app.H2().Text("Settings"), p.linkIdentityForm(), app.Select().OnChange(p.ValueTo(&p.SettingKind)).Body(choices...), p.field("Record ID (empty for a new record)", &p.SettingID), p.field("Current version", &p.SettingVersion), app.Textarea().Rows(16).Text(p.Settings).OnInput(p.ValueTo(&p.Settings)), app.Button().Text("Load settings").OnClick(func(ctx app.Context, e app.Event) {
		kind := p.SettingKind
		p.action(ctx, func() error {
			var result any
			path := "/api/v1/" + kind
			if kind == "preferences" {
				path = "/api/v1/me"
			}
			if kind == "project-preferences" {
				path = "/api/v1/projects"
			}
			if err := request("GET", path, nil, &result, 0); err != nil {
				return err
			}
			b, _ := json.MarshalIndent(result, "", "  ")
			ctx.Dispatch(func(ctx app.Context) { p.Settings = string(b) })
			return nil
		})
	}), app.Button().Text("Save settings").OnClick(func(ctx app.Context, e app.Event) {
		kind, id, version, text, project := p.SettingKind, p.SettingID, p.SettingVersion, p.Settings, p.Project
		p.action(ctx, func() error {
			var value any
			if err := json.Unmarshal([]byte(text), &value); err != nil {
				return err
			}
			method, path := "POST", "/api/v1/"+kind
			if id != "" {
				method = "PUT"
				path += "/" + id
			}
			if kind == "preferences" {
				method = "PUT"
			}
			if kind == "project-preferences" {
				method = "PUT"
				path = "/api/v1/projects/" + project + "/preferences"
			}
			var rev int64
			_, _ = fmt.Sscan(version, &rev)
			var result any
			if err := request(method, path, value, &result, rev); err != nil {
				return err
			}
			b, _ := json.MarshalIndent(result, "", "  ")
			ctx.Dispatch(func(ctx app.Context) { p.Settings = string(b) })
			return nil
		})
	}))
}

const styles = `body{margin:0;background:#f5f7fa;color:#192735;font:16px system-ui}main{max-width:1080px;margin:32px auto;padding:24px}header,nav{display:flex;align-items:center;justify-content:space-between;gap:16px}h1{letter-spacing:-1px}section,details{background:white;padding:24px;margin-top:20px;border:1px solid #dce3ea;border-radius:8px}button,select,input,textarea{font:inherit;padding:9px;border:1px solid #becbd8;border-radius:5px}button{cursor:pointer;margin:5px;background:#164d76;color:white}button:disabled{opacity:.5}label{display:block;margin:12px 0}label span{display:block;margin-bottom:4px}input:not([type=checkbox]),textarea{width:100%;box-sizing:border-box}textarea,pre{font:14px ui-monospace,monospace}pre{white-space:pre-wrap;overflow-wrap:anywhere;max-height:480px;overflow:auto}.logs{background:#13212c;color:#dceaf5;padding:20px}table{width:100%;border-collapse:collapse}td{padding:8px;border-bottom:1px solid #e0e6ec}.message{min-height:24px;color:#81410e}@media(max-width:700px){main{margin:0;padding:12px}nav{display:block}td{font-size:13px}}`

func (p *Page) inspectForm() app.UI {
	return app.Div().Body(p.field("Runner ID for repository inspection", &p.InspectorRunnerID), app.Button().Text("Read repository YAML").Disabled(p.Project == "" || p.InspectorRunnerID == "").OnClick(func(ctx app.Context, e app.Event) {
		project, runner := p.Project, p.InspectorRunnerID
		p.action(ctx, func() error {
			var inspection struct {
				ID, Status, Error string
				Document          jobs.Document
			}
			if err := request("POST", "/api/v1/projects/"+project+"/inspections", map[string]string{"runnerID": runner}, &inspection, 0); err != nil {
				return err
			}
			ctx.Dispatch(func(ctx app.Context) {
				p.InspectionID = inspection.ID
				p.Message = "The runner is reading the repository."
			})
			deadline := time.Now().Add(5 * time.Minute)
			for time.Now().Before(deadline) {
				time.Sleep(2 * time.Second)
				if err := request("GET", "/api/v1/inspections/"+inspection.ID, nil, &inspection, 0); err != nil {
					return err
				}
				switch inspection.Status {
				case "complete":
					b, err := inspection.Document.YAML()
					if err != nil {
						return err
					}
					ctx.Dispatch(func(ctx app.Context) { p.YAML = string(b); p.Preview = string(b); p.Document = &inspection.Document })
					return nil
				case "failure", "lost":
					return fmt.Errorf("%s", inspection.Error)
				}
			}
			return fmt.Errorf("repository inspection timed out")
		})
	}))
}

func (p *Page) linkIdentityForm() app.UI {
	options := []app.UI{app.Option().Value("").Text("Select an additional identity provider")}
	for _, provider := range p.Providers {
		if provider.Kind != "git" {
			options = append(options, app.Option().Value(provider.ID).Text(provider.Name))
		}
	}
	return app.Details().Body(app.Summary().Text("Link another login identity"), app.Select().OnChange(p.ValueTo(&p.Provider)).Body(options...), p.field("AT Protocol handle (Tangled only)", &p.Hint), app.Button().Text("Link identity").OnClick(func(ctx app.Context, e app.Event) {
		id, hint := p.Provider, p.Hint
		p.action(ctx, func() error {
			var reply struct{ URL string }
			if err := request("POST", "/auth/"+id+"/begin", map[string]any{"link": true, "hint": hint}, &reply, 0); err != nil {
				return err
			}
			ctx.Dispatch(func(ctx app.Context) { app.Window().Get("location").Set("href", reply.URL) })
			return nil
		})
	}))
}
