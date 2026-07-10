package ghreleases2rss

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
