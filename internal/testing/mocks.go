//nolint:revive // Mock implementation - many trivial method wrappers with intentionally simplified signatures
package testing

import (
	"errors"
	"time"

	"github.com/akhilrex/podgrab/db"
	"github.com/akhilrex/podgrab/model"
)

// MockRepository is a mock implementation of database.Repository for testing.
// It stores data in memory and allows tests to control behavior and verify calls.
type MockRepository struct {
	GetPodcastByURLError    error
	GetOrCreateSettingError error
	GetAllPodcastItemsError error
	CreatePodcastItemError  error
	GetPodcastItemByIdError error
	UpdatePodcastError      error
	CreatePodcastError      error
	PodcastItems            map[string]*db.PodcastItem
	Tags                    map[string]*db.Tag
	Settings                *db.Setting
	JobLocks                map[string]*db.JobLock
	Podcasts                map[string]*db.Podcast
	GetPodcastByURLCalls    int
	LockCalls               int
	UnlockCalls             int
	GetOrCreateSettingCalls int
	GetAllPodcastItemsCalls int
	UpdatePodcastItemCalls  int
	CreatePodcastItemCalls  int
	DeletePodcastByIdCalls  int
	UpdatePodcastCalls      int
	CreatePodcastCalls      int
}

// NewMockRepository creates a new mock repository with empty data stores.
func NewMockRepository() *MockRepository {
	return &MockRepository{
		Podcasts:     make(map[string]*db.Podcast),
		PodcastItems: make(map[string]*db.PodcastItem),
		Tags:         make(map[string]*db.Tag),
		JobLocks:     make(map[string]*db.JobLock),
		Settings: &db.Setting{
			DownloadOnAdd:          true,
			InitialDownloadCount:   5,
			AutoDownload:           true,
			MaxDownloadConcurrency: 5,
		},
	}
}

// Reset clears all data and resets call counters.
func (m *MockRepository) Reset() {
	m.Podcasts = make(map[string]*db.Podcast)
	m.PodcastItems = make(map[string]*db.PodcastItem)
	m.Tags = make(map[string]*db.Tag)
	m.JobLocks = make(map[string]*db.JobLock)

	m.GetPodcastByURLCalls = 0
	m.CreatePodcastCalls = 0
	m.UpdatePodcastCalls = 0
	m.DeletePodcastByIdCalls = 0
	m.CreatePodcastItemCalls = 0
	m.UpdatePodcastItemCalls = 0
	m.GetAllPodcastItemsCalls = 0
	m.GetOrCreateSettingCalls = 0
	m.LockCalls = 0
	m.UnlockCalls = 0

	m.GetPodcastByURLError = nil
	m.CreatePodcastError = nil
	m.UpdatePodcastError = nil
	m.GetPodcastItemByIdError = nil
	m.CreatePodcastItemError = nil
	m.GetAllPodcastItemsError = nil
}

// Podcast operations

func (m *MockRepository) GetPodcastByURL(url string, podcast *db.Podcast) error {
	m.GetPodcastByURLCalls++

	if m.GetPodcastByURLError != nil {
		return m.GetPodcastByURLError
	}

	for _, p := range m.Podcasts {
		if p.URL == url {
			*podcast = *p
			return nil
		}
	}

	return errors.New("podcast not found")
}

func (m *MockRepository) GetPodcastsByURLList(urls []string, podcasts *[]db.Podcast) error {
	*podcasts = []db.Podcast{}
	for _, url := range urls {
		for _, p := range m.Podcasts {
			if p.URL == url {
				*podcasts = append(*podcasts, *p)
			}
		}
	}
	return nil
}

func (m *MockRepository) GetAllPodcasts(podcasts *[]db.Podcast, sorting string) error {
	*podcasts = []db.Podcast{}
	for _, p := range m.Podcasts {
		*podcasts = append(*podcasts, *p)
	}
	return nil
}

func (m *MockRepository) GetPodcastByID(id string, podcast *db.Podcast) error {
	if p, exists := m.Podcasts[id]; exists {
		*podcast = *p
		return nil
	}
	return errors.New("podcast not found")
}

func (m *MockRepository) GetPodcastByTitleAndAuthor(title string, author string, podcast *db.Podcast) error {
	for _, p := range m.Podcasts {
		if p.Title == title && p.Author == author {
			*podcast = *p
			return nil
		}
	}
	return errors.New("podcast not found")
}

func (m *MockRepository) CreatePodcast(podcast *db.Podcast) error {
	m.CreatePodcastCalls++

	if m.CreatePodcastError != nil {
		return m.CreatePodcastError
	}

	if podcast.ID == "" {
		podcast.ID = generateID()
	}
	m.Podcasts[podcast.ID] = podcast
	return nil
}

func (m *MockRepository) UpdatePodcast(podcast *db.Podcast) error {
	m.UpdatePodcastCalls++

	if m.UpdatePodcastError != nil {
		return m.UpdatePodcastError
	}

	if _, exists := m.Podcasts[podcast.ID]; !exists {
		return errors.New("podcast not found")
	}
	m.Podcasts[podcast.ID] = podcast
	return nil
}

func (m *MockRepository) DeletePodcastByID(id string) error {
	m.DeletePodcastByIdCalls++
	delete(m.Podcasts, id)
	return nil
}

func (m *MockRepository) UpdateLastEpisodeDateForPodcast(podcastID string, lastEpisode time.Time) error {
	if p, exists := m.Podcasts[podcastID]; exists {
		p.LastEpisode = &lastEpisode
		return nil
	}
	return errors.New("podcast not found")
}

func (m *MockRepository) ForceSetLastEpisodeDate(podcastID string) {
	// Mock implementation - no-op
}

func (m *MockRepository) TogglePodcastPauseStatus(podcastID string, isPaused bool) error {
	if p, exists := m.Podcasts[podcastID]; exists {
		p.IsPaused = isPaused
		return nil
	}
	return errors.New("podcast not found")
}

func (m *MockRepository) SetAllEpisodesToDownload(podcastID string) error {
	for _, item := range m.PodcastItems {
		if item.PodcastID == podcastID && item.DownloadStatus == db.Deleted {
			item.DownloadStatus = db.NotDownloaded
		}
	}
	return nil
}

// PodcastItem operations

func (m *MockRepository) GetAllPodcastItems(podcasts *[]db.PodcastItem) error {
	m.GetAllPodcastItemsCalls++

	if m.GetAllPodcastItemsError != nil {
		return m.GetAllPodcastItemsError
	}

	*podcasts = []db.PodcastItem{}
	for _, item := range m.PodcastItems {
		*podcasts = append(*podcasts, *item)
	}
	return nil
}

func (m *MockRepository) GetAllPodcastItemsWithoutSize() (*[]db.PodcastItem, error) {
	items := []db.PodcastItem{}
	for _, item := range m.PodcastItems {
		if item.FileSize <= 0 {
			items = append(items, *item)
		}
	}
	return &items, nil
}

func (m *MockRepository) GetPaginatedPodcastItemsNew(queryModel model.EpisodesFilter) (*[]db.PodcastItem, int64, error) {
	// Simplified mock - returns all items
	items := make([]db.PodcastItem, 0, len(m.PodcastItems))
	for _, item := range m.PodcastItems {
		items = append(items, *item)
	}
	return &items, int64(len(items)), nil
}

func (m *MockRepository) GetPaginatedPodcastItems(page int, count int, downloadedOnly *bool, playedOnly *bool, fromDate time.Time, podcasts *[]db.PodcastItem, total *int64) error {
	// Simplified mock implementation
	*podcasts = []db.PodcastItem{}
	for _, item := range m.PodcastItems {
		*podcasts = append(*podcasts, *item)
	}
	*total = int64(len(m.PodcastItems))
	return nil
}

func (m *MockRepository) GetPodcastItemByID(id string, podcastItem *db.PodcastItem) error {
	if m.GetPodcastItemByIdError != nil {
		return m.GetPodcastItemByIdError
	}

	if item, exists := m.PodcastItems[id]; exists {
		*podcastItem = *item
		return nil
	}
	return errors.New("podcast item not found")
}

func (m *MockRepository) GetAllPodcastItemsByPodcastID(podcastID string, podcastItems *[]db.PodcastItem) error {
	*podcastItems = []db.PodcastItem{}
	for _, item := range m.PodcastItems {
		if item.PodcastID == podcastID {
			*podcastItems = append(*podcastItems, *item)
		}
	}
	return nil
}

func (m *MockRepository) GetAllPodcastItemsByPodcastIDs(podcastIds []string, podcastItems *[]db.PodcastItem) error {
	*podcastItems = []db.PodcastItem{}
	for _, podcastID := range podcastIds {
		for _, item := range m.PodcastItems {
			if item.PodcastID == podcastID {
				*podcastItems = append(*podcastItems, *item)
			}
		}
	}
	return nil
}

func (m *MockRepository) GetAllPodcastItemsByIDs(podcastItemIds []string) (*[]db.PodcastItem, error) {
	items := []db.PodcastItem{}
	for _, id := range podcastItemIds {
		if item, exists := m.PodcastItems[id]; exists {
			items = append(items, *item)
		}
	}
	return &items, nil
}

func (m *MockRepository) GetPodcastItemsByPodcastIDAndGUIDs(podcastID string, guids []string) (*[]db.PodcastItem, error) {
	items := []db.PodcastItem{}
	for _, guid := range guids {
		for _, item := range m.PodcastItems {
			if item.PodcastID == podcastID && item.GUID == guid {
				items = append(items, *item)
			}
		}
	}
	return &items, nil
}

func (m *MockRepository) GetPodcastItemByPodcastIDAndGUID(podcastID string, guid string, podcastItem *db.PodcastItem) error {
	for _, item := range m.PodcastItems {
		if item.PodcastID == podcastID && item.GUID == guid {
			*podcastItem = *item
			return nil
		}
	}
	return errors.New("podcast item not found")
}

func (m *MockRepository) GetAllPodcastItemsWithoutImage() (*[]db.PodcastItem, error) {
	items := []db.PodcastItem{}
	for _, item := range m.PodcastItems {
		if item.LocalImage == "" && item.Image != "" && item.DownloadStatus == db.Downloaded {
			items = append(items, *item)
		}
	}
	return &items, nil
}

func (m *MockRepository) GetAllPodcastItemsToBeDownloaded() (*[]db.PodcastItem, error) {
	items := []db.PodcastItem{}
	for _, item := range m.PodcastItems {
		if item.DownloadStatus == db.NotDownloaded {
			items = append(items, *item)
		}
	}
	return &items, nil
}

func (m *MockRepository) GetAllPodcastItemsAlreadyDownloaded() (*[]db.PodcastItem, error) {
	items := []db.PodcastItem{}
	for _, item := range m.PodcastItems {
		if item.DownloadStatus == db.Downloaded {
			items = append(items, *item)
		}
	}
	return &items, nil
}

func (m *MockRepository) CreatePodcastItem(podcastItem *db.PodcastItem) error {
	m.CreatePodcastItemCalls++

	if m.CreatePodcastItemError != nil {
		return m.CreatePodcastItemError
	}

	if podcastItem.ID == "" {
		podcastItem.ID = generateID()
	}
	m.PodcastItems[podcastItem.ID] = podcastItem
	return nil
}

func (m *MockRepository) UpdatePodcastItem(podcastItem *db.PodcastItem) error {
	m.UpdatePodcastItemCalls++

	if _, exists := m.PodcastItems[podcastItem.ID]; !exists {
		return errors.New("podcast item not found")
	}
	m.PodcastItems[podcastItem.ID] = podcastItem
	return nil
}

func (m *MockRepository) UpdatePodcastItemFileSize(podcastItemID string, size int64) error {
	if item, exists := m.PodcastItems[podcastItemID]; exists {
		item.FileSize = size
		return nil
	}
	return errors.New("podcast item not found")
}

func (m *MockRepository) DeletePodcastItemByID(id string) error {
	delete(m.PodcastItems, id)
	return nil
}

func (m *MockRepository) GetEpisodeNumber(podcastItemID, podcastID string) (int, error) {
	// Simplified mock - returns 1
	return 1, nil
}

// Stats operations

func (m *MockRepository) GetPodcastEpisodeStats() (*[]db.PodcastItemStatsModel, error) {
	stats := []db.PodcastItemStatsModel{}
	return &stats, nil
}

func (m *MockRepository) GetPodcastEpisodeDiskStats() (db.PodcastItemConsolidateDiskStatsModel, error) {
	return db.PodcastItemConsolidateDiskStatsModel{}, nil
}

// Tag operations

func (m *MockRepository) GetAllTags(sorting string) (*[]db.Tag, error) {
	tags := make([]db.Tag, 0, len(m.Tags))
	for _, tag := range m.Tags {
		tags = append(tags, *tag)
	}
	return &tags, nil
}

func (m *MockRepository) GetPaginatedTags(page int, count int, tags *[]db.Tag, total *int64) error {
	*tags = []db.Tag{}
	for _, tag := range m.Tags {
		*tags = append(*tags, *tag)
	}
	*total = int64(len(m.Tags))
	return nil
}

func (m *MockRepository) GetTagByID(id string) (*db.Tag, error) {
	if tag, exists := m.Tags[id]; exists {
		return tag, nil
	}
	return nil, errors.New("tag not found")
}

func (m *MockRepository) GetTagsByIDs(ids []string) (*[]db.Tag, error) {
	tags := []db.Tag{}
	for _, id := range ids {
		if tag, exists := m.Tags[id]; exists {
			tags = append(tags, *tag)
		}
	}
	return &tags, nil
}

func (m *MockRepository) GetTagByLabel(label string) (*db.Tag, error) {
	for _, tag := range m.Tags {
		if tag.Label == label {
			return tag, nil
		}
	}
	return nil, errors.New("tag not found")
}

func (m *MockRepository) CreateTag(tag *db.Tag) error {
	if tag.ID == "" {
		tag.ID = generateID()
	}
	m.Tags[tag.ID] = tag
	return nil
}

func (m *MockRepository) UpdateTag(tag *db.Tag) error {
	if _, exists := m.Tags[tag.ID]; !exists {
		return errors.New("tag not found")
	}
	m.Tags[tag.ID] = tag
	return nil
}

func (m *MockRepository) DeleteTagByID(id string) error {
	delete(m.Tags, id)
	return nil
}

func (m *MockRepository) AddTagToPodcast(id, tagID string) error {
	// Simplified mock - no-op
	return nil
}

func (m *MockRepository) RemoveTagFromPodcast(id, tagID string) error {
	// Simplified mock - no-op
	return nil
}

func (m *MockRepository) UntagAllByTagID(tagID string) error {
	// Simplified mock - no-op
	return nil
}

// Settings operations

func (m *MockRepository) GetOrCreateSetting() *db.Setting {
	m.GetOrCreateSettingCalls++
	return m.Settings
}

func (m *MockRepository) UpdateSettings(setting *db.Setting) error {
	m.Settings = setting
	return nil
}

// Job lock operations

func (m *MockRepository) GetLock(name string) *db.JobLock {
	if lock, exists := m.JobLocks[name]; exists {
		return lock
	}
	return &db.JobLock{Name: name}
}

func (m *MockRepository) Lock(name string, duration int) {
	m.LockCalls++
	m.JobLocks[name] = &db.JobLock{
		Name:     name,
		Duration: duration,
		Date:     time.Now(),
	}
}

func (m *MockRepository) Unlock(name string) {
	m.UnlockCalls++
	if lock, exists := m.JobLocks[name]; exists {
		lock.Date = time.Time{}
		lock.Duration = 0
	}
}

func (m *MockRepository) UnlockMissedJobs() {
	// Simplified mock - no-op
}

// Helper functions

func generateID() string {
	return "mock-id-" + time.Now().Format("20060102150405")
}
