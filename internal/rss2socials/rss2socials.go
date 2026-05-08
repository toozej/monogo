// Package rss2socials provides the main logic for monitoring RSS feeds and posting updates to Mastodon, Bluesky, and Threads.
// It handles configuration, feed checking, post processing, and integration with other components.
package rss2socials

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/rss2socials/internal/bluesky"
	"github.com/toozej/rss2socials/internal/db"
	"github.com/toozej/rss2socials/internal/gotify"
	"github.com/toozej/rss2socials/internal/mastodon"
	"github.com/toozej/rss2socials/internal/rss"
	"github.com/toozej/rss2socials/internal/threads"
	"github.com/toozej/rss2socials/pkg/config"
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

func Run(conf config.Config) {
	if conf.FeedURL == "" {
		log.Fatal("RSS feed URL is required")
	}

	if conf.Interval <= 0 {
		log.Error("Interval must be a positive integer")
		conf.Interval = 60
	}

	db.InitDB(conf.DBPath)
	defer db.CloseDB()

	var startupTime string
	firstCycle := true

	for {
		posts, err := rss.CheckRSSFeed(conf.FeedURL)
		if err != nil {
			log.Printf("Error fetching RSS feed: %v", err)
			continue
		}

		if firstCycle {
			startupTime = time.Now().Format(time.RFC3339)
			if conf.PostNewEntriesOnly && !db.IsFirstCycle() {
				log.Info("PostNewEntriesOnly enabled: skipping posts already in DB from first cycle")
			}
			firstCycle = false
		}

		if conf.ShortRun && len(posts) > 3 {
			log.Info("Short run mode: processing only the 3 most recent items")
			posts = posts[:3]
		}

		for _, post := range posts {
			if shouldSkipPost(post, conf.SkipPrefixCategories) {
				log.Debugf("Skipping post %s: matches skip prefix category", post.Title)
				continue
			}

			if conf.Category != "" {
				lastSegment := path.Base(post.Link)
				if !strings.Contains(lastSegment, conf.Category) {
					log.Debugf("Skipping post %s: category filter '%s' not in URL segment '%s'", post.Title, conf.Category, lastSegment)
					continue
				}
			}

			skipIfExisting := conf.PostNewEntriesOnly && db.IsFirstCycle()
			handlePost(post, &conf, startupTime, skipIfExisting)
		}

		if conf.ShortRun {
			log.Info("Short run mode complete, exiting")
			return
		}

		time.Sleep(time.Duration(conf.Interval) * time.Minute)
	}
}

func handlePost(post rss.RSSItem, conf *config.Config, startupTime string, skipIfExisting bool) {
	exists, updated, err := db.HasPostChanged(post.Link, post.Content)
	if err != nil {
		log.Error("Database error: ", err)
		return
	}

	if skipIfExisting && exists && !updated {
		log.Debugf("Skipping existing post %s: PostNewEntriesOnly enabled on first cycle", post.Link)
		return
	}

	var tootContent string
	var isUpdate bool

	switch {
	case exists && updated:
		log.Printf("Post has been updated: %s", post.Title)
		tootContent = fmt.Sprintf("Updated post: %s", post.Link)
		isUpdate = true
	case !exists:
		tootContent = mastodon.GetTootContent(post)
		isUpdate = false
	case exists && !updated:
		if sitePosted, err := db.IsSitePosted(post.Link, "mastodon"); err != nil || sitePosted {
			if sitePosted, err := db.IsSitePosted(post.Link, "bluesky"); err != nil || sitePosted {
				if sitePosted, err := db.IsSitePosted(post.Link, "threads"); err != nil || sitePosted {
					return
				}
			}
		}
		tootContent = mastodon.GetTootContent(post)
		isUpdate = false
	default:
		return
	}

	if err := db.StoreTootedPost(post.Link, post.Content, startupTime); err != nil {
		log.Error("Storing post in database failed: ", err)
		return
	}

	enabledSites := conf.EnabledSites()
	siteMap := make(map[string]bool, len(enabledSites))
	for _, s := range enabledSites {
		siteMap[s] = true
	}

	if siteMap["mastodon"] {
		alreadyPosted, err := db.IsSitePosted(post.Link, "mastodon")
		switch {
		case err != nil:
			log.Error("Error checking mastodon post status: ", err)
		case alreadyPosted && !isUpdate:
			log.Debugf("Skipping Mastodon: already posted %s", post.Link)
		default:
			err = mastodon.TootPost(*conf, tootContent)
			if err != nil {
				if isUpdate {
					gotify.LogFailure("Failed to toot updated post", err, conf)
				} else {
					gotify.LogFailure("Failed to toot new post", err, conf)
				}
			} else {
				gotify.LogSuccess(fmt.Sprintf("Successfully posted to Mastodon: %s", post.Title), conf)
				if markErr := db.MarkSitePosted(post.Link, "mastodon"); markErr != nil {
					log.Error("Failed to mark mastodon as posted: ", markErr)
				}
			}
		}
	}

	if siteMap["bluesky"] && conf.BlueskyHandle != "" && conf.BlueskyAppKey != "" {
		alreadyPosted, err := db.IsSitePosted(post.Link, "bluesky")
		switch {
		case err != nil:
			log.Error("Error checking bluesky post status: ", err)
		case alreadyPosted && !isUpdate:
			log.Debugf("Skipping Bluesky: already posted %s", post.Link)
		default:
			if err := bluesky.Post(context.Background(), *conf, tootContent); err != nil {
				gotify.LogFailure(fmt.Sprintf("Failed to post to Bluesky: %s", post.Title), err, conf)
			} else {
				gotify.LogSuccess(fmt.Sprintf("Successfully posted to Bluesky: %s", post.Title), conf)
				if markErr := db.MarkSitePosted(post.Link, "bluesky"); markErr != nil {
					log.Error("Failed to mark bluesky as posted: ", markErr)
				}
			}
		}
	}

	if siteMap["threads"] && conf.ThreadsToken != "" && conf.ThreadsClientID != "" && conf.ThreadsClientSecret != "" {
		alreadyPosted, err := db.IsSitePosted(post.Link, "threads")
		switch {
		case err != nil:
			log.Error("Error checking threads post status: ", err)
		case alreadyPosted && !isUpdate:
			log.Debugf("Skipping Threads: already posted %s", post.Link)
		default:
			if err := threads.Post(context.Background(), *conf, tootContent); err != nil {
				gotify.LogFailure(fmt.Sprintf("Failed to post to Threads: %s", post.Title), err, conf)
			} else {
				gotify.LogSuccess(fmt.Sprintf("Successfully posted to Threads: %s", post.Title), conf)
				if markErr := db.MarkSitePosted(post.Link, "threads"); markErr != nil {
					log.Error("Failed to mark threads as posted: ", markErr)
				}
			}
		}
	}
}
