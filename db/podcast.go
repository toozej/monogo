// Package db provides database models and data access functions.
package db

import (
	"time"
)

// Podcast is
type Podcast struct {
	Base
	Title string

	Summary string `gorm:"type:text"`

	Author string

	Image string

	URL string

	LastEpisode *time.Time

	PodcastItems []PodcastItem

	Tags []*Tag `gorm:"many2many:podcast_tags;"`

	DownloadedEpisodesCount  int `gorm:"-"`
	DownloadingEpisodesCount int `gorm:"-"`
	AllEpisodesCount         int `gorm:"-"`

	DownloadedEpisodesSize  int64 `gorm:"-"`
	DownloadingEpisodesSize int64 `gorm:"-"`
	AllEpisodesSize         int64 `gorm:"-"`

	IsPaused bool `gorm:"default:false"`
}

// PodcastItem is
type PodcastItem struct {
	Base
	PubDate        time.Time
	BookmarkDate   time.Time
	DownloadDate   time.Time
	FileURL        string
	PodcastID      string
	LocalImage     string
	Summary        string `gorm:"type:text"`
	Title          string
	GUID           string
	Image          string
	EpisodeType    string
	DownloadPath   string
	Podcast        Podcast
	DownloadStatus DownloadStatus `gorm:"default:0"`
	Duration       int
	FileSize       int64
	IsPlayed       bool `gorm:"default:false"`
}

// DownloadStatus represents the download state of a podcast episode.
type DownloadStatus int

// Download status constants.
const (
	// NotDownloaded indicates the episode has not been downloaded yet.
	NotDownloaded DownloadStatus = iota
	// Downloading indicates the episode is currently being downloaded.
	Downloading
	// Downloaded indicates the episode has been successfully downloaded.
	Downloaded
	// Deleted indicates the episode file has been removed.
	Deleted
)

// Setting represents setting data.
type Setting struct {
	Base
	UserAgent                     string
	BaseURL                       string
	InitialDownloadCount          int  `gorm:"default:5"`
	MaxDownloadConcurrency        int  `gorm:"default:5"`
	DarkMode                      bool `gorm:"default:false"`
	AppendEpisodeNumberToFileName bool `gorm:"default:false"`
	DownloadEpisodeImages         bool `gorm:"default:false"`
	GenerateNFOFile               bool `gorm:"default:false"`
	DontDownloadDeletedFromDisk   bool `gorm:"default:false"`
	AppendDateToFileName          bool `gorm:"default:false"`
	AutoDownload                  bool `gorm:"default:true"`
	DownloadOnAdd                 bool `gorm:"default:true"`
}

// Migration represents migration data.
type Migration struct {
	Base
	Date time.Time
	Name string
}

// JobLock represents job lock data.
type JobLock struct {
	Base
	Date     time.Time
	Name     string
	Duration int
}

// Tag represents tag data.
type Tag struct {
	Base
	Label       string
	Description string     `gorm:"type:text"`
	Podcasts    []*Podcast `gorm:"many2many:podcast_tags;"`
}

// IsLocked returns true if the job lock is currently active.
func (lock *JobLock) IsLocked() bool {
	return lock != nil && lock.Date != time.Time{}
}

// PodcastItemStatsModel represents podcast item stats model data.
type PodcastItemStatsModel struct {
	PodcastID      string
	DownloadStatus DownloadStatus
	Count          int
	Size           int64
}

// PodcastItemDiskStatsModel represents podcast item disk stats model data.
type PodcastItemDiskStatsModel struct {
	DownloadStatus DownloadStatus
	Count          int
	Size           int64
}

// PodcastItemConsolidateDiskStatsModel represents podcast item consolidate disk stats model data.
type PodcastItemConsolidateDiskStatsModel struct {
	Downloaded      int64
	Downloading     int64
	NotDownloaded   int64
	Deleted         int64
	PendingDownload int64
}
