// Package rss2socials provides the main logic for monitoring RSS feeds and posting updates to Mastodon, Bluesky, and Threads.
// It handles configuration, feed checking, post processing, and integration with other components.
package rss2socials

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/monogo/apps/rss2socials/internal/bluesky"
	"github.com/toozej/monogo/apps/rss2socials/internal/config"
	"github.com/toozej/monogo/apps/rss2socials/internal/db"
	"github.com/toozej/monogo/apps/rss2socials/internal/gotify"
	"github.com/toozej/monogo/apps/rss2socials/internal/mastodon"
	"github.com/toozej/monogo/apps/rss2socials/internal/rss"
	"github.com/toozej/monogo/apps/rss2socials/internal/threads"
)

// shouldSkipPost checks whether a post should be skipped based on the
// SkipPrefixCategories config. A post is skipped when any category in the
// list matches (case-insensitive) either the beginning of the post Title or
// the last path segment of the post Link.
func shouldSkipPost(post rss.RSSItem, skipPrefixCategories []string) bool {
	lastSegment := path.Base(post.Link)
	titleLower := strings.ToLower(post.Title)
	segmentLower := strings.ToLower(lastSegment)

	for _, cat := range skipPrefixCategories {
		catLower := strings.ToLower(cat)
		if strings.HasPrefix(titleLower, catLower) || strings.HasPrefix(segmentLower, catLower) {
			return true
		}
	}
	return false
}

func Run(conf config.Config) error {
	return RunContext(context.Background(), conf)
}

func RunContext(ctx context.Context, conf config.Config) (runErr error) {
	if err := config.ValidateRequired(conf); err != nil {
		return err
	}
	if err := db.InitDB(conf.DBPath); err != nil {
		return err
	}
	defer func() {
		runErr = errors.Join(runErr, db.CloseDB())
	}()

	startupTime := time.Now().UTC().Format(time.RFC3339)
	firstSnapshot, err := db.IsFirstCycleE()
	if err != nil {
		return fmt.Errorf("check database state: %w", err)
	}
	seedFirstSnapshot := conf.PostNewEntriesOnly && firstSnapshot
	interval := time.Duration(conf.Interval) * time.Minute

	for {
		posts, err := rss.CheckRSSFeedContext(ctx, conf.FeedURL)
		if err != nil {
			if conf.ShortRun {
				return err
			}
			log.Printf("Error fetching RSS feed: %v", err)
			if err := waitForNextCycle(ctx, interval); err != nil {
				return err
			}
			continue
		}

		posts = filteredPosts(posts, conf)
		if seedFirstSnapshot {
			if err := db.SeedPosts(posts, startupTime); err != nil {
				return fmt.Errorf("seed existing feed snapshot: %w", err)
			}
			seedFirstSnapshot = false
			if conf.ShortRun {
				return nil
			}
		} else {
			if conf.ShortRun && len(posts) > 3 {
				log.Info("Short run mode: processing only the 3 most recent items")
				posts = newestPosts(posts, 3)
			}

			var cycleErr error
			for _, post := range posts {
				cycleErr = errors.Join(cycleErr, handlePostContext(ctx, post, &conf, startupTime, false))
			}
			if cycleErr != nil {
				if conf.ShortRun {
					return cycleErr
				}
				log.Errorf("Errors processing feed: %v", cycleErr)
			}
			if conf.ShortRun {
				return nil
			}
		}

		if err := waitForNextCycle(ctx, interval); err != nil {
			return err
		}
	}
}

func waitForNextCycle(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func filteredPosts(posts []rss.RSSItem, conf config.Config) []rss.RSSItem {
	filtered := make([]rss.RSSItem, 0, len(posts))
	for _, post := range posts {
		if shouldSkipPost(post, conf.SkipPrefixCategories) {
			continue
		}
		if conf.Category != "" && !strings.Contains(path.Base(post.Link), conf.Category) {
			continue
		}
		filtered = append(filtered, post)
	}
	return filtered
}

func newestPosts(posts []rss.RSSItem, limit int) []rss.RSSItem {
	sorted := append([]rss.RSSItem(nil), posts...)
	sort.SliceStable(sorted, func(i, j int) bool {
		iTime, iErr := sorted[i].ParsePubDate()
		jTime, jErr := sorted[j].ParsePubDate()
		switch {
		case iErr == nil && jErr == nil:
			return iTime.After(jTime)
		case iErr == nil:
			return true
		case jErr == nil:
			return false
		default:
			return false
		}
	})
	if len(sorted) > limit {
		return sorted[:limit]
	}
	return sorted
}

func handlePost(post rss.RSSItem, conf *config.Config, startupTime string, skipIfExisting bool) error {
	return handlePostContext(context.Background(), post, conf, startupTime, skipIfExisting)
}

func handlePostContext(ctx context.Context, post rss.RSSItem, conf *config.Config, startupTime string, skipIfExisting bool) error {
	exists, updated, err := db.HasPostChanged(post.Link, post.Content)
	if err != nil {
		return fmt.Errorf("check post %q: %w", post.Link, err)
	}

	if skipIfExisting && exists && !updated {
		log.Debugf("Skipping existing post %s: PostNewEntriesOnly enabled on first cycle", post.Link)
		return nil
	}

	var tootContent string
	isUpdate := updated
	if exists && !updated {
		isUpdate, err = db.IsUpdatePending(post.Link)
		if err != nil {
			return fmt.Errorf("check update state for %q: %w", post.Link, err)
		}
	}

	switch {
	case exists && updated:
		log.Printf("Post has been updated: %s", post.Title)
		tootContent = fmt.Sprintf("Updated post: %s", post.Link)
	case !exists:
		tootContent = mastodon.GetTootContent(post)
	case exists && !updated:
		complete, err := allEnabledSitesPosted(post.Link, conf.EnabledSites())
		if err != nil {
			return err
		}
		if complete {
			return nil
		}
		if isUpdate {
			tootContent = fmt.Sprintf("Updated post: %s", post.Link)
		} else {
			tootContent = mastodon.GetTootContent(post)
		}
	default:
		return nil
	}

	if updated {
		err = db.StoreUpdatedPost(post.Link, post.Content, startupTime)
	} else {
		err = db.StoreTootedPost(post.Link, post.Content, startupTime)
	}
	if err != nil {
		return fmt.Errorf("store post %q: %w", post.Link, err)
	}

	enabledSites := conf.EnabledSites()
	siteMap := make(map[string]bool, len(enabledSites))
	for _, s := range enabledSites {
		siteMap[s] = true
	}

	var postErr error
	if siteMap["mastodon"] {
		alreadyPosted, err := db.IsSitePosted(post.Link, "mastodon")
		switch {
		case err != nil:
			postErr = errors.Join(postErr, fmt.Errorf("check mastodon status for %q: %w", post.Link, err))
		case alreadyPosted:
			log.Debugf("Skipping Mastodon: already posted %s", post.Link)
		default:
			err = mastodon.TootPostContext(ctx, *conf, tootContent, deliveryKey(post, "mastodon"))
			if err != nil {
				postErr = errors.Join(postErr, fmt.Errorf("post %q to mastodon: %w", post.Link, err))
				if isUpdate {
					gotify.LogFailure("Failed to toot updated post", err, conf)
				} else {
					gotify.LogFailure("Failed to toot new post", err, conf)
				}
			} else {
				gotify.LogSuccess(fmt.Sprintf("Successfully posted to Mastodon: %s", post.Title), conf)
				if markErr := db.MarkSitePosted(post.Link, "mastodon"); markErr != nil {
					postErr = errors.Join(postErr, fmt.Errorf("mark mastodon posted for %q: %w", post.Link, markErr))
				}
			}
		}
	}

	if siteMap["bluesky"] && conf.BlueskyHandle != "" && conf.BlueskyAppKey != "" {
		alreadyPosted, err := db.IsSitePosted(post.Link, "bluesky")
		switch {
		case err != nil:
			postErr = errors.Join(postErr, fmt.Errorf("check bluesky status for %q: %w", post.Link, err))
		case alreadyPosted:
			log.Debugf("Skipping Bluesky: already posted %s", post.Link)
		default:
			opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := bluesky.Post(opCtx, *conf, tootContent)
			cancel()
			if err != nil {
				postErr = errors.Join(postErr, fmt.Errorf("post %q to bluesky: %w", post.Link, err))
				gotify.LogFailure(fmt.Sprintf("Failed to post to Bluesky: %s", post.Title), err, conf)
			} else {
				gotify.LogSuccess(fmt.Sprintf("Successfully posted to Bluesky: %s", post.Title), conf)
				if markErr := db.MarkSitePosted(post.Link, "bluesky"); markErr != nil {
					postErr = errors.Join(postErr, fmt.Errorf("mark bluesky posted for %q: %w", post.Link, markErr))
				}
			}
		}
	}

	if siteMap["threads"] && conf.ThreadsToken != "" && conf.ThreadsClientID != "" && conf.ThreadsClientSecret != "" {
		alreadyPosted, err := db.IsSitePosted(post.Link, "threads")
		switch {
		case err != nil:
			postErr = errors.Join(postErr, fmt.Errorf("check threads status for %q: %w", post.Link, err))
		case alreadyPosted:
			log.Debugf("Skipping Threads: already posted %s", post.Link)
		default:
			opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := threads.Post(opCtx, *conf, tootContent)
			cancel()
			if err != nil {
				postErr = errors.Join(postErr, fmt.Errorf("post %q to threads: %w", post.Link, err))
				gotify.LogFailure(fmt.Sprintf("Failed to post to Threads: %s", post.Title), err, conf)
			} else {
				gotify.LogSuccess(fmt.Sprintf("Successfully posted to Threads: %s", post.Title), conf)
				if markErr := db.MarkSitePosted(post.Link, "threads"); markErr != nil {
					postErr = errors.Join(postErr, fmt.Errorf("mark threads posted for %q: %w", post.Link, markErr))
				}
			}
		}
	}

	complete, err := allEnabledSitesPosted(post.Link, enabledSites)
	if err != nil {
		postErr = errors.Join(postErr, err)
	} else if complete && isUpdate {
		postErr = errors.Join(postErr, db.MarkUpdateComplete(post.Link))
	}
	return postErr
}

func allEnabledSitesPosted(link string, sites []string) (bool, error) {
	if len(sites) == 0 {
		return false, errors.New("no social sites are enabled")
	}
	for _, site := range sites {
		posted, err := db.IsSitePosted(link, site)
		if err != nil {
			return false, fmt.Errorf("check %s status for %q: %w", site, link, err)
		}
		if !posted {
			return false, nil
		}
	}
	return true, nil
}

func deliveryKey(post rss.RSSItem, site string) string {
	return fmt.Sprintf("rss2socials-%x", rss.HashContent(site+"\x00"+post.Link+"\x00"+post.Content))
}
