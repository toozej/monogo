// Package database provides abstractions for database operations.
// This package defines repository interfaces that enable dependency injection
// and testing with mock implementations.
package database

import (
	"time"

	"github.com/akhilrex/podgrab/db"
	"github.com/akhilrex/podgrab/model"
)

// Repository defines the interface for all database operations.
// This abstraction enables dependency injection and testing with mocks.
type Repository interface {
	// Podcast operations
	GetPodcastByURL(url string, podcast *db.Podcast) error
	GetPodcastsByURLList(urls []string, podcasts *[]db.Podcast) error
	GetAllPodcasts(podcasts *[]db.Podcast, sorting string) error
	GetPodcastByID(id string, podcast *db.Podcast) error
	GetPodcastByTitleAndAuthor(title string, author string, podcast *db.Podcast) error
	CreatePodcast(podcast *db.Podcast) error
	UpdatePodcast(podcast *db.Podcast) error
	DeletePodcastByID(id string) error
	UpdateLastEpisodeDateForPodcast(podcastID string, lastEpisode time.Time) error
	ForceSetLastEpisodeDate(podcastID string)
	TogglePodcastPauseStatus(podcastID string, isPaused bool) error
	SetAllEpisodesToDownload(podcastID string) error

	// PodcastItem operations
	GetAllPodcastItems(podcasts *[]db.PodcastItem) error
	GetAllPodcastItemsWithoutSize() (*[]db.PodcastItem, error)
	GetPaginatedPodcastItemsNew(queryModel model.EpisodesFilter) (*[]db.PodcastItem, int64, error)
	GetPaginatedPodcastItems(page int, count int, downloadedOnly *bool, playedOnly *bool, fromDate time.Time, podcasts *[]db.PodcastItem, total *int64) error
	GetPodcastItemByID(id string, podcastItem *db.PodcastItem) error
	GetAllPodcastItemsByPodcastID(podcastID string, podcastItems *[]db.PodcastItem) error
	GetAllPodcastItemsByPodcastIDs(podcastIDs []string, podcastItems *[]db.PodcastItem) error
	GetAllPodcastItemsByIDs(podcastItemIDs []string) (*[]db.PodcastItem, error)
	GetPodcastItemsByPodcastIDAndGUIDs(podcastID string, guids []string) (*[]db.PodcastItem, error)
	GetPodcastItemByPodcastIDAndGUID(podcastID string, guid string, podcastItem *db.PodcastItem) error
	GetAllPodcastItemsWithoutImage() (*[]db.PodcastItem, error)
	GetAllPodcastItemsToBeDownloaded() (*[]db.PodcastItem, error)
	GetAllPodcastItemsAlreadyDownloaded() (*[]db.PodcastItem, error)
	CreatePodcastItem(podcastItem *db.PodcastItem) error
	UpdatePodcastItem(podcastItem *db.PodcastItem) error
	UpdatePodcastItemFileSize(podcastItemID string, size int64) error
	DeletePodcastItemByID(id string) error
	GetEpisodeNumber(podcastItemID, podcastID string) (int, error)

	// Stats operations
	GetPodcastEpisodeStats() (*[]db.PodcastItemStatsModel, error)
	GetPodcastEpisodeDiskStats() (db.PodcastItemConsolidateDiskStatsModel, error)

	// Tag operations
	GetAllTags(sorting string) (*[]db.Tag, error)
	GetPaginatedTags(page int, count int, tags *[]db.Tag, total *int64) error
	GetTagByID(id string) (*db.Tag, error)
	GetTagsByIDs(ids []string) (*[]db.Tag, error)
	GetTagByLabel(label string) (*db.Tag, error)
	CreateTag(tag *db.Tag) error
	UpdateTag(tag *db.Tag) error
	DeleteTagByID(id string) error
	AddTagToPodcast(id, tagID string) error
	RemoveTagFromPodcast(id, tagID string) error
	UntagAllByTagID(tagID string) error

	// Settings operations
	GetOrCreateSetting() *db.Setting
	UpdateSettings(setting *db.Setting) error

	// Job lock operations
	GetLock(name string) *db.JobLock
	Lock(name string, duration int)
	Unlock(name string)
	UnlockMissedJobs()
}
