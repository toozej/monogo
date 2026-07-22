package notification

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/monogo/apps/go-find-liquor/internal/config"
)

func TestGotifyErrorHookSendsErrorEntries(t *testing.T) {
	var payload struct {
		Title   string `json:"title"`
		Message string `json:"message"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/message" || r.URL.Query().Get("token") != "token" {
			t.Errorf("request = %s", r.URL.String())
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hook := NewGotifyErrorHook([]config.UserConfig{{Notifications: []config.NotificationConfig{{
		Type: "gotify", Endpoint: server.URL, Credential: map[string]string{"token": "token"},
	}}}})
	logger := log.New()
	logger.AddHook(hook)
	logger.Error("search failed")
	if payload.Title != "GFL - Error" || payload.Message != "search failed" {
		t.Errorf("payload = %+v", payload)
	}
}
