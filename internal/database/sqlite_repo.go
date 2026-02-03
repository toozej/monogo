package database

import (
	"time"

	"github.com/akhilrex/podgrab/db"
	"github.com/akhilrex/podgrab/model"
	"gorm.io/gorm"
)

// SQLiteRepository implements Repository interface using GORM with SQLite.
// This is a thin wrapper around the existing db package functions to enable
// dependency injection and testing.
type SQLiteRepository struct {
	database *gorm.DB
}

// NewSQLiteRepository creates a new SQLite repository instance.
func NewSQLiteRepository(database *gorm.DB) *SQLiteRepository {
	return &SQLiteRepository{database: database}
}

// NewDefaultSQLiteRepository creates a repository using the global DB connection.
// This maintains backwards compatibility with existing code.
func NewDefaultSQLiteRepository() *SQLiteRepository {
	return &SQLiteRepository{database: db.DB}
}

// Podcast operations

// GetPodcastByURL retrieves a podcast by its RSS feed URL.
func (r *SQLiteRepository) GetPodcastByURL(url string, podcast *db.Podcast) error {
	return db.GetPodcastByURL(url, podcast)
}

// GetPodcastsByURLList retrieves multiple podcasts by their RSS feed URLs.
func (r *SQLiteRepository) GetPodcastsByURLList(urls []string, podcasts *[]db.Podcast) error {
	return db.GetPodcastsByURLList(urls, podcasts)
}

// GetAllPodcasts retrieves all podcasts with optional sorting.
func (r *SQLiteRepository) GetAllPodcasts(podcasts *[]db.Podcast, sorting string) error {
	return db.GetAllPodcasts(podcasts, sorting)
}

// GetPodcastByID retrieves a podcast by its ID.
//
//nolint:revive // Method name matches existing db package convention
func (r *SQLiteRepository) GetPodcastByID(id string, podcast *db.Podcast) error {
	return db.GetPodcastByID(id, podcast)
}

// GetPodcastByTitleAndAuthor retrieves a podcast by its title and author.
func (r *SQLiteRepository) GetPodcastByTitleAndAuthor(title string, author string, podcast *db.Podcast) error {
	return db.GetPodcastByTitleAndAuthor(title, author, podcast)
}

// CreatePodcast creates a new podcast record.
func (r *SQLiteRepository) CreatePodcast(podcast *db.Podcast) error {
	return db.CreatePodcast(podcast)
}

// UpdatePodcast updates an existing podcast record.
func (r *SQLiteRepository) UpdatePodcast(podcast *db.Podcast) error {
	return db.UpdatePodcast(podcast)
}

// DeletePodcastByID deletes a podcast by its ID.
//
//nolint:revive // Method name matches existing db package convention
func (r *SQLiteRepository) DeletePodcastByID(id string) error {
	return db.DeletePodcastByID(id)
}

// UpdateLastEpisodeDateForPodcast updates the last episode date for a podcast.
func (r *SQLiteRepository) UpdateLastEpisodeDateForPodcast(podcastID string, lastEpisode time.Time) error {
	return db.UpdateLastEpisodeDateForPodcast(podcastID, lastEpisode)
}

// ForceSetLastEpisodeDate forces the last episode date to be recalculated.
func (r *SQLiteRepository) ForceSetLastEpisodeDate(podcastID string) {
	db.ForceSetLastEpisodeDate(podcastID)
}

// TogglePodcastPauseStatus toggles the pause status of a podcast.
func (r *SQLiteRepository) TogglePodcastPauseStatus(podcastID string, isPaused bool) error {
	return db.TogglePodcastPauseStatus(podcastID, isPaused)
}

// SetAllEpisodesToDownload marks all deleted episodes as ready for download.
func (r *SQLiteRepository) SetAllEpisodesToDownload(podcastID string) error {
	return db.SetAllEpisodesToDownload(podcastID)
}

// PodcastItem operations

// GetAllPodcastItems retrieves all podcast episodes.
func (r *SQLiteRepository) GetAllPodcastItems(podcasts *[]db.PodcastItem) error {
	return db.GetAllPodcastItems(podcasts)
}

// GetAllPodcastItemsWithoutSize retrieves episodes without file size information.
func (r *SQLiteRepository) GetAllPodcastItemsWithoutSize() (*[]db.PodcastItem, error) {
	return db.GetAllPodcastItemsWithoutSize()
}

// GetPaginatedPodcastItemsNew retrieves paginated episodes with advanced filtering.
func (r *SQLiteRepository) GetPaginatedPodcastItemsNew(queryModel model.EpisodesFilter) (*[]db.PodcastItem, int64, error) {
	return db.GetPaginatedPodcastItemsNew(queryModel)
}

// GetPaginatedPodcastItems retrieves paginated episodes with basic filtering.
func (r *SQLiteRepository) GetPaginatedPodcastItems(page int, count int, downloadedOnly *bool, playedOnly *bool, fromDate time.Time, podcasts *[]db.PodcastItem, total *int64) error {
	return db.GetPaginatedPodcastItems(page, count, downloadedOnly, playedOnly, fromDate, podcasts, total)
}

// GetPodcastItemByID retrieves a podcast episode by its ID.
//
//nolint:revive // Method name matches existing db package convention
func (r *SQLiteRepository) GetPodcastItemByID(id string, podcastItem *db.PodcastItem) error {
	return db.GetPodcastItemByID(id, podcastItem)
}

// GetAllPodcastItemsByPodcastID retrieves all episodes for a specific podcast.
func (r *SQLiteRepository) GetAllPodcastItemsByPodcastID(podcastID string, podcastItems *[]db.PodcastItem) error {
	return db.GetAllPodcastItemsByPodcastID(podcastID, podcastItems)
}

// GetAllPodcastItemsByPodcastIDs retrieves episodes for multiple podcasts.
func (r *SQLiteRepository) GetAllPodcastItemsByPodcastIDs(podcastIDs []string, podcastItems *[]db.PodcastItem) error {
	return db.GetAllPodcastItemsByPodcastIDs(podcastIDs, podcastItems)
}

// GetAllPodcastItemsByIDs retrieves episodes by their IDs in specified order.
//
//nolint:revive // Method name matches existing db package convention
func (r *SQLiteRepository) GetAllPodcastItemsByIDs(podcastItemIDs []string) (*[]db.PodcastItem, error) {
	return db.GetAllPodcastItemsByIDs(podcastItemIDs)
}

// GetPodcastItemsByPodcastIDAndGUIDs retrieves episodes by podcast ID and GUIDs.
func (r *SQLiteRepository) GetPodcastItemsByPodcastIDAndGUIDs(podcastID string, guids []string) (*[]db.PodcastItem, error) {
	return db.GetPodcastItemsByPodcastIDAndGUIDs(podcastID, guids)
}

// GetPodcastItemByPodcastIDAndGUID retrieves an episode by podcast ID and GUID.
func (r *SQLiteRepository) GetPodcastItemByPodcastIDAndGUID(podcastID string, guid string, podcastItem *db.PodcastItem) error {
	return db.GetPodcastItemByPodcastIDAndGUID(podcastID, guid, podcastItem)
}

// GetAllPodcastItemsWithoutImage retrieves episodes without downloaded images.
func (r *SQLiteRepository) GetAllPodcastItemsWithoutImage() (*[]db.PodcastItem, error) {
	return db.GetAllPodcastItemsWithoutImage()
}

// GetAllPodcastItemsToBeDownloaded retrieves episodes queued for download.
func (r *SQLiteRepository) GetAllPodcastItemsToBeDownloaded() (*[]db.PodcastItem, error) {
	return db.GetAllPodcastItemsToBeDownloaded()
}

// GetAllPodcastItemsAlreadyDownloaded retrieves all downloaded episodes.
func (r *SQLiteRepository) GetAllPodcastItemsAlreadyDownloaded() (*[]db.PodcastItem, error) {
	return db.GetAllPodcastItemsAlreadyDownloaded()
}

// CreatePodcastItem creates a new podcast episode record.
func (r *SQLiteRepository) CreatePodcastItem(podcastItem *db.PodcastItem) error {
	return db.CreatePodcastItem(podcastItem)
}

// UpdatePodcastItem updates an existing podcast episode record.
func (r *SQLiteRepository) UpdatePodcastItem(podcastItem *db.PodcastItem) error {
	return db.UpdatePodcastItem(podcastItem)
}

// UpdatePodcastItemFileSize updates the file size of an episode.
func (r *SQLiteRepository) UpdatePodcastItemFileSize(podcastItemID string, size int64) error {
	return db.UpdatePodcastItemFileSize(podcastItemID, size)
}

// DeletePodcastItemByID deletes an episode by its ID.
//
//nolint:revive // Method name matches existing db package convention
func (r *SQLiteRepository) DeletePodcastItemByID(id string) error {
	return db.DeletePodcastItemByID(id)
}

// GetEpisodeNumber retrieves the sequential episode number within a podcast.
func (r *SQLiteRepository) GetEpisodeNumber(podcastItemID, podcastID string) (int, error) {
	return db.GetEpisodeNumber(podcastItemID, podcastID)
}

// Stats operations

// GetPodcastEpisodeStats retrieves episode statistics grouped by podcast and download status.
func (r *SQLiteRepository) GetPodcastEpisodeStats() (*[]db.PodcastItemStatsModel, error) {
	return db.GetPodcastEpisodeStats()
}

// GetPodcastEpisodeDiskStats retrieves consolidated disk usage statistics.
func (r *SQLiteRepository) GetPodcastEpisodeDiskStats() (db.PodcastItemConsolidateDiskStatsModel, error) {
	return db.GetPodcastEpisodeDiskStats()
}

// Tag operations

// GetAllTags retrieves all tags with optional sorting.
func (r *SQLiteRepository) GetAllTags(sorting string) (*[]db.Tag, error) {
	return db.GetAllTags(sorting)
}

// GetPaginatedTags retrieves paginated tags.
func (r *SQLiteRepository) GetPaginatedTags(page int, count int, tags *[]db.Tag, total *int64) error {
	return db.GetPaginatedTags(page, count, tags, total)
}

// GetTagByID retrieves a tag by its ID.
//
//nolint:revive // Method name matches existing db package convention
func (r *SQLiteRepository) GetTagByID(id string) (*db.Tag, error) {
	return db.GetTagByID(id)
}

// GetTagsByIDs retrieves multiple tags by their IDs.
//
//nolint:revive // Method name matches existing db package convention
func (r *SQLiteRepository) GetTagsByIDs(ids []string) (*[]db.Tag, error) {
	return db.GetTagsByIDs(ids)
}

// GetTagByLabel retrieves a tag by its label.
func (r *SQLiteRepository) GetTagByLabel(label string) (*db.Tag, error) {
	return db.GetTagByLabel(label)
}

// CreateTag creates a new tag record.
func (r *SQLiteRepository) CreateTag(tag *db.Tag) error {
	return db.CreateTag(tag)
}

// UpdateTag updates an existing tag record.
func (r *SQLiteRepository) UpdateTag(tag *db.Tag) error {
	return db.UpdateTag(tag)
}

// DeleteTagByID deletes a tag by its ID.
//
//nolint:revive // Method name matches existing db package convention
func (r *SQLiteRepository) DeleteTagByID(id string) error {
	return db.DeleteTagByID(id)
}

// AddTagToPodcast associates a tag with a podcast.
func (r *SQLiteRepository) AddTagToPodcast(id, tagID string) error {
	return db.AddTagToPodcast(id, tagID)
}

// RemoveTagFromPodcast removes a tag association from a podcast.
func (r *SQLiteRepository) RemoveTagFromPodcast(id, tagID string) error {
	return db.RemoveTagFromPodcast(id, tagID)
}

// UntagAllByTagID removes all podcast associations for a tag.
func (r *SQLiteRepository) UntagAllByTagID(tagID string) error {
	return db.UntagAllByTagID(tagID)
}

// Settings operations

// GetOrCreateSetting retrieves or creates the application settings record.
func (r *SQLiteRepository) GetOrCreateSetting() *db.Setting {
	return db.GetOrCreateSetting()
}

// UpdateSettings updates the application settings record.
func (r *SQLiteRepository) UpdateSettings(setting *db.Setting) error {
	return db.UpdateSettings(setting)
}

// Job lock operations

// GetLock retrieves a job lock by name.
func (r *SQLiteRepository) GetLock(name string) *db.JobLock {
	return db.GetLock(name)
}

// Lock acquires a lock for a job with specified duration.
func (r *SQLiteRepository) Lock(name string, duration int) {
	db.Lock(name, duration)
}

// Unlock releases a lock for a job.
func (r *SQLiteRepository) Unlock(name string) {
	db.Unlock(name)
}

// UnlockMissedJobs releases locks for jobs that have exceeded their duration.
func (r *SQLiteRepository) UnlockMissedJobs() {
	db.UnlockMissedJobs()
}
