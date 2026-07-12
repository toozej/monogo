package ghreleases2rss

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/toozej/monogo/apps/ghreleases2rss/internal/config"
)

func testCommand(file, category string, clear, debug bool) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().String("file", file, "")
	cmd.Flags().String("category", category, "")
	cmd.Flags().Bool("clearCategoryFeeds", clear, "")
	cmd.Flags().Bool("debug", debug, "")
	return cmd
}

func TestRunRequiresCategoryForClear(t *testing.T) {
	err := Run(testCommand("missing", "", true, false), nil, config.Config{})
	if err == nil {
		t.Fatal("expected clear without category to fail")
	}
}

func TestRunDebugDoesNotMutateMiniflux(t *testing.T) {
	dir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(originalWD) }()
	input := filepath.Join(dir, "repos.txt")
	if err := os.WriteFile(input, []byte("owner/repo\n"), 0600); err != nil {
		t.Fatal(err)
	}

	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/categories":
			_, _ = w.Write([]byte(`[{"id":1,"title":"Tech"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/categories/1/feeds":
			_, _ = w.Write([]byte(`[{"id":10}]`))
		default:
			mutations++
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	err = Run(testCommand("repos.txt", "Tech", true, true), nil, config.Config{MinifluxURL: server.URL, MinifluxAPIKey: "key"})
	if err != nil {
		t.Fatal(err)
	}
	if mutations != 0 {
		t.Fatalf("debug run performed %d mutations", mutations)
	}
}

// TestRunMissingInputFileDoesNotTouchMiniflux verifies the "fail safely"
// guarantee: when the input file cannot be opened, the run aborts before any
// remote call so a clear operation can never empty a category on bad input.
func TestRunMissingInputFileDoesNotTouchMiniflux(t *testing.T) {
	dir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	contacted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contacted = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err = Run(testCommand("does-not-exist.txt", "Tech", true, false), nil, config.Config{MinifluxURL: server.URL, MinifluxAPIKey: "key"})
	if err == nil {
		t.Fatal("expected error when input file is missing")
	}
	if contacted {
		t.Fatal("miniflux must not be contacted when input file is missing")
	}
}

func TestRunValidatesEntireInputBeforeClear(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "invalid repository", content: "not-a-repository\n"},
		{name: "scanner failure", content: strings.Repeat("a", 70<<10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			if err := os.WriteFile("repos.txt", []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}

			contacted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contacted = true
				switch r.URL.Path {
				case "/v1/categories":
					_, _ = w.Write([]byte(`[{"id":1,"title":"Tech"}]`))
				case "/v1/categories/1/feeds":
					_, _ = w.Write([]byte(`[]`))
				default:
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer server.Close()

			err := Run(testCommand("repos.txt", "Tech", true, false), nil, config.Config{
				MinifluxURL:    server.URL,
				MinifluxAPIKey: "key",
			})
			if err == nil {
				t.Fatal("expected invalid input to fail")
			}
			if contacted {
				t.Fatal("miniflux must not be contacted before the entire input is validated")
			}
		})
	}
}

func TestRunProcessesValidReposWhileReportingInvalidRepos(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("repos.txt", []byte("not-a-repository\nowner/repo\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	subscriptions := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/feeds" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		subscriptions++
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	err := Run(testCommand("repos.txt", "", false, false), nil, config.Config{
		MinifluxURL:    server.URL,
		MinifluxAPIKey: "key",
	})
	if err == nil || !strings.Contains(err.Error(), `process repo "not-a-repository"`) {
		t.Fatalf("expected aggregated invalid-repository error, got %v", err)
	}
	if subscriptions != 1 {
		t.Fatalf("subscriptions = %d, want 1", subscriptions)
	}
}

func TestRunReportsInputAndCategoryErrors(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("repos.txt", []byte("not-a-repository\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/categories" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	err := Run(testCommand("repos.txt", "Missing", false, false), nil, config.Config{
		MinifluxURL:    server.URL,
		MinifluxAPIKey: "key",
	})
	if err == nil ||
		!strings.Contains(err.Error(), `process repo "not-a-repository"`) ||
		!strings.Contains(err.Error(), `validate category "Missing"`) {
		t.Fatalf("expected input and category errors, got %v", err)
	}
}

func TestOpenFileSecurelyRejectsSymlinkEscape(t *testing.T) {
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "repos.txt")
	if err := os.WriteFile(outside, []byte("owner/repo\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "repos.txt")); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	file, err := openFileSecurely("repos.txt")
	if err == nil {
		_ = file.Close()
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestRunAggregatesRemoteFailuresAndContinues(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("repos.txt", []byte("owner/one\nowner/two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	deletions := 0
	subscriptions := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/categories":
			_, _ = w.Write([]byte(`[{"id":1,"title":"Tech"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/categories/1/feeds":
			_, _ = w.Write([]byte(`[{"id":10},{"id":20}]`))
		case r.Method == http.MethodDelete:
			deletions++
			if r.URL.Path == "/v1/feeds/10" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/feeds":
			subscriptions++
			w.WriteHeader(http.StatusBadGateway)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	err := Run(testCommand("repos.txt", "Tech", true, false), nil, config.Config{
		MinifluxURL:    server.URL,
		MinifluxAPIKey: "key",
	})
	if err == nil ||
		!strings.Contains(err.Error(), "delete feed 10") ||
		!strings.Contains(err.Error(), "owner/one/releases.atom") ||
		!strings.Contains(err.Error(), "owner/two/releases.atom") {
		t.Fatalf("expected all remote failures, got %v", err)
	}
	if deletions != 2 || subscriptions != 2 {
		t.Fatalf("deletions = %d, subscriptions = %d; want 2 each", deletions, subscriptions)
	}
}
