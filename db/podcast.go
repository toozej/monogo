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

type DownloadStatus int

const (
	NotDownloaded DownloadStatus = iota
	Downloading
	Downloaded
	Deleted
)

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
type Migration struct {
	Base
	Date time.Time
	Name string
}

type JobLock struct {
	Base
	Date     time.Time
	Name     string
	Duration int
}

type Tag struct {
	Base
	Label       string
	Description string     `gorm:"type:text"`
	Podcasts    []*Podcast `gorm:"many2many:podcast_tags;"`
}

func (lock *JobLock) IsLocked() bool {
	return lock != nil && lock.Date != time.Time{}
}

type PodcastItemStatsModel struct {
	PodcastID      string
	DownloadStatus DownloadStatus
	Count          int
	Size           int64
}

type PodcastItemDiskStatsModel struct {
	DownloadStatus DownloadStatus
	Count          int
	Size           int64
}

type PodcastItemConsolidateDiskStatsModel struct {
	Downloaded      int64
	Downloading     int64
	NotDownloaded   int64
	Deleted         int64
	PendingDownload int64
}
