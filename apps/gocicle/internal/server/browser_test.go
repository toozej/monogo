package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/gocicle/internal/config"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/service"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"github.com/toozej/monogo/apps/gocicle/internal/testutil"
)

func TestBrowserOnboardingEditingAndLiveEvents(t *testing.T) {
	if os.Getenv("GOCICLE_PLAYWRIGHT_MODULE") == "" {
		t.Skip("set GOCICLE_TEST_BROWSER=1 when running the acceptance script")
	}
	db := testutil.Database(t)
	svc := service.New(db, security.Keyring{Active: "test", Keys: map[string]string{"test": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))}})
	owner := storage.User{ID: storage.ID(), Name: "Browser test", Enabled: true, Administrator: true, Version: 1, Preferences: storage.Encode(map[string]any{})}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	actor := service.Actor{User: owner, Scopes: []string{"admin"}}
	enrollment, err := svc.EnrollToken(actor, "browser-runner", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	runner, token, err := svc.Enroll(enrollment)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RunnerGrant(actor, runner.ID, owner.ID, true); err != nil {
		t.Fatal(err)
	}
	session, csrf := security.Token(), security.Token()
	if err := db.Exec("INSERT INTO sessions VALUES (?,?,?,?)", security.Hash(session), owner.ID, security.Hash(csrf), time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	control := New(config.Config{}, svc)
	host := httptest.NewUnstartedServer(control.Handler())
	control.Config.PublicURL = "https://" + host.Listener.Addr().String()
	host.StartTLS()
	defer host.Close()
	script, err := filepath.Abs("../../tests/browser.cjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "node", script)
	command.Stdin = bytes.NewReader(storage.Encode(map[string]string{"url": host.URL, "session": session, "csrf": csrf, "runnerToken": token}))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("browser acceptance failed: %v\n%s", err, output)
	}
}
