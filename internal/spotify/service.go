package spotify

import (
	"errors"

	"github.com/sirupsen/logrus"
	"github.com/toozej/kmhd2spotify/internal/types"
	"github.com/toozej/kmhd2spotify/pkg/config"
)

// Service implements the types.SpotifyService interface
type Service struct {
	client *Client
	logger *logrus.Logger
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
func NewService(cfg config.SpotifyConfig, logger *logrus.Logger) *Service {
	logger.WithFields(logrus.Fields{
		"client_id":     cfg.ClientID,
		"client_secret": cfg.ClientSecret != "",
		"redirect_url":  cfg.RedirectURL,
	}).Debug("Creating Spotify service with config")

	client, err := NewClient(cfg, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to create Spotify client")
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

	s.logger.WithFields(logrus.Fields{
		"component": "spotify_service",
		"operation": "search_artist",
		"query":     query,
	}).Debug("Searching for artist")

	artist, err := s.client.SearchArtist(query)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"component": "spotify_service",
			"operation": "search_artist",
			"query":     query,
		}).WithError(err).Error("Failed to search for artist")
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"component":      "spotify_service",
		"operation":      "search_artist",
		"query":          query,
		"matched_artist": artist.Name,
		"artist_id":      artist.ID,
	}).Info("Artist search completed successfully")

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

	s.logger.WithFields(logrus.Fields{
		"component": "spotify_service",
		"operation": "get_top_tracks",
		"artist_id": artistID,
	}).Debug("Retrieving artist top tracks")

	tracks, err := s.client.GetArtistTopTracks(artistID)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"component": "spotify_service",
			"operation": "get_top_tracks",
			"artist_id": artistID,
		}).WithError(err).Error("Failed to retrieve artist top tracks")
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
			Album: types.Album{
				ID:   track.Album.ID,
				Name: track.Album.Name,
				Type: track.Album.Type,
			},
		}
		trackNames[i] = track.Name
	}

	s.logger.WithFields(logrus.Fields{
		"component":   "spotify_service",
		"operation":   "get_top_tracks",
		"artist_id":   artistID,
		"track_count": len(serverTracks),
		"track_names": trackNames,
	}).Info("Retrieved artist top tracks successfully")

	return serverTracks, nil
}

// GetUserPlaylists retrieves playlists from a specific folder
func (s *Service) GetUserPlaylists(folderName string) ([]types.Playlist, error) {
	if s.client == nil {
		return nil, errors.New("spotify client not available")
	}

	s.logger.WithFields(logrus.Fields{
		"component":   "spotify_service",
		"operation":   "get_playlists",
		"folder_name": folderName,
	}).Debug("Retrieving user playlists")

	playlists, err := s.client.GetUserPlaylists(folderName)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"component":   "spotify_service",
			"operation":   "get_playlists",
			"folder_name": folderName,
		}).WithError(err).Error("Failed to retrieve user playlists")
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

	s.logger.WithFields(logrus.Fields{
		"component":      "spotify_service",
		"operation":      "get_playlists",
		"folder_name":    folderName,
		"playlist_count": len(serverPlaylists),
		"playlist_names": playlistNames,
	}).Info("Retrieved user playlists successfully")

	return serverPlaylists, nil
}

// AddTracksToPlaylist adds tracks to a specified playlist
func (s *Service) AddTracksToPlaylist(playlistID string, trackIDs []string) error {
	if s.client == nil {
		return errors.New("spotify client not available")
	}

	s.logger.WithFields(logrus.Fields{
		"component":   "spotify_service",
		"operation":   "add_tracks",
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
		"track_ids":   trackIDs,
	}).Debug("Adding tracks to playlist")

	err := s.client.AddTracksToPlaylist(playlistID, trackIDs)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"component":   "spotify_service",
			"operation":   "add_tracks",
			"playlist_id": playlistID,
			"track_count": len(trackIDs),
		}).WithError(err).Error("Failed to add tracks to playlist")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"component":   "spotify_service",
		"operation":   "add_tracks",
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
	}).Info("Successfully added tracks to playlist")

	return nil
}

// CheckTracksInPlaylist checks if tracks already exist in a playlist
func (s *Service) CheckTracksInPlaylist(playlistID string, trackIDs []string) ([]bool, error) {
	if s.client == nil {
		return nil, errors.New("spotify client not available")
	}

	s.logger.WithFields(logrus.Fields{
		"component":   "spotify_service",
		"operation":   "check_tracks",
		"playlist_id": playlistID,
		"track_count": len(trackIDs),
	}).Debug("Checking for duplicate tracks in playlist")

	results, err := s.client.CheckTracksInPlaylist(playlistID, trackIDs)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"component":   "spotify_service",
			"operation":   "check_tracks",
			"playlist_id": playlistID,
			"track_count": len(trackIDs),
		}).WithError(err).Error("Failed to check tracks in playlist")
		return nil, err
	}

	duplicateCount := 0
	for _, isDuplicate := range results {
		if isDuplicate {
			duplicateCount++
		}
	}

	s.logger.WithFields(logrus.Fields{
		"component":       "spotify_service",
		"operation":       "check_tracks",
		"playlist_id":     playlistID,
		"track_count":     len(trackIDs),
		"duplicate_count": duplicateCount,
	}).Info("Completed duplicate track check")

	return results, nil
}

// CreatePlaylist creates a new playlist with the given name and description
func (s *Service) CreatePlaylist(name, description string, public bool) (*types.Playlist, error) {
	if s.client == nil {
		return nil, errors.New("spotify client not available")
	}

	s.logger.WithFields(logrus.Fields{
		"component":     "spotify_service",
		"operation":     "create_playlist",
		"playlist_name": name,
		"description":   description,
		"public":        public,
	}).Debug("Creating new playlist")

	playlist, err := s.client.CreatePlaylist(name, description, public)
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"component":     "spotify_service",
			"operation":     "create_playlist",
			"playlist_name": name,
		}).WithError(err).Error("Failed to create playlist")
		return nil, err
	}

	s.logger.WithFields(logrus.Fields{
		"component":     "spotify_service",
		"operation":     "create_playlist",
		"playlist_id":   playlist.ID,
		"playlist_name": playlist.Name,
	}).Info("Successfully created playlist")

	return &types.Playlist{
		ID:         playlist.ID,
		Name:       playlist.Name,
		URI:        playlist.URI,
		TrackCount: playlist.TrackCount,
		EmbedURL:   playlist.EmbedURL,
		IsIncoming: playlist.IsIncoming,
	}, nil
}
