// Package db provides database models and data access functions.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/akhilrex/podgrab/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetPodcastByURL get podcast by u r l.
func GetPodcastByURL(url string, podcast *Podcast) error {
	result := DB.Preload(clause.Associations).Where(&Podcast{URL: url}).First(&podcast)
	return result.Error
}

// GetPodcastsByURLList get podcasts by u r l list.
func GetPodcastsByURLList(urls []string, podcasts *[]Podcast) error {
	result := DB.Preload(clause.Associations).Where("url in ?", urls).First(&podcasts)
	return result.Error
}

// GetAllPodcasts get all podcasts.
func GetAllPodcasts(podcasts *[]Podcast, sorting string) error {
	if sorting == "" {
		sorting = "created_at"
	}
	result := DB.Preload("Tags").Order(sorting).Find(&podcasts)
	return result.Error
}

// GetAllPodcastItems get all podcast items.
func GetAllPodcastItems(podcasts *[]PodcastItem) error {
	result := DB.Preload("Podcast").Order("pub_date desc").Find(&podcasts)
	return result.Error
}

// GetAllPodcastItemsWithoutSize get all podcast items without size.
func GetAllPodcastItemsWithoutSize() (*[]PodcastItem, error) {
	var podcasts []PodcastItem
	result := DB.Where("file_size<=?", 0).Order("pub_date desc").Find(&podcasts)
	return &podcasts, result.Error
}

func getSortOrder(sorting model.EpisodeSort) string {
	switch sorting {
	case model.ReleaseAsc:
		return "pub_date asc"
	case model.ReleaseDesc:
		return "pub_date desc"
	case model.DurationAsc:
		return "duration asc"
	case model.DurationDesc:
		return "duration desc"
	default:
		return "pub_date desc"
	}
}

// GetPaginatedPodcastItemsNew get paginated podcast items new.
func GetPaginatedPodcastItemsNew(queryModel model.EpisodesFilter) (*[]PodcastItem, int64, error) {
	var podcasts []PodcastItem
	var total int64
	query := DB.Debug().Preload("Podcast")
	if queryModel.IsDownloaded != nil {
		isDownloaded, err := strconv.ParseBool(*queryModel.IsDownloaded)
		if err == nil {
			if isDownloaded {
				query = query.Where("download_status=?", Downloaded)
			} else {
				query = query.Where("download_status!=?", Downloaded)
			}
		}
	}
	if queryModel.IsPlayed != nil {
		isPlayed, err := strconv.ParseBool(*queryModel.IsPlayed)
		if err == nil {
			if isPlayed {
				query = query.Where("is_played=?", 1)
			} else {
				query = query.Where("is_played=?", 0)
			}
		}
	}

	if queryModel.Q != "" {
		query = query.Where("UPPER(title) like ?", "%"+strings.TrimSpace(strings.ToUpper(queryModel.Q))+"%")
	}

	if len(queryModel.TagIDs) > 0 {
		query = query.Where("podcast_id in (select podcast_id from podcast_tags where tag_id in ?)", queryModel.TagIDs)
	}

	if len(queryModel.PodcastIDs) > 0 {
		query = query.Where("podcast_id in ?", queryModel.PodcastIDs)
	}

	totalsQuery := query.Order(getSortOrder(queryModel.Sorting)).Find(&podcasts)
	totalsQuery.Count(&total)

	result := query.Limit(queryModel.Count).Offset((queryModel.Page - 1) * queryModel.Count).Order("pub_date desc").Find(&podcasts)
	return &podcasts, total, result.Error
}

// GetPaginatedPodcastItems get paginated podcast items.
func GetPaginatedPodcastItems(page int, count int, downloadedOnly *bool, playedOnly *bool, fromDate time.Time, podcasts *[]PodcastItem, total *int64) error {
	query := DB.Preload("Podcast")
	if downloadedOnly != nil {
		if *downloadedOnly {
			query = query.Where("download_status=?", Downloaded)
		} else {
			query = query.Where("download_status!=?", Downloaded)
		}
	}
	if playedOnly != nil {
		if *playedOnly {
			query = query.Where("is_played=?", 1)
		} else {
			query = query.Where("is_played=?", 0)
		}
	}
	if (fromDate != time.Time{}) {
		query = query.Where("pub_date>=?", fromDate)
	}

	totalsQuery := query.Order("pub_date desc").Find(&podcasts)
	totalsQuery.Count(total)

	result := query.Limit(count).Offset((page - 1) * count).Order("pub_date desc").Find(&podcasts)
	return result.Error
}

// GetPaginatedTags get paginated tags.
func GetPaginatedTags(page int, count int, tags *[]Tag, total *int64) error {
	query := DB.Preload("Podcasts")

	result := query.Limit(count).Offset((page - 1) * count).Order("created_at desc").Find(&tags)

	query.Count(total)

	return result.Error
}

// GetPodcastByID get podcast by id.
func GetPodcastByID(id string, podcast *Podcast) error {
	result := DB.Preload("PodcastItems", func(db *gorm.DB) *gorm.DB {
		return db.Order("podcast_items.pub_date DESC")
	}).First(&podcast, "id=?", id)
	return result.Error
}

// GetPodcastItemByID get podcast item by id.
func GetPodcastItemByID(id string, podcastItem *PodcastItem) error {
	result := DB.Preload(clause.Associations).First(&podcastItem, "id=?", id)
	return result.Error
}

// DeletePodcastItemByID delete podcast item by id.
func DeletePodcastItemByID(id string) error {
	result := DB.Where("id=?", id).Delete(&PodcastItem{})
	return result.Error
}

// DeletePodcastByID delete podcast by id.
func DeletePodcastByID(id string) error {
	// Delete associated podcast items first
	if err := DB.Where("podcast_id = ?", id).Delete(&PodcastItem{}).Error; err != nil {
		return err
	}

	// Then delete the podcast
	result := DB.Where("id=?", id).Delete(&Podcast{})
	return result.Error
}

// DeleteTagByID delete tag by id.
func DeleteTagByID(id string) error {
	result := DB.Where("id=?", id).Delete(&Tag{})
	return result.Error
}

// GetAllPodcastItemsByPodcastID get all podcast items by podcast id.
func GetAllPodcastItemsByPodcastID(podcastID string, podcastItems *[]PodcastItem) error {
	result := DB.Preload(clause.Associations).Where(&PodcastItem{PodcastID: podcastID}).Find(&podcastItems)
	return result.Error
}

// GetAllPodcastItemsByPodcastIDs get all podcast items by podcast ids.
func GetAllPodcastItemsByPodcastIDs(podcastIDs []string, podcastItems *[]PodcastItem) error {
	result := DB.Preload(clause.Associations).Where("podcast_id in ?", podcastIDs).Order("pub_date desc").Find(&podcastItems)
	return result.Error
}

// GetAllPodcastItemsByIDs get all podcast items by ids.
func GetAllPodcastItemsByIDs(podcastItemIDs []string) (*[]PodcastItem, error) {
	var podcastItems []PodcastItem

	var sb strings.Builder

	sb.WriteString("\n CASE ID \n")

	for i, v := range podcastItemIDs {
		sb.WriteString(fmt.Sprintf("WHEN '%v' THEN %v \n", v, i+1))
	}

	sb.WriteString(fmt.Sprintln("END"))

	result := DB.Debug().Preload(clause.Associations).Where("id in ?", podcastItemIDs).Order(sb.String()).Find(&podcastItems)
	return &podcastItems, result.Error
}

// SetAllEpisodesToDownload set all episodes to download.
func SetAllEpisodesToDownload(podcastID string) error {
	result := DB.Model(PodcastItem{}).Where(&PodcastItem{PodcastID: podcastID, DownloadStatus: Deleted}).Update("download_status", NotDownloaded)
	return result.Error
}

// UpdateLastEpisodeDateForPodcast update last episode date for podcast.
func UpdateLastEpisodeDateForPodcast(podcastID string, lastEpisode time.Time) error {
	result := DB.Model(Podcast{}).Where("id=?", podcastID).Update("last_episode", lastEpisode)
	return result.Error
}

// UpdatePodcastItemFileSize update podcast item file size.
func UpdatePodcastItemFileSize(podcastItemID string, size int64) error {
	result := DB.Model(PodcastItem{}).Where("id=?", podcastItemID).Update("file_size", size)
	return result.Error
}

// GetAllPodcastItemsWithoutImage get all podcast items without image.
func GetAllPodcastItemsWithoutImage() (*[]PodcastItem, error) {
	var podcastItems []PodcastItem
	result := DB.Preload(clause.Associations).Where("local_image is ?", nil).Where("image != ?", "").Where("download_status=?", Downloaded).Order("created_at desc").Find(&podcastItems)
	// fmt.Println("To be downloaded : " + string(len(podcastItems)))
	return &podcastItems, result.Error
}

// GetAllPodcastItemsToBeDownloaded get all podcast items to be downloaded.
func GetAllPodcastItemsToBeDownloaded() (*[]PodcastItem, error) {
	// Return empty slice if database is not available
	if DB == nil {
		return &[]PodcastItem{}, nil
	}

	var podcastItems []PodcastItem
	result := DB.Preload(clause.Associations).Where("download_status=?", NotDownloaded).Find(&podcastItems)
	// fmt.Println("To be downloaded : " + string(len(podcastItems)))
	return &podcastItems, result.Error
}

// GetAllPodcastItemsAlreadyDownloaded get all podcast items already downloaded.
func GetAllPodcastItemsAlreadyDownloaded() (*[]PodcastItem, error) {
	var podcastItems []PodcastItem
	result := DB.Preload(clause.Associations).Where("download_status=?", Downloaded).Find(&podcastItems)
	return &podcastItems, result.Error
}

// GetPodcastEpisodeStats get podcast episode stats.
func GetPodcastEpisodeStats() (*[]PodcastItemStatsModel, error) {
	var stats []PodcastItemStatsModel
	result := DB.Model(&PodcastItem{}).Select("download_status,podcast_id, count(1) as count,sum(file_size) as size").Group("podcast_id,download_status").Find(&stats)
	return &stats, result.Error
}

// GetPodcastEpisodeDiskStats get podcast episode disk stats.
func GetPodcastEpisodeDiskStats() (PodcastItemConsolidateDiskStatsModel, error) {
	var stats []PodcastItemDiskStatsModel
	result := DB.Model(&PodcastItem{}).Select("download_status,count(1) as count,sum(file_size) as size").Group("download_status").Find(&stats)
	dict := make(map[DownloadStatus]int64)
	for _, stat := range stats {
		dict[stat.DownloadStatus] = stat.Size
	}

	toReturn := PodcastItemConsolidateDiskStatsModel{
		Downloaded:      dict[Downloaded],
		Downloading:     dict[Downloading],
		Deleted:         dict[Deleted],
		NotDownloaded:   dict[NotDownloaded],
		PendingDownload: dict[NotDownloaded] + dict[Downloading],
	}

	return toReturn, result.Error
}

// GetEpisodeNumber get episode number.
func GetEpisodeNumber(podcastItemID, podcastID string) (int, error) {
	var id string
	var sequence int
	row := DB.Raw(`;With cte as(
		SELECT
			id,
			RANK() OVER (ORDER BY pub_date) as sequence
		FROM
			podcast_items
		WHERE
			podcast_id=?
	)
	select *
	from cte
	where id = ?
	`, podcastID, podcastItemID).Row()
	err := row.Scan(&id, &sequence)
	return sequence, err
}

// ForceSetLastEpisodeDate force set last episode date.
func ForceSetLastEpisodeDate(podcastID string) {
	DB.Exec("update podcasts set last_episode = (select max(pi.pub_date) from podcast_items pi where pi.podcast_id = @id) where id = @id", sql.Named("id", podcastID))
}

// TogglePodcastPauseStatus toggle podcast pause status.
func TogglePodcastPauseStatus(podcastID string, isPaused bool) error {
	tx := DB.Debug().Exec("update podcasts set is_paused = @isPaused where id = @id", sql.Named("id", podcastID), sql.Named("isPaused", isPaused))
	return tx.Error
}

// GetPodcastItemsByPodcastIDAndGUIDs get podcast items by podcast id and g u i ds.
func GetPodcastItemsByPodcastIDAndGUIDs(podcastID string, guids []string) (*[]PodcastItem, error) {
	var podcastItems []PodcastItem
	result := DB.Preload(clause.Associations).Where(&PodcastItem{PodcastID: podcastID}).Where("guid IN ?", guids).Find(&podcastItems)
	return &podcastItems, result.Error
}

// GetPodcastItemByPodcastIDAndGUID get podcast item by podcast id and g u i d.
func GetPodcastItemByPodcastIDAndGUID(podcastID string, guid string, podcastItem *PodcastItem) error {
	result := DB.Preload(clause.Associations).Where(&PodcastItem{PodcastID: podcastID, GUID: guid}).First(&podcastItem)
	return result.Error
}

// GetPodcastByTitleAndAuthor get podcast by title and author.
func GetPodcastByTitleAndAuthor(title string, author string, podcast *Podcast) error {
	result := DB.Preload(clause.Associations).Where(&Podcast{Title: title, Author: author}).First(&podcast)
	return result.Error
}

// CreatePodcast create podcast.
func CreatePodcast(podcast *Podcast) error {
	tx := DB.Create(&podcast)
	return tx.Error
}

// CreatePodcastItem create podcast item.
func CreatePodcastItem(podcastItem *PodcastItem) error {
	tx := DB.Omit("Podcast").Create(&podcastItem)
	return tx.Error
}

// UpdatePodcast update podcast.
func UpdatePodcast(podcast *Podcast) error {
	tx := DB.Save(&podcast)
	return tx.Error
}

// UpdatePodcastItem update podcast item.
func UpdatePodcastItem(podcastItem *PodcastItem) error {
	tx := DB.Omit("Podcast").Save(&podcastItem)
	return tx.Error
}

// UpdateSettings update settings.
func UpdateSettings(setting *Setting) error {
	tx := DB.Save(&setting)
	return tx.Error
}

// GetOrCreateSetting get or create setting.
func GetOrCreateSetting() *Setting {
	// Return default setting if database is not available
	if DB == nil {
		return &Setting{}
	}

	var setting Setting
	result := DB.First(&setting)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		DB.Save(&Setting{})
		DB.First(&setting)
	}
	return &setting
}

// GetLock get lock.
func GetLock(name string) *JobLock {
	// Return unlocked job if database is not available
	if DB == nil {
		return &JobLock{
			Name: name,
		}
	}

	var jobLock JobLock
	result := DB.Where("name = ?", name).First(&jobLock)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return &JobLock{
			Name: name,
		}
	}
	return &jobLock
}

// Lock lock.
func Lock(name string, duration int) {
	// Skip if database is not available
	if DB == nil {
		return
	}

	jobLock := GetLock(name)
	if jobLock == nil {
		jobLock = &JobLock{
			Name: name,
		}
	}
	jobLock.Duration = duration
	jobLock.Date = time.Now()
	if jobLock.ID == "" {
		DB.Create(&jobLock)
	} else {
		DB.Save(&jobLock)
	}
}

// Unlock unlock.
func Unlock(name string) {
	// Skip if database is not available
	if DB == nil {
		return
	}

	jobLock := GetLock(name)
	if jobLock == nil {
		return
	}
	jobLock.Duration = 0
	jobLock.Date = time.Time{}
	DB.Save(&jobLock)
}

// UnlockMissedJobs unlock missed jobs.
func UnlockMissedJobs() {
	var jobLocks []JobLock

	result := DB.Find(&jobLocks)
	if result.Error != nil {
		return
	}
	for _, job := range jobLocks {
		if (job.Date.Equal(time.Time{})) {
			continue
		}
		var duration = time.Duration(job.Duration)
		d := job.Date.Add(time.Minute * duration)
		if d.Before(time.Now()) {
			fmt.Println(job.Name + " is unlocked")
			Unlock(job.Name)
		}
	}
}

// GetAllTags get all tags.
func GetAllTags(sorting string) (*[]Tag, error) {
	var tags []Tag
	if sorting == "" {
		sorting = "created_at"
	}
	result := DB.Preload(clause.Associations).Order(sorting).Find(&tags)
	return &tags, result.Error
}

// GetTagByID get tag by id.
func GetTagByID(id string) (*Tag, error) {
	var tag Tag
	result := DB.Preload(clause.Associations).
		First(&tag, "id=?", id)

	return &tag, result.Error
}

// GetTagsByIDs get tags by ids.
func GetTagsByIDs(ids []string) (*[]Tag, error) {
	var tag []Tag
	result := DB.Preload(clause.Associations).Where("id in ?", ids).Find(&tag)

	return &tag, result.Error
}

// GetTagByLabel get tag by label.
func GetTagByLabel(label string) (*Tag, error) {
	var tag Tag
	result := DB.Preload(clause.Associations).
		First(&tag, "label=?", label)

	return &tag, result.Error
}

// CreateTag create tag.
func CreateTag(tag *Tag) error {
	tx := DB.Omit("Podcasts").Create(&tag)
	return tx.Error
}

// UpdateTag update tag.
func UpdateTag(tag *Tag) error {
	tx := DB.Omit("Podcast").Save(&tag)
	return tx.Error
}

// AddTagToPodcast add tag to podcast.
func AddTagToPodcast(id, tagID string) error {
	tx := DB.Exec("INSERT INTO `podcast_tags` (`podcast_id`,`tag_id`) VALUES (?,?) ON CONFLICT DO NOTHING", id, tagID)
	return tx.Error
}

// RemoveTagFromPodcast remove tag from podcast.
func RemoveTagFromPodcast(id, tagID string) error {
	tx := DB.Exec("DELETE FROM `podcast_tags` WHERE `podcast_id`=? AND `tag_id`=?", id, tagID)
	return tx.Error
}

// UntagAllByTagID untag all by tag id.
func UntagAllByTagID(tagID string) error {
	tx := DB.Exec("DELETE FROM `podcast_tags` WHERE `tag_id`=?", tagID)
	return tx.Error
}
