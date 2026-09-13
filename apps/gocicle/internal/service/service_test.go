package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/protocol"
	"github.com/toozej/monogo/apps/gocicle/internal/providers"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"github.com/toozej/monogo/apps/gocicle/internal/testutil"
)

func fixture(t *testing.T) (*Service, Actor, storage.Project, storage.Job) {
	t.Helper()
	db := testutil.Database(t)
	s := New(db, security.Keyring{Active: "test", Keys: map[string]string{"test": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))}})
	owner := storage.User{ID: storage.ID(), Name: "Owner", Enabled: true, Administrator: true, Version: 1, Preferences: storage.Encode(jobs.Preferences{})}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	a := Actor{User: owner, Scopes: []string{"read", "write", "admin"}}
	source := jobs.Source{Git: &jobs.Git{URL: "https://example.com/project.git", Branch: "main"}}
	project, err := s.CreateProject(a, storage.Project{Name: "project", Source: storage.Encode(source)})
	if err != nil {
		t.Fatal(err)
	}
	spec := jobs.Spec{Schedule: "* * * * *", Container: jobs.Container{Image: "alpine:3.22"}, Command: jobs.Command{Path: "/bin/true"}, Runner: jobs.Selector{Labels: []string{"linux"}}}
	spec.Defaults()
	document := jobs.Document{APIVersion: jobs.APIVersion, Project: jobs.Project{Name: project.Name, Source: source}, Jobs: map[string]jobs.Spec{"test": spec}}
	imported, err := s.Import(a, project.ID, "first-import", Import{Document: document})
	if err != nil {
		t.Fatal(err)
	}
	if imported[0].Enabled {
		t.Fatal("import enabled a job")
	}
	job, err := s.UpdateJob(a, imported[0].ID, imported[0].Version, spec, true)
	if err != nil {
		t.Fatal(err)
	}
	return s, a, project, job
}
func runnerFixture(t *testing.T, s *Service, a Actor, name string) storage.Runner {
	t.Helper()
	token, err := s.EnrollToken(a, name, []string{"linux"}, []string{"/srv/jobs"})
	if err != nil {
		t.Fatal(err)
	}
	r, credential, err := s.Enroll(token)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Enroll(token); !errors.Is(err, ErrForbidden) {
		t.Fatal("enrollment token was reused")
	}
	if r.TokenHash == credential {
		t.Fatal("runner credential was stored in plaintext")
	}
	if err := s.RunnerGrant(a, r.ID, a.User.ID, true); err != nil {
		t.Fatal(err)
	}
	return r
}
func TestMigrationsAreExplicitAndRepeatable(t *testing.T) {
	s, _, _, _ := fixture(t)
	if err := storage.Migrate(context.Background(), s.DB); err != nil {
		t.Fatal(err)
	}
	if err := storage.Check(s.DB); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := s.DB.Table("schema_migrations").Count(&n).Error; err != nil || n != 1 {
		t.Fatal("migration history is invalid")
	}
	var revision storage.Revision
	if err := s.DB.First(&revision).Error; err != nil {
		t.Fatal(err)
	}
	if s.DB.Model(&revision).Update("spec", storage.Encode(map[string]string{"changed": "yes"})).Error == nil {
		t.Fatal("revision was mutable")
	}
}
func TestImportIdempotencyConflictAndMapping(t *testing.T) {
	s, a, p, j := fixture(t)
	d, err := s.Export(a, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import(a, p.ID, "conflict", Import{Document: d}); !errors.Is(err, ErrConflict) {
		t.Fatalf("missing conflict resolution: %v", err)
	}
	input := Import{Document: d, Versions: map[string]int64{"test": j.Version}}
	first, err := s.Import(a, p.ID, "retry-import", input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Import(a, p.ID, "retry-import", input)
	if err != nil || first[0].RevisionID != second[0].RevisionID {
		t.Fatalf("import retry: %v", err)
	}
	secret, err := s.CreateSecret(a, "token", "environment", raw(protocol.Credential{Value: "private-value"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Grant(a, secret.ID, p.ID, true); err != nil {
		t.Fatal(err)
	}
	spec := d.Jobs["test"]
	spec.Secrets = map[string]string{"TOKEN": "source-name"}
	d.Jobs["test"] = spec
	mapped := Import{Document: d, Versions: map[string]int64{"test": first[0].Version}, Credentials: map[string]string{"source-name": secret.ID}}
	first, err = s.Import(a, p.ID, "mapped", mapped)
	if err != nil {
		t.Fatal(err)
	}
	second, err = s.Import(a, p.ID, "mapped", mapped)
	if err != nil || first[0].RevisionID != second[0].RevisionID {
		t.Fatalf("mapped retry: %v", err)
	}
	var revision storage.Revision
	if err := s.DB.First(&revision, "id=?", first[0].RevisionID).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(revision.Spec), "private-value") {
		t.Fatal("secret appeared in revision")
	}
}
func TestRolesScopesAndPreferences(t *testing.T) {
	s, a, p, j := fixture(t)
	viewer := storage.User{ID: storage.ID(), Name: "viewer", Enabled: true, Version: 1, Preferences: storage.Encode(jobs.Preferences{})}
	if err := s.DB.Create(&viewer).Error; err != nil {
		t.Fatal(err)
	}
	v := Actor{User: viewer, Scopes: []string{"read", "write"}}
	if err := s.Authorize(v, p.ID, "viewer"); !errors.Is(err, ErrForbidden) {
		t.Fatal("unrelated user has access")
	}
	if err := s.Membership(a, p.ID, viewer.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	if err := s.Authorize(v, p.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(v, j.ID, "viewer-run"); !errors.Is(err, ErrForbidden) {
		t.Fatal("viewer started a run")
	}
	if err := s.Membership(a, p.ID, viewer.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	if err := s.Authorize(v, p.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	if err := s.Preferences(v, p.ID, jobs.Preferences{}, 2); !errors.Is(err, ErrForbidden) {
		t.Fatal("operator changed preferences")
	}
	token, err := s.CreateToken(a, []string{"read"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := s.Authenticate(context.Background(), token, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Authorize(reader, p.ID, "operator"); !errors.Is(err, ErrForbidden) {
		t.Fatal("read-only admin token obtained write access")
	}
	ownerName, projectName := "owner commit", "project commit"
	if err := s.Preferences(a, "", jobs.Preferences{CommitName: &ownerName}, 1); err != nil {
		t.Fatal(err)
	}
	var latest storage.Project
	if err := s.DB.First(&latest, "id=?", p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Preferences(a, p.ID, jobs.Preferences{CommitName: &projectName}, latest.Version); err != nil {
		t.Fatal(err)
	}
	run, err := s.Start(a, j.ID, "prefs")
	if err != nil {
		t.Fatal(err)
	}
	prefs, err := Decode[jobs.Preferences](run.Preferences)
	if err != nil || *prefs.CommitName != projectName {
		t.Fatal("project preference did not take precedence")
	}
}
func TestConcurrentSchedulesLeasesAndReports(t *testing.T) {
	s, a, p, j := fixture(t)
	r1 := runnerFixture(t, s, a, "one")
	r2 := runnerFixture(t, s, a, "two")
	now := time.Now().Truncate(time.Minute)
	if err := s.DB.Exec("UPDATE schedules SET next_at=?", now).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Tick(context.Background(), now) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var runs []storage.Run
	if err := s.DB.Where("project_id=?", p.ID).Find(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status != "queued" {
		t.Fatalf("schedule produced %d runs: %+v", len(runs), runs)
	}
	assignments := make(chan *protocol.Assignment, 2)
	errs = make(chan error, 2)
	for _, r := range []storage.Runner{r1, r2} {
		wg.Add(1)
		go func(r storage.Runner) {
			defer wg.Done()
			assignment, err := s.Poll(r)
			assignments <- assignment
			errs <- err
		}(r)
	}
	wg.Wait()
	close(assignments)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var assignment *protocol.Assignment
	count := 0
	for value := range assignments {
		if value != nil {
			assignment = value
			count++
		}
	}
	if count != 1 {
		t.Fatalf("assignment count=%d", count)
	}
	var stored storage.Run
	if err := s.DB.First(&stored, "id=?", assignment.RunID).Error; err != nil {
		t.Fatal(err)
	}
	r := r1
	if *stored.RunnerID == r2.ID {
		r = r2
	}
	event := protocol.Event{Sequence: 1, Kind: "log", Data: raw("hello\n")}
	if err := s.Report(r, stored.ID, assignment.LeaseToken, event); err != nil {
		t.Fatal(err)
	}
	if err := s.Report(r, stored.ID, assignment.LeaseToken, event); err != nil {
		t.Fatal(err)
	}
	event.Data = raw("changed")
	if err := s.Report(r, stored.ID, assignment.LeaseToken, event); !errors.Is(err, ErrConflict) {
		t.Fatal("conflicting event was accepted")
	}
	event.Sequence = 3
	if err := s.Report(r, stored.ID, assignment.LeaseToken, event); !errors.Is(err, ErrConflict) {
		t.Fatal("event gap was accepted")
	}
	if _, _, err := s.Heartbeat(r, stored.ID, "wrong-token"); !errors.Is(err, ErrForbidden) {
		t.Fatal("invalid lease was accepted")
	}
	if err := s.Cancel(a, stored.ID); err != nil {
		t.Fatal(err)
	}
	_, cancel, err := s.Heartbeat(r, stored.ID, assignment.LeaseToken)
	if err != nil || !cancel {
		t.Fatal("cancellation did not reach the runner")
	}
	complete := protocol.Event{Sequence: 2, Kind: "complete", Data: raw(protocol.Completion{Status: "success", ExitCode: 0})}
	if err := s.Report(r, stored.ID, assignment.LeaseToken, complete); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.First(&stored, "id=?", stored.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "cancelled" {
		t.Fatal("completion ignored cancellation")
	}
	next, err := s.Start(a, j.ID, "next")
	if err != nil {
		t.Fatal(err)
	}
	assignment, err = s.Poll(r)
	if err != nil || assignment == nil {
		t.Fatalf("next assignment: %v", err)
	}
	if err := s.DB.Exec("UPDATE runner_leases SET expires_at=? WHERE run_id=?", time.Now().Add(-time.Minute), next.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Tick(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	var lost storage.Run
	if err := s.DB.First(&lost, "id=?", next.ID).Error; err != nil {
		t.Fatal(err)
	}
	if lost.Status != "lost" {
		t.Fatalf("status=%s", lost.Status)
	}
	assignment, err = s.Poll(r)
	if err != nil || assignment != nil {
		t.Fatal("lost run was retried")
	}
}
func TestRevokedGrantsAndRedaction(t *testing.T) {
	s, a, p, j := fixture(t)
	r := runnerFixture(t, s, a, "runner")
	secret, err := s.CreateSecret(a, "token", "environment", raw(protocol.Credential{Value: "secret-value"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Grant(a, secret.ID, p.ID, true); err != nil {
		t.Fatal(err)
	}
	var revision storage.Revision
	if err := s.DB.First(&revision, "id=?", j.RevisionID).Error; err != nil {
		t.Fatal(err)
	}
	spec, err := Decode[jobs.Spec](revision.Spec)
	if err != nil {
		t.Fatal(err)
	}
	spec.Secrets = map[string]string{"TOKEN": secret.ID}
	j, err = s.UpdateJob(a, j.ID, j.Version, spec, true)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.Start(a, j.ID, "redaction")
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := s.Poll(r)
	if err != nil || assignment == nil {
		t.Fatal(err)
	}
	event := protocol.Event{Sequence: 1, Kind: "log", Data: raw("token=secret-value")}
	if err := s.Report(r, run.ID, assignment.LeaseToken, event); err != nil {
		t.Fatal(err)
	}
	var chunk struct{ Content string }
	if err := s.DB.Table("log_chunks").Where("run_id=?", run.ID).Take(&chunk).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(chunk.Content, "secret-value") {
		t.Fatal("secret was retained in logs")
	}
	if err := s.Grant(a, secret.ID, p.ID, false); err != nil {
		t.Fatal(err)
	}
	_, cancel, err := s.Heartbeat(r, run.ID, assignment.LeaseToken)
	if err != nil || !cancel {
		t.Fatal("revoked grant did not stop work")
	}
	if _, err := s.Start(a, j.ID, "revoked"); err == nil {
		t.Fatal("revoked credential was reused")
	}
	exported, err := s.ExportConfiguration(a)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(exported)
	if strings.Contains(string(data), "secret-value") || strings.Contains(string(data), "encrypted") {
		t.Fatal("configuration export exposed secrets")
	}
}

func raw(value any) json.RawMessage { return json.RawMessage(storage.Encode(value)) }

func TestNotificationOutboxRetriesWithoutChangingResult(t *testing.T) {
	s, a, p, j := fixture(t)
	secret, err := s.CreateSecret(a, "notify", "notification", raw(map[string]string{"token": "secret"}))
	if err != nil {
		t.Fatal(err)
	}
	destination, err := s.SaveResource(a, "notifications", storage.Resource{Name: "alerts", SecretID: secret.ID, Config: storage.Encode(map[string]string{"type": "gotify", "endpoint": "https://notify.example"})})
	if err != nil {
		t.Fatal(err)
	}
	var latest storage.Project
	if err := s.DB.First(&latest, "id=?", p.ID).Error; err != nil {
		t.Fatal(err)
	}
	ids := []string{destination.ID}
	if err := s.Preferences(a, p.ID, jobs.Preferences{Notifications: &ids}, latest.Version); err != nil {
		t.Fatal(err)
	}
	run, err := s.Start(a, j.ID, "notify-run")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		return s.With(tx).finish(&run, protocol.Completion{Status: "failure", ExitCode: 1})
	}); err != nil {
		t.Fatal(err)
	}
	count := 0
	sender := func(_ *Service, _ context.Context, _, subject, _ string) error {
		count++
		if !strings.Contains(subject, "failure") {
			t.Error("notification omitted the event")
		}
		if count == 1 {
			return errors.New("temporary error with a credential")
		}
		return nil
	}
	if err := s.dispatch(context.Background(), sender); err != nil {
		t.Fatal(err)
	}
	var delivery struct {
		Attempts    int
		DeliveredAt *time.Time
		LastError   string
	}
	if err := s.DB.Table("notification_deliveries").Take(&delivery).Error; err != nil {
		t.Fatal(err)
	}
	if delivery.Attempts != 1 || delivery.DeliveredAt != nil || strings.Contains(delivery.LastError, "credential") {
		t.Fatal("failed delivery did not retain a redacted retry")
	}
	if err := s.DB.Exec("UPDATE notification_deliveries SET next_at=now()").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.dispatch(context.Background(), sender); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Table("notification_deliveries").Take(&delivery).Error; err != nil {
		t.Fatal(err)
	}
	if delivery.DeliveredAt == nil || delivery.Attempts != 2 {
		t.Fatal("notification retry did not complete")
	}
	var stored storage.Run
	if err := s.DB.First(&stored, "id=?", run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "failure" {
		t.Fatal("notification changed the execution result")
	}
}
func TestRepositoryInspectionAndLostLease(t *testing.T) {
	s, a, p, _ := fixture(t)
	r := runnerFixture(t, s, a, "inspection")
	row, err := s.InspectRepository(a, p.ID, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := s.Poll(r)
	if err != nil || assignment == nil || assignment.InspectionID != row.ID {
		t.Fatalf("inspection assignment: %+v %v", assignment, err)
	}
	if err := s.InspectionHeartbeat(r, row.ID, assignment.LeaseToken); err != nil {
		t.Fatal(err)
	}
	document, err := s.Export(a, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	text, err := document.YAML()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.InspectionReport(r, row.ID, assignment.LeaseToken, string(text), ""); err != nil {
		t.Fatal(err)
	}
	result, err := s.Inspection(a, row.ID)
	if err != nil || result.Status != "complete" {
		t.Fatalf("inspection result: %+v %v", result, err)
	}
	if strings.Contains(string(result.Document), "token_hash") {
		t.Fatal("inspection included a runner credential")
	}
}
func TestMissedAndOverlappingSchedules(t *testing.T) {
	s, a, _, j := fixture(t)
	now := time.Now().Truncate(time.Minute)
	if err := s.DB.Exec("UPDATE schedules SET next_at=?", now.Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Tick(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	var rows []storage.Run
	if err := s.DB.Where("job_id=?", j.ID).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != "missed" {
		t.Fatal("missed schedule was replayed")
	}
	if _, err := s.Start(a, j.ID, "overlap"); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Exec("UPDATE schedules SET next_at=?", now).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Tick(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := s.DB.Model(&storage.Run{}).Where("job_id=? AND status='skipped'", j.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("overlapping schedule was not recorded")
	}
}

func TestIdentityApprovalInvitationAndLinking(t *testing.T) {
	s, a, _, _ := fixture(t)
	provider := storage.Provider{ID: storage.ID(), Name: "fixture", Kind: "github", Config: storage.Encode(map[string]any{}), Version: 1}
	if err := s.DB.Create(&provider).Error; err != nil {
		t.Fatal(err)
	}
	state := func() string {
		t.Helper()
		value := security.Token()
		if err := s.DB.Exec("INSERT INTO oauth_states(hash,provider_id,encrypted,expires_at) VALUES (?,?,?,?)", security.Hash(value), provider.ID, storage.Encode(map[string]any{}), time.Now().Add(time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
		return value
	}
	profile := providers.Profile{Subject: "stable-id", Name: "First Name"}
	if _, _, err := s.completeIdentity(context.Background(), provider.ID, state(), profile, nil, loginStateRow{}); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("unapproved login=%v", err)
	}
	var user storage.User
	if err := s.DB.Where("name=?", profile.Name).First(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.Enabled || user.Administrator {
		t.Fatal("new user obtained approval")
	}
	invitation, err := s.Invite(a)
	if err != nil {
		t.Fatal(err)
	}
	hash := security.Hash(invitation)
	pending := state()
	token, _, err := s.completeIdentity(context.Background(), provider.ID, pending, profile, nil, loginStateRow{InvitationHash: &hash})
	if err != nil {
		t.Fatal(err)
	}
	actor, err := s.Authenticate(context.Background(), token, true)
	if err != nil || actor.User.ID != user.ID {
		t.Fatal("invitation did not approve the existing identity")
	}
	if _, _, err := s.completeIdentity(context.Background(), provider.ID, pending, profile, nil, loginStateRow{}); !errors.Is(err, ErrForbidden) {
		t.Fatal("OAuth state was reused")
	}
	profile.Name = "Changed provider display name"
	_, _, err = s.completeIdentity(context.Background(), provider.ID, state(), profile, nil, loginStateRow{})
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := s.DB.Table("identities").Where("provider_id=? AND subject=?", provider.ID, profile.Subject).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("display name change created another identity")
	}
	if _, _, err := s.completeIdentity(context.Background(), provider.ID, state(), profile, nil, loginStateRow{UserID: &a.User.ID}); !errors.Is(err, ErrConflict) {
		t.Fatal("identity linked to a different account")
	}
	linked := providers.Profile{Subject: "additional-id", Name: "Another Identity"}
	if _, _, err := s.completeIdentity(context.Background(), provider.ID, state(), linked, nil, loginStateRow{UserID: &actor.User.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Table("identities").Where("user_id=?", user.ID).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("authenticated identity linking failed")
	}
}

func TestManagementAndProviderIsolation(t *testing.T) {
	s, a, p, _ := fixture(t)
	token, err := s.CreateToken(a, []string{"read"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.Tokens(a, 0)
	if err != nil || len(rows) != 1 || strings.Contains(string(storage.Encode(rows)), token) {
		t.Fatal("token metadata is invalid")
	}
	if err := s.RevokeToken(a, rows[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(context.Background(), token, false); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked token authenticated")
	}
	if err := s.DB.First(&p, "id=?", p.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.RenameProject(a, p.ID, "renamed", p.Version); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameProject(a, p.ID, "stale", p.Version); !errors.Is(err, ErrConflict) {
		t.Fatal("stale project version was accepted")
	}
	user, err := s.CreateUser(a, "Disabled user")
	if err != nil || user.Enabled {
		t.Fatal("administrator-created user was enabled")
	}
	if err := s.DeleteUser(a, user.ID, user.Version); err != nil {
		t.Fatal(err)
	}
	provider := storage.Provider{Name: "GitHub", Kind: "github", Config: storage.Encode(map[string]string{"clientSecret": "must-not-store"})}
	if _, err := s.SaveProvider(a, provider); !errors.Is(err, ErrInvalid) {
		t.Fatal("inline provider secret was accepted")
	}
	provider.Config = storage.Encode(providers.Config{})
	provider, err = s.SaveProvider(a, provider)
	if err != nil {
		t.Fatal(err)
	}
	provider.Config = storage.Encode(providers.Config{BaseURL: "https://another.example.com"})
	if _, err := s.SaveProvider(a, provider); !errors.Is(err, ErrInvalid) {
		t.Fatal("provider origin changed inside an existing identity namespace")
	}
	if err := s.DeleteProvider(a, provider.ID, provider.Version); err != nil {
		t.Fatal(err)
	}
}
