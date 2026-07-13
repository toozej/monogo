package spotify

import (
	"errors"
	"log/slog"

	"github.com/toozej/monogo/apps/go-listen/internal/config"
	"github.com/toozej/monogo/apps/go-listen/internal/types"
)

// Service implements the types.SpotifyService interface
type Service struct {
	client *Client
	logger *slog.Logger
}

// GetAuthURL returns the URL for user authentication
func (s *Service) GetAuthURL() string {
	if s.client == nil {
		return ""
	}
	return s.client.GetAuthURL()
}

// IsAuthenticated returns whether the user is authenticated
func (s *Service) IsAuthenticated() bool {
	if s.client == nil {
		return false
	}
	return s.client.IsAuthenticated()
}

// CompleteAuth completes the authentication process
func (s *Service) CompleteAuth(code, state string) error {
	if s.client == nil {
		return errors.New("spotify client not available")
	}
	return s.client.CompleteAuth(code, state)
}

// NewService creates a new Spotify service that implements types.SpotifyService
func NewService(cfg config.SpotifyConfig, logger *slog.Logger) *Service {
	logger.Debug("Creating Spotify service with config",
		"client_id", cfg.ClientID,
		"client_secret", cfg.ClientSecret != "",
		"redirect_url", cfg.RedirectURL,
	)

	client, err := NewClient(cfg, logger)
	if err != nil {
		logger.Error("Failed to create Spotify client", "error", err)
		// Return service with nil client for now - will be handled in actual implementation
		return &Service{
			client: nil,
			logger: logger,
		}
	}

	return &Service{
		client: client,
		logger: logger,
	}
}

// SearchArtist searches for an artist by name and returns the best match
func (s *Service) SearchArtist(query string) (*types.Artist, error) {
	if s.client == nil {
		return nil, errors.New("spotify client not available")
	}

	s.logger.Debug("Searching for artist",
		"component", "spotify_service",
		"operation", "search_artist",
		"query", query,
	)

	artist, err := s.client.SearchArtist(query)
	if err != nil {
		s.logger.Error("Failed to search for artist",
			"error", err,
			"component", "spotify_service",
			"operation", "search_artist",
			"query", query,
		)
		return nil, err
	}

	s.logger.Info("Artist search completed successfully",
		"component", "spotify_service",
		"operation", "search_artist",
		"query", query,
		"matched_artist", artist.Name,
		"artist_id", artist.ID,
	)

	return &types.Artist{
		ID:     artist.ID,
		Name:   artist.Name,
		URI:    artist.URI,
		Genres: artist.Genres,
	}, nil
}

// GetArtistTopTracks retrieves the top 5 tracks for an artist
func (s *Service) GetArtistTopTracks(artistID string) ([]types.Track, error) {
	if s.client == nil {
		return nil, errors.New("spotify client not available")
	}

	s.logger.Debug("Retrieving artist top tracks",
		"component", "spotify_service",
		"operation", "get_top_tracks",
		"artist_id", artistID,
	)

	tracks, err := s.client.GetArtistTopTracks(artistID)
	if err != nil {
		s.logger.Error("Failed to retrieve artist top tracks",
			"error", err,
			"component", "spotify_service",
			"operation", "get_top_tracks",
			"artist_id", artistID,
		)
		return nil, err
	}

	serverTracks := make([]types.Track, len(tracks))
	trackNames := make([]string, len(tracks))
	for i, track := range tracks {
		// Convert artists
		artists := make([]types.Artist, len(track.Artists))
		for j, artist := range track.Artists {
			artists[j] = types.Artist{
				ID:     artist.ID,
				Name:   artist.Name,
				URI:    artist.URI,
				Genres: artist.Genres,
			}
		}

		serverTracks[i] = types.Track{
			ID:       track.ID,
			Name:     track.Name,
			URI:      track.URI,
			Artists:  artists,
			Duration: track.Duration,
		}
		trackNames[i] = track.Name
	}

	s.logger.Info("Retrieved artist top tracks successfully",
		"component", "spotify_service",
		"operation", "get_top_tracks",
		"artist_id", artistID,
		"track_count", len(serverTracks),
		"track_names", trackNames,
	)

	return serverTracks, nil
}

// GetUserPlaylists retrieves playlists from a specific folder
func (s *Service) GetUserPlaylists(folderName string) ([]types.Playlist, error) {
	if s.client == nil {
		return nil, errors.New("spotify client not available")
	}

	s.logger.Debug("Retrieving user playlists",
		"component", "spotify_service",
		"operation", "get_playlists",
		"folder_name", folderName,
	)

	playlists, err := s.client.GetUserPlaylists(folderName)
	if err != nil {
		s.logger.Error("Failed to retrieve user playlists",
			"error", err,
			"component", "spotify_service",
			"operation", "get_playlists",
			"folder_name", folderName,
		)
		return nil, err
	}

	serverPlaylists := make([]types.Playlist, len(playlists))
	playlistNames := make([]string, len(playlists))
	for i, playlist := range playlists {
		serverPlaylists[i] = types.Playlist{
			ID:         playlist.ID,
			Name:       playlist.Name,
			URI:        playlist.URI,
			TrackCount: playlist.TrackCount,
			EmbedURL:   playlist.EmbedURL,
			IsIncoming: playlist.IsIncoming,
		}
		playlistNames[i] = playlist.Name
	}

	s.logger.Info("Retrieved user playlists successfully",
		"component", "spotify_service",
		"operation", "get_playlists",
		"folder_name", folderName,
		"playlist_count", len(serverPlaylists),
		"playlist_names", playlistNames,
	)

	return serverPlaylists, nil
}

// AddTracksToPlaylist adds tracks to a specified playlist
func (s *Service) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	if s.client == nil {
		return errors.New("spotify client not available")
	}

	s.logger.Debug("Adding tracks to playlist",
		"component", "spotify_service",
		"operation", "add_tracks",
		"playlist_id", playlistID,
		"track_count", len(trackIDs),
		"track_ids", trackIDs,
	)

	err := s.client.AddTracksToPlaylist(playlistID, trackIDs)
	if err != nil {
		s.logger.Error("Failed to add tracks to playlist",
			"error", err,
			"component", "spotify_service",
			"operation", "add_tracks",
			"playlist_id", playlistID,
			"track_count", len(trackIDs),
		)
		return err
	}

	s.logger.Info("Successfully added tracks to playlist",
		"component", "spotify_service",
		"operation", "add_tracks",
		"playlist_id", playlistID,
		"track_count", len(trackIDs),
	)

	return nil
}

// CheckTracksInPlaylist checks if tracks already exist in a playlist
func (s *Service) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	if s.client == nil {
		return nil, errors.New("spotify client not available")
	}

	s.logger.Debug("Checking for duplicate tracks in playlist",
		"component", "spotify_service",
		"operation", "check_tracks",
		"playlist_id", playlistID,
		"track_count", len(trackIDs),
	)

	results, err := s.client.CheckTracksInPlaylist(playlistID, trackIDs)
	if err != nil {
		s.logger.Error("Failed to check tracks in playlist",
			"error", err,
			"component", "spotify_service",
			"operation", "check_tracks",
			"playlist_id", playlistID,
			"track_count", len(trackIDs),
		)
		return nil, err
	}

	duplicateCount := 0
	for _, isDuplicate := range results {
		if isDuplicate {
			duplicateCount++
		}
	}

	s.logger.Info("Completed duplicate track check",
		"component", "spotify_service",
		"operation", "check_tracks",
		"playlist_id", playlistID,
		"track_count", len(trackIDs),
		"duplicate_count", duplicateCount,
	)

	return results, nil
}
