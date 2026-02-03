// Package model defines data structures for external API responses and RSS feeds.
package model

import "math"

// Pagination represents pagination data.
type Pagination struct {
	Page         int `uri:"page" query:"page" json:"page" form:"page" default:"1"`
	Count        int `uri:"count" query:"count" json:"count" form:"count" default:"20"`
	NextPage     int `uri:"nextPage" query:"nextPage" json:"nextPage" form:"nextPage"`
	PreviousPage int `uri:"previousPage" query:"previousPage" json:"previousPage" form:"previousPage"`
	TotalCount   int `uri:"totalCount" query:"totalCount" json:"totalCount" form:"totalCount"`
	TotalPages   int `uri:"totalPages" query:"totalPages" json:"totalPages" form:"totalPages"`
}

// EpisodeSort represents episode sorting options.
type EpisodeSort string

const (
	// ReleaseAsc sorts episodes by release date in ascending order.
	ReleaseAsc EpisodeSort = "release_asc"
	// ReleaseDesc sorts episodes by release date in descending order.
	ReleaseDesc EpisodeSort = "release_desc"
	// DurationAsc sorts episodes by duration in ascending order.
	DurationAsc EpisodeSort = "duration_asc"
	// DurationDesc sorts episodes by duration in descending order.
	DurationDesc EpisodeSort = "duration_desc"
)

// EpisodesFilter represents episodes filter data.
type EpisodesFilter struct {
	IsDownloaded *string     `uri:"isDownloaded" query:"isDownloaded" json:"isDownloaded" form:"isDownloaded"`
	IsPlayed     *string     `uri:"isPlayed" query:"isPlayed" json:"isPlayed" form:"isPlayed"`
	Sorting      EpisodeSort `uri:"sorting" query:"sorting" json:"sorting" form:"sorting"`
	Q            string      `uri:"q" query:"q" json:"q" form:"q"`
	TagIDs       []string    `uri:"tagIDs" query:"tagIds[]" json:"tagIDs" form:"tagIds[]"`
	PodcastIDs   []string    `uri:"podcastIDs" query:"podcastIDs[]" json:"podcastIDs" form:"podcastIDs[]"`
	Pagination
}

// VerifyPaginationValues sets default values for pagination parameters.
func (filter *EpisodesFilter) VerifyPaginationValues() {
	if filter.Count == 0 {
		filter.Count = 20
	}
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.Sorting == "" {
		filter.Sorting = ReleaseDesc
	}
}

// SetCounts calculates and sets pagination metadata based on total count.
func (filter *EpisodesFilter) SetCounts(totalCount int64) {
	totalPages := int(math.Ceil(float64(totalCount) / float64(filter.Count)))
	nextPage, previousPage := 0, 0
	if filter.Page < totalPages {
		nextPage = filter.Page + 1
	}
	if filter.Page > 1 {
		previousPage = filter.Page - 1
	}
	filter.NextPage = nextPage
	filter.PreviousPage = previousPage
	filter.TotalCount = int(totalCount)
	filter.TotalPages = totalPages
}
