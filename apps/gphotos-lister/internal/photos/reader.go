// Package photos reads album hearts through the Google Photos web interface.
package photos

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"github.com/toozej/monogo/apps/gphotos-lister/internal/lister"
)

// Session provides the read operations needed by the album reader.
type Session interface {
	Navigate(context.Context, string) (string, error)
	RPC(context.Context, string, []any) ([]any, error)
}

// ValidateAlbumURL permits Google Photos album and short sharing URLs.
func ValidateAlbumURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid album URL")
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return fmt.Errorf("use an HTTPS Google Photos album URL")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if u.Host == "photos.app.goo.gl" && len(parts) == 1 && parts[0] != "" {
		return nil
	}
	if u.Host != "photos.google.com" {
		return fmt.Errorf("use a photos.google.com or photos.app.goo.gl album URL")
	}
	if len(parts) >= 2 && parts[0] == "u" {
		parts = parts[2:]
	}
	if len(parts) != 2 || (parts[0] != "album" && parts[0] != "share") || parts[1] == "" {
		return fmt.Errorf("use an album URL, without a photo path")
	}
	return nil
}

// Read enumerates every album page before it reads hearts and filenames.
func Read(ctx context.Context, session Session, albumURL string) ([]string, error) {
	return read(ctx, session, albumURL, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func read(ctx context.Context, session Session, albumURL string, logger *slog.Logger) ([]string, error) {
	if err := ValidateAlbumURL(albumURL); err != nil {
		return nil, err
	}
	resolved, err := session.Navigate(ctx, albumURL)
	if err != nil {
		return nil, err
	}
	if err := ValidateAlbumURL(resolved); err != nil {
		return nil, fmt.Errorf("cannot open this public album; check that link sharing is enabled")
	}
	u, _ := url.Parse(resolved)
	if u.Host != "photos.google.com" {
		return nil, fmt.Errorf("the sharing URL did not open a Google Photos album")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	albumID := parts[len(parts)-1]
	authKey := u.Query().Get("key")
	logger.DebugContext(ctx, "Resolved sharing link", "sharing_key", authKey != "")
	var page any
	var media []string
	seenPages := make(map[string]bool)
	seenMedia := make(map[string]bool)
	albumPages := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := session.RPC(ctx, "snAcKc", []any{albumID, page, nil, optional(authKey)})
		if err != nil {
			return nil, fmt.Errorf("list album: %w", err)
		}
		if len(data) < 4 {
			return nil, fmt.Errorf("the Google Photos interface changed the album response or denied access")
		}
		if page == nil {
			metadataID, ok := at(data, 3, 0).(string)
			if !ok || metadataID == "" {
				return nil, fmt.Errorf("the Google Photos interface did not return album metadata")
			}
			logger.DebugContext(ctx, "Read album metadata", "metadata_id_matches_link", metadataID == albumID)
		}
		if authKey == "" {
			authKey, _ = at(data, 3, 19).(string)
		}
		items, ok := data[1].([]any)
		if data[1] != nil && !ok {
			return nil, fmt.Errorf("the Google Photos interface returned invalid album items")
		}
		for _, item := range items {
			values, ok := item.([]any)
			if !ok || len(values) == 0 {
				return nil, fmt.Errorf("the Google Photos interface returned an invalid album item")
			}
			id, ok := values[0].(string)
			if !ok || id == "" {
				return nil, fmt.Errorf("the Google Photos interface returned an invalid photo ID")
			}
			// The web protocol marks videos in this extension object.
			video := false
			for _, value := range values {
				if extensions, ok := value.(map[string]any); ok {
					_, video = extensions["76647426"]
					if video {
						break
					}
				}
			}
			if !video && !seenMedia[id] {
				media = append(media, id)
				seenMedia[id] = true
			}
		}
		next, ok := data[2].(string)
		if data[2] != nil && !ok {
			return nil, fmt.Errorf("the Google Photos interface returned an invalid album page token")
		}
		albumPages++
		logger.DebugContext(ctx, "Read album page", "page", albumPages, "items", len(items), "photos_total", len(media), "has_next", next != "")
		if next == "" {
			break
		}
		if seenPages[next] {
			return nil, fmt.Errorf("the Google Photos interface repeated an album page token")
		}
		seenPages[next] = true
		page = next
	}
	liked, err := readHeartsWithLogger(ctx, session, albumID, authKey, logger)
	if err != nil {
		return nil, err
	}
	matched := 0
	for _, id := range media {
		if liked[id] {
			matched++
		}
	}
	logger.DebugContext(ctx, "Matched album hearts", "photos", len(media), "hearted_photo_ids", len(liked), "matched_photos", matched)
	names := make([]string, 0)
	for _, id := range media {
		if !liked[id] {
			continue
		}
		data, err := session.RPC(ctx, "fDcn4b", []any{id, nil, optional(authKey), nil, nil, []any{2}})
		if err != nil {
			return nil, fmt.Errorf("read original filename: %w", err)
		}
		name, ok := at(data, 0, 2).(string)
		if !ok || !lister.ValidFilename(name) {
			return nil, fmt.Errorf("the Google Photos interface did not return a valid original filename")
		}
		names = append(names, name)
		logger.DebugContext(ctx, "Read original filename", "completed", len(names), "total", matched)
	}
	return names, nil
}

// Collection moments use type 3 for hearts. A type 1 heart target is a photo.
// A type 2 heart target is the album itself. Comments use moment type 2.
func readHearts(ctx context.Context, session Session, albumID, authKey string) (map[string]bool, error) {
	return readHeartsWithLogger(ctx, session, albumID, authKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func readHeartsWithLogger(ctx context.Context, session Session, albumID, authKey string, logger *slog.Logger) (map[string]bool, error) {
	liked := make(map[string]bool)
	seenPages := make(map[string]bool)
	var page any
	activityPages := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := session.RPC(ctx, "wQ6iqd", []any{[]any{albumID}, page, optional(authKey)})
		if err != nil {
			return nil, fmt.Errorf("read album activity: %w", err)
		}
		if len(data) < 3 {
			return nil, fmt.Errorf("the Google Photos interface returned invalid album activity")
		}
		if id, ok := data[2].(string); !ok || id == "" {
			return nil, fmt.Errorf("the Google Photos interface did not identify the activity album")
		}
		events, ok := data[1].([]any)
		if data[1] != nil && !ok {
			return nil, fmt.Errorf("the Google Photos interface returned invalid album events")
		}
		eventTypes := make(map[string]int)
		albumHearts := 0
		for _, event := range events {
			item, ok := event.([]any)
			if !ok || len(item) == 0 {
				return nil, fmt.Errorf("the Google Photos interface returned an invalid activity event")
			}
			kind, ok := item[0].(float64)
			if !ok {
				return nil, fmt.Errorf("the Google Photos interface returned an invalid activity type")
			}
			eventTypes[fmt.Sprint(kind)]++
			if kind != 3 {
				continue
			}
			targetType, ok := at(item, 5, 1).(float64)
			if !ok {
				return nil, fmt.Errorf("the Google Photos interface returned an invalid heart target")
			}
			if targetType == 2 {
				albumHearts++
				continue
			}
			if targetType != 1 {
				return nil, fmt.Errorf("the Google Photos interface returned an unknown heart target")
			}
			id, ok := at(item, 5, 2, 0).(string)
			if !ok || id == "" {
				return nil, fmt.Errorf("the Google Photos interface did not identify a hearted photo")
			}
			liked[id] = true
		}
		next, ok := data[0].(string)
		if data[0] != nil && !ok {
			return nil, fmt.Errorf("the Google Photos interface returned an invalid activity page token")
		}
		activityPages++
		logger.DebugContext(ctx, "Read activity page", "page", activityPages, "events", len(events), "event_types", eventTypes, "album_hearts", albumHearts, "hearted_photo_ids_total", len(liked), "has_next", next != "")
		if next == "" {
			return liked, nil
		}
		if seenPages[next] {
			return nil, fmt.Errorf("the Google Photos interface repeated an activity page token")
		}
		seenPages[next] = true
		page = next
	}
}

func optional(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func at(data any, indices ...int) any {
	for _, index := range indices {
		list, ok := data.([]any)
		if !ok || index >= len(list) {
			return nil
		}
		data = list[index]
	}
	return data
}
