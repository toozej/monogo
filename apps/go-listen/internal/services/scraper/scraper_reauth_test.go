package scraper

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/toozej/monogo/apps/go-listen/internal/types"
)

// fakeSearcherReauth implements types.ArtistSearcher for reauth tests.
type fakeSearcherReauth struct {
	matchErr error
}

func (f *fakeSearcherReauth) FindBestMatch(query string) (*types.Artist, float64, error) {
	if f.matchErr != nil {
		return nil, 0, f.matchErr
	}
	return &types.Artist{ID: "artist-1", Name: "Test Artist"}, 1.0, nil
}

// fakePlaylistReauth implements types.PlaylistManager, surfacing a reauth error
// from GetTop5Tracks (and AddTracksToPlaylist) to exercise the add-loop abort.
type fakePlaylistReauth struct {
	getTracksErr error
	addErr       error
}

func (f *fakePlaylistReauth) AddArtistToPlaylist(artistName, playlistID string, force bool) (*types.AddResult, error) {
	return nil, errors.New("not implemented")
}
func (f *fakePlaylistReauth) GetIncomingPlaylists() ([]types.Playlist, error) {
	return nil, errors.New("not implemented")
}
func (f *fakePlaylistReauth) GetTop5Tracks(artistID string) ([]types.Track, error) {
	if f.getTracksErr != nil {
		return nil, f.getTracksErr
	}
	return []types.Track{{ID: "track-1", Name: "Track One"}}, nil
}
func (f *fakePlaylistReauth) FilterPlaylistsBySearch(playlists []types.Playlist, searchTerm string) []types.Playlist {
	return playlists
}
func (f *fakePlaylistReauth) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	if f.addErr != nil {
		return f.addErr
	}
	return nil
}
func (f *fakePlaylistReauth) CheckForDuplicates(playlistID string, trackIDs []string) (*types.DuplicateResult, error) {
	return &types.DuplicateResult{HasDuplicates: false}, nil
}

// stubExtractor returns a fixed list of artist names regardless of input text.
type stubExtractor struct{ names []string }

func (s *stubExtractor) ExtractArtists(text string) ([]string, error) { return s.names, nil }
func (s *stubExtractor) CleanArtistName(name string) string           { return name }

// TestMatchArtists_AbortsOnReauth verifies that matchArtists aborts the batch
// and returns a wrapped types.ErrReauthenticationRequired as soon as the
// searcher surfaces a reauth error, instead of recording it per-artist.
func TestMatchArtists_AbortsOnReauth(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	reauthErr := fmt.Errorf("search failed: %w", types.ErrReauthenticationRequired)
	w := &WebScraper{
		logger:   logger,
		searcher: &fakeSearcherReauth{matchErr: reauthErr},
	}

	results, err := w.matchArtists([]string{"Test Artist"})
	if err == nil {
		t.Fatal("matchArtists() expected reauth error but got none")
	}
	if !errors.Is(err, types.ErrReauthenticationRequired) {
		t.Errorf("matchArtists() error does not wrap ErrReauthenticationRequired: %v", err)
	}
	// The batch should abort before appending any completed result.
	if len(results) != 0 {
		t.Errorf("matchArtists() should not record results on reauth abort, got %d", len(results))
	}
}

// TestMatchArtists_RecordsOrdinaryFailure verifies non-reauth search failures
// are still recorded per-artist and do not abort the batch.
func TestMatchArtists_RecordsOrdinaryFailure(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	w := &WebScraper{
		logger:   logger,
		searcher: &fakeSearcherReauth{matchErr: errors.New("network blip")},
	}

	results, err := w.matchArtists([]string{"Test Artist"})
	if err != nil {
		t.Fatalf("matchArtists() unexpected error for ordinary failure: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("matchArtists() expected 1 recorded result, got %d", len(results))
	}
	if results[0].Error == "" {
		t.Error("matchArtists() should record the search error on the result")
	}
}

// TestScrapeAndAddToPlaylist_AbortsOnReauthDuringAdd verifies the scrape
// workflow aborts and returns the typed reauth error when GetTop5Tracks fails
// with reauth, instead of continuing to fail every remaining artist.
func TestScrapeAndAddToPlaylist_AbortsOnReauthDuringAdd(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Serve minimal HTML so ScrapeArtists can fetch and parse successfully.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div>Test Artist</div></body></html>`))
	}))
	defer ts.Close()

	reauthErr := fmt.Errorf("refresh failed: %w", types.ErrReauthenticationRequired)
	cfg := DefaultScraperConfig()
	cfg.AllowPrivateNetwork = true // allow the loopback httptest server
	w := NewWebScraper(
		cfg,
		NewGoqueryParser(logger),
		&stubExtractor{names: []string{"Test Artist"}},
		&fakeSearcherReauth{}, // successful match
		&fakePlaylistReauth{getTracksErr: reauthErr},
		logger,
	)

	result, err := w.ScrapeAndAddToPlaylist(ts.URL, "", "playlist-1", false)
	if err == nil {
		t.Fatal("ScrapeAndAddToPlaylist() expected reauth error but got none")
	}
	if !errors.Is(err, types.ErrReauthenticationRequired) {
		t.Errorf("ScrapeAndAddToPlaylist() error does not wrap ErrReauthenticationRequired: %v", err)
	}
	if result == nil {
		t.Fatal("ScrapeAndAddToPlaylist() should return a partial result even on reauth abort")
	}
	if !strings.Contains(result.Message, "reauthorization required") && !strings.Contains(result.Message, "reauth") {
		// Duplicate-check path may surface the message differently; ensure an error is recorded.
		if len(result.Errors) == 0 {
			t.Errorf("ScrapeAndAddToPlaylist() should record the reauth error, message=%q", result.Message)
		}
	}
}

// TestScrapeAndAddToPlaylist_AbortsOnReauthDuringMatch verifies the workflow
// aborts when the match phase itself hits a reauth error.
func TestScrapeAndAddToPlaylist_AbortsOnReauthDuringMatch(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div>Test Artist</div></body></html>`))
	}))
	defer ts.Close()

	reauthErr := fmt.Errorf("search failed: %w", types.ErrReauthenticationRequired)
	cfg := DefaultScraperConfig()
	cfg.AllowPrivateNetwork = true // allow the loopback httptest server
	w := NewWebScraper(
		cfg,
		NewGoqueryParser(logger),
		&stubExtractor{names: []string{"Test Artist"}},
		&fakeSearcherReauth{matchErr: reauthErr},
		&fakePlaylistReauth{},
		logger,
	)

	_, err := w.ScrapeAndAddToPlaylist(ts.URL, "", "playlist-1", false)
	if err == nil {
		t.Fatal("ScrapeAndAddToPlaylist() expected reauth error during match but got none")
	}
	if !errors.Is(err, types.ErrReauthenticationRequired) {
		t.Errorf("ScrapeAndAddToPlaylist() error does not wrap ErrReauthenticationRequired: %v", err)
	}
}
