package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/toozej/monogo/apps/podgrab/db"
	"github.com/toozej/monogo/apps/podgrab/internal/logger"
	"github.com/toozej/monogo/apps/podgrab/internal/sanitize"
	"github.com/toozej/monogo/apps/podgrab/model"
	"github.com/toozej/monogo/apps/podgrab/service"
)

// Sorting field constants for podcast queries.
const (
	DateAdded   = "dateadded"
	Name        = "name"
	LastEpisode = "lastepisode"
)

// Sort order constants for query results.
const (
	Asc  = "asc"
	Desc = "desc"
)

// SearchQuery represents search query data.
type SearchQuery struct {
	Q    string `binding:"required" form:"q"`
	Type string `form:"type"`
}

// PodcastListQuery represents podcast list query data.
type PodcastListQuery struct {
	Sort  string `uri:"sort" query:"sort" json:"sort" form:"sort" default:"created_at"`
	Order string `uri:"order" query:"order" json:"order" form:"order" default:"asc"`
}

// SearchByIDQuery represents search by id query data.
type SearchByIDQuery struct {
	ID string `binding:"required" uri:"id" json:"id" form:"id"`
}

// AddRemoveTagQuery represents add remove tag query data.
type AddRemoveTagQuery struct {
	ID    string `binding:"required" uri:"id" json:"id" form:"id"`
	TagID string `binding:"required" uri:"tagID" json:"tagID" form:"tagID"`
}

// PatchPodcastItem represents patch podcast item data.
type PatchPodcastItem struct {
	Title    string `form:"title" json:"title" query:"title"`
	IsPlayed bool   `json:"isPlayed" form:"isPlayed" query:"isPlayed"`
}

// AddPodcastData represents add podcast data data.
type AddPodcastData struct {
	URL string `binding:"required" form:"url" json:"url"`
}

// AddTagData represents add tag data data.
type AddTagData struct {
	Label       string `binding:"required" form:"label" json:"label"`
	Description string `form:"description" json:"description"`
}

// PodcastItemsResponse is the paginated podcast episode response.
type PodcastItemsResponse struct {
	PodcastItems []db.PodcastItem     `json:"podcastItems"`
	Filter       model.EpisodesFilter `json:"filter"`
}

// GetAllPodcasts handles the get all podcasts request.
// @Summary List podcasts
// @Description Returns every saved podcast using the requested sorting.
// @Tags podcasts
// @Produce json
// @Security BasicAuth
// @Param sort query string false "Sort field" Enums(dateadded,name,lastepisode) default(dateadded)
// @Param order query string false "Sort order" Enums(asc,desc) default(asc)
// @Success 200 {array} db.Podcast
// @Failure 400 {object} map[string]string
// @Router /podcasts [get]
func GetAllPodcasts(c *gin.Context) {
	var podcastListQuery PodcastListQuery

	if c.ShouldBindQuery(&podcastListQuery) == nil {
		var order = strings.ToLower(podcastListQuery.Order)
		var sorting = "created_at"
		switch sort := strings.ToLower(podcastListQuery.Sort); sort {
		case DateAdded:
			sorting = "created_at"
		case Name:
			sorting = "title"
		case LastEpisode:
			sorting = "last_episode"
		}
		if order == Desc {
			sorting = fmt.Sprintf("%s desc", sorting)
		}

		c.JSON(200, service.GetAllPodcasts(sorting))
	}
}

// entityFetcher is a function type for fetching an entity by ID
type entityFetcher func(id string, entity interface{}) error

// handleEntityByID is a generic handler for fetching entities by ID
func handleEntityByID(c *gin.Context, fetcher entityFetcher, entity interface{}, notFoundMsg string) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	err := fetcher(searchByIDQuery.ID, entity)
	if err != nil {
		logger.Log.Errorw("getting entity by ID", "error", err, "id", searchByIDQuery.ID)
		c.JSON(http.StatusNotFound, gin.H{"error": notFoundMsg})
		return
	}
	c.JSON(200, entity)
}

// GetPodcastByID handles the get podcast by id request.
// @Summary Get a podcast
// @Tags podcasts
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 200 {object} db.Podcast
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /podcasts/{id} [get]
func GetPodcastByID(c *gin.Context) {
	var podcast db.Podcast
	handleEntityByID(c, func(id string, entity interface{}) error {
		podcastEntity, ok := entity.(*db.Podcast)
		if !ok {
			return fmt.Errorf("invalid entity type: expected *db.Podcast")
		}
		return db.GetPodcastByID(id, podcastEntity)
	}, &podcast, "Podcast not found")
}

// PausePodcastByID handles the pause podcast by id request.
// @Summary Pause a podcast
// @Description Prevents automatic processing for the podcast.
// @Tags podcasts
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 200 {object} object
// @Failure 400 {object} map[string]string
// @Router /podcasts/{id}/pause [get]
func PausePodcastByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery
	if c.ShouldBindUri(&searchByIDQuery) == nil {
		err := service.TogglePodcastPause(searchByIDQuery.ID, true)
		if err != nil {
			c.JSON(http.StatusBadRequest, err)
			return
		}
		c.JSON(200, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// UnpausePodcastByID handles the unpause podcast by id request.
// @Summary Unpause a podcast
// @Tags podcasts
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 200 {object} object
// @Failure 400 {object} map[string]string
// @Router /podcasts/{id}/unpause [get]
func UnpausePodcastByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery
	if c.ShouldBindUri(&searchByIDQuery) == nil {
		err := service.TogglePodcastPause(searchByIDQuery.ID, false)
		if err != nil {
			c.JSON(http.StatusBadRequest, err)
			return
		}
		c.JSON(200, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// DeletePodcastByID handles the delete podcast by id request.
// @Summary Delete a podcast and its files
// @Tags podcasts
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 204 "Podcast and episode files deleted"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /podcasts/{id} [delete]
func DeletePodcastByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		if err := service.DeletePodcast(searchByIDQuery.ID, true); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// DeleteOnlyPodcastByID handles the delete only podcast by id request.
// @Summary Delete a podcast but keep its files
// @Tags podcasts
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 204 "Podcast deleted"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /podcasts/{id}/podcast [delete]
func DeleteOnlyPodcastByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		if err := service.DeletePodcast(searchByIDQuery.ID, false); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// DeletePodcastEpisodesByID handles the delete podcast episodes by id request.
// @Summary Delete all episode files for a podcast
// @Tags podcasts
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 204 "Episode files deleted"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /podcasts/{id}/items [delete]
func DeletePodcastEpisodesByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		if err := service.DeletePodcastEpisodes(searchByIDQuery.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// DeletePodcasDeleteOnlyPodcasttEpisodesByID handles the delete podcas delete only podcastt episodes by id request.
func DeletePodcasDeleteOnlyPodcasttEpisodesByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		if err := service.DeletePodcastEpisodes(searchByIDQuery.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusNoContent, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// GetPodcastItemsByPodcastID handles the get podcast items by podcast id request.
// @Summary List a podcast's episodes
// @Tags podcasts,episodes
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 200 {array} db.PodcastItem
// @Failure 400 {object} map[string]string
// @Router /podcasts/{id}/items [get]
func GetPodcastItemsByPodcastID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcastItems []db.PodcastItem

		err := db.GetAllPodcastItemsByPodcastID(searchByIDQuery.ID, &podcastItems)
		logger.Log.Error(err)
		c.JSON(200, podcastItems)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// DownloadAllEpisodesByPodcastID handles the download all episodes by podcast id request.
// @Summary Queue all episodes for download
// @Tags podcasts,episodes
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 200 {object} object
// @Failure 400 {object} map[string]string
// @Router /podcasts/{id}/download [get]
func DownloadAllEpisodesByPodcastID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		err := service.SetAllEpisodesToDownload(searchByIDQuery.ID)
		if err != nil {
			logger.Log.Errorw("setting episodes to download", "error", err)
		}
		go func() {
			if refreshErr := service.RefreshEpisodes(); refreshErr != nil {
				logger.Log.Errorw("refreshing episodes", "error", refreshErr)
			}
		}()
		c.JSON(200, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// RefreshEpisodes handles the refresh all episodes request.
// @Summary Refresh all podcasts
// @Description Starts an asynchronous refresh of every saved podcast feed.
// @Tags podcasts
// @Produce json
// @Security BasicAuth
// @Success 200 {object} object
// @Router /refreshAll [get]
func RefreshEpisodes(c *gin.Context) {
	go func() {
		if err := service.RefreshEpisodes(); err != nil {
			logger.Log.Errorw("refreshing episodes", "error", err)
		}
	}()
	c.JSON(200, gin.H{})
}

// RefreshEpisodesByPodcastID handles the refresh episodes by podcast id request.
// @Summary Refresh one podcast
// @Description Starts an asynchronous refresh of the selected podcast feed.
// @Tags podcasts
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 200 {object} object
// @Failure 400 {object} map[string]string
// @Router /podcasts/{id}/refresh [get]
func RefreshEpisodesByPodcastID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		go func() {
			if err := service.RefreshPodcastByPodcastID(searchByIDQuery.ID); err != nil {
				logger.Log.Errorw("refreshing podcast", "id", searchByIDQuery.ID, "error", err)
			}
		}()
		c.JSON(200, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// GetAllPodcastItems handles the get all podcast items request.
// @Summary List episodes
// @Description Returns a filtered, sorted, paginated list of podcast episodes.
// @Tags episodes
// @Produce json
// @Security BasicAuth
// @Param downloadStatus query string false "Download status"
// @Param episodeType query string false "Episode type"
// @Param isPlayed query string false "Played-state filter"
// @Param sorting query string false "Sort order" Enums(release_asc,release_desc,duration_asc,duration_desc) default(release_desc)
// @Param q query string false "Text search"
// @Param tagIds[] query []string false "Tag IDs"
// @Param podcastIDs[] query []string false "Podcast IDs"
// @Param page query int false "Page number" default(1)
// @Param count query int false "Episodes per page" default(20)
// @Success 200 {object} PodcastItemsResponse
// @Failure 400 {object} map[string]string
// @Router /podcastitems [get]
func GetAllPodcastItems(c *gin.Context) {
	var filter model.EpisodesFilter
	err := c.ShouldBindQuery(&filter)
	if err != nil {
		logger.Log.Error(err.Error())
	}
	filter.VerifyPaginationValues()
	if podcastItems, totalCount, err := db.GetPaginatedPodcastItemsNew(&filter); err == nil {
		filter.SetCounts(totalCount)
		toReturn := gin.H{
			"podcastItems": podcastItems,
			"filter":       &filter,
		}
		c.JSON(http.StatusOK, toReturn)
	} else {
		c.JSON(http.StatusBadRequest, err)
	}
}

// GetPodcastItemByID handles the get podcast item by id request.
// @Summary Get an episode
// @Tags episodes
// @Produce json
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Success 200 {object} db.PodcastItem
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /podcastitems/{id} [get]
func GetPodcastItemByID(c *gin.Context) {
	var podcast db.PodcastItem
	handleEntityByID(c, func(id string, entity interface{}) error {
		itemEntity, ok := entity.(*db.PodcastItem)
		if !ok {
			return fmt.Errorf("invalid entity type: expected *db.PodcastItem")
		}
		return db.GetPodcastItemByID(id, itemEntity)
	}, &podcast, "Episode not found")
}

// GetPodcastItemImageByID handles the get podcast item image by id request.
// @Summary Get an episode image
// @Description Returns the cached episode image or redirects to its remote image.
// @Tags episodes,media
// @Produce application/octet-stream
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Success 200 {file} binary
// @Success 302 "Redirect to remote image"
// @Failure 400 {object} map[string]string
// @Router /podcastitems/{id}/image [get]
func GetPodcastItemImageByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcast db.PodcastItem

		err := db.GetPodcastItemByID(searchByIDQuery.ID, &podcast)
		if err == nil {
			if _, err = os.Stat(podcast.LocalImage); os.IsNotExist(err) {
				c.Redirect(302, podcast.Image)
			} else {
				c.File(podcast.LocalImage)
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// GetPodcastImageByID handles the get podcast image by id request.
// @Summary Get a podcast image
// @Description Returns the cached podcast image or redirects to its remote image.
// @Tags podcasts,media
// @Produce application/octet-stream
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 200 {file} binary
// @Success 302 "Redirect to remote image"
// @Failure 400 {object} map[string]string
// @Router /podcasts/{id}/image [get]
func GetPodcastImageByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcast db.Podcast

		err := db.GetPodcastByID(searchByIDQuery.ID, &podcast)
		if err == nil {
			localPath := service.GetPodcastLocalImagePath(podcast.Image, podcast.Title)
			if _, err = os.Stat(localPath); os.IsNotExist(err) {
				c.Redirect(302, podcast.Image)
			} else {
				c.File(localPath)
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// GetPodcastItemFileByID handles the get podcast item file by id request.
// @Summary Get an episode audio file
// @Description Downloads the local episode file or redirects to its remote audio URL.
// @Tags episodes,media
// @Produce application/octet-stream
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Success 200 {file} binary
// @Success 302 "Redirect to remote audio"
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /podcastitems/{id}/file [get]
func GetPodcastItemFileByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var item db.PodcastItem

		err := db.GetPodcastItemByID(searchByIDQuery.ID, &item)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
			return
		}

		// Try the original DownloadPath first
		filePath := item.DownloadPath
		if _, err = os.Stat(filePath); os.IsNotExist(err) {
			// File not found at stored path - try to find it
			// This handles backward compatibility when folder naming conventions changed
			filePath = findEpisodeFile(&item)
		}

		if filePath != "" {
			if _, err = os.Stat(filePath); !os.IsNotExist(err) {
				c.Header("Content-Description", "File Transfer")
				c.Header("Content-Transfer-Encoding", "binary")
				c.Header("Content-Disposition", "attachment; filename="+path.Base(filePath))
				c.Header("Content-Type", GetFileContentType(filePath))
				c.File(filePath)
				return
			}
		}

		// File not found locally - redirect to remote URL if available
		if item.FileURL != "" {
			c.Redirect(302, item.FileURL)
			return
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// findEpisodeFile attempts to locate the same episode filename in the current
// and legacy podcast directories. It never substitutes another episode's file.
func findEpisodeFile(item *db.PodcastItem) string {
	episodeFileName := filepath.Base(item.DownloadPath)
	if episodeFileName == "." || episodeFileName == string(filepath.Separator) || episodeFileName == "" {
		return ""
	}

	dataPath := os.Getenv("DATA")
	if dataPath == "" {
		dataPath = "./assets"
	}

	// Get the podcast name - handle both preloaded and non-preloaded cases
	podcastName := item.Podcast.Title
	if podcastName == "" {
		var podcast db.Podcast
		if err := db.GetPodcastByID(item.PodcastID, &podcast); err == nil {
			podcastName = podcast.Title
		}
	}

	// If we have a podcast name, first look in that podcast's folder
	if podcastName != "" {
		sanitizedName := sanitize.Name(podcastName)
		podcastDir := filepath.Join(dataPath, sanitizedName)

		candidate := filepath.Join(podcastDir, episodeFileName)
		if resolved := existingRegularFileWithin(dataPath, candidate); resolved != "" {
			return resolved
		}

		// Also try the old-style folder name (with spaces/unsanitized)
		oldStyleDir := filepath.Join(dataPath, podcastName)
		if oldStyleDir != podcastDir {
			candidate = filepath.Join(oldStyleDir, episodeFileName)
			if resolved := existingRegularFileWithin(dataPath, candidate); resolved != "" {
				return resolved
			}
		}
	}
	return ""
}

func existingRegularFileWithin(baseDir, candidate string) string {
	resolvedBase, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return ""
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return ""
	}
	relative, err := filepath.Rel(resolvedBase, resolvedCandidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	info, err := os.Stat(resolvedCandidate) // #nosec G703 -- EvalSymlinks and filepath.Rel above prove the resolved path remains within resolvedBase
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return candidate
}

// GetFileContentType handles the get file content type request.
func GetFileContentType(filePath string) string {
	file, err := os.Open(filePath) // #nosec G304 -- filePath is from database, managed by application
	if err != nil {
		return "application/octet-stream"
	}
	defer func() {
		if err := file.Close(); err != nil {
			logger.Log.Errorw("closing file", "error", err)
		}
	}()
	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil {
		return "application/octet-stream"
	}
	return http.DetectContentType(buffer)
}

// MarkPodcastItemAsUnplayed handles the mark podcast item as unplayed request.
// @Summary Mark an episode unplayed
// @Tags episodes
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Success 200 "Episode marked unplayed"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /podcastitems/{id}/markUnplayed [get]
func MarkPodcastItemAsUnplayed(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		if err := service.SetPodcastItemPlayedStatus(searchByIDQuery.ID, false); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// MarkPodcastItemAsPlayed handles the mark podcast item as played request.
// @Summary Mark an episode played
// @Tags episodes
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Success 200 "Episode marked played"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /podcastitems/{id}/markPlayed [get]
func MarkPodcastItemAsPlayed(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		if err := service.SetPodcastItemPlayedStatus(searchByIDQuery.ID, true); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// BookmarkPodcastItem handles the bookmark podcast item request.
// @Summary Bookmark an episode
// @Tags episodes
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Success 200 "Episode bookmarked"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /podcastitems/{id}/bookmark [get]
func BookmarkPodcastItem(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		if err := service.SetPodcastItemBookmarkStatus(searchByIDQuery.ID, true); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// UnbookmarkPodcastItem handles the unbookmark podcast item request.
// @Summary Remove an episode bookmark
// @Tags episodes
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Success 200 "Episode bookmark removed"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /podcastitems/{id}/unbookmark [get]
func UnbookmarkPodcastItem(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		if err := service.SetPodcastItemBookmarkStatus(searchByIDQuery.ID, false); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// PatchPodcastItemByID handles the patch podcast item by id request.
// @Summary Update an episode
// @Description Updates an episode's title and played state.
// @Tags episodes
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Param episode body PatchPodcastItem true "Episode changes"
// @Success 200 {object} db.PodcastItem
// @Failure 400 {object} map[string]string
// @Router /podcastitems/{id} [patch]
func PatchPodcastItemByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcast db.PodcastItem

		err := db.GetPodcastItemByID(searchByIDQuery.ID, &podcast)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		var input PatchPodcastItem

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		db.DB.Model(&podcast).Updates(input)
		c.JSON(200, podcast)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// DownloadPodcastItem handles the download podcast item request.
// @Summary Queue an episode for download
// @Description Starts an asynchronous download for the selected episode.
// @Tags episodes
// @Produce json
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Success 200 {object} object
// @Failure 400 {object} map[string]string
// @Router /podcastitems/{id}/download [get]
func DownloadPodcastItem(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		go func() {
			if downloadErr := service.DownloadSingleEpisode(searchByIDQuery.ID); downloadErr != nil {
				logger.Log.Errorw("downloading episode", "error", downloadErr)
			}
		}()
		c.JSON(200, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// DeletePodcastItem handles the delete podcast item request.
// @Summary Delete an episode file
// @Description Starts asynchronous deletion of the selected episode's local file.
// @Tags episodes
// @Produce json
// @Security BasicAuth
// @Param id path string true "Episode ID"
// @Success 200 {object} object
// @Failure 400 {object} map[string]string
// @Router /podcastitems/{id}/delete [get]
func DeletePodcastItem(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		go func() {
			if deleteErr := service.DeleteEpisodeFile(searchByIDQuery.ID); deleteErr != nil {
				logger.Log.Errorw("deleting episode file", "error", deleteErr)
			}
		}()
		c.JSON(200, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// AddPodcast handles the add podcast request.
// @Summary Add a podcast
// @Description Saves a podcast from its feed URL and starts an asynchronous feed refresh.
// @Tags podcasts
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param podcast body AddPodcastData true "Podcast feed"
// @Success 200 {object} db.Podcast
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /podcasts [post]
func AddPodcast(c *gin.Context) {
	var addPodcastData AddPodcastData
	err := c.ShouldBindJSON(&addPodcastData)
	if err == nil {
		pod, addErr := service.AddPodcast(addPodcastData.URL)
		if addErr == nil {
			go func() {
				if refreshErr := service.RefreshEpisodes(); refreshErr != nil {
					logger.Log.Errorw("refreshing episodes", "error", refreshErr)
				}
			}()
			c.JSON(200, pod)
		} else {
			if v, ok := addErr.(*model.PodcastAlreadyExistsError); ok {
				c.JSON(409, gin.H{"message": v.Error()})
			} else {
				logger.Log.Error(addErr.Error())
				c.JSON(http.StatusBadRequest, gin.H{"message": addErr.Error()})
			}
		}
	} else {
		logger.Log.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	}
}

// GetAllTags handles the get all tags request.
// @Summary List tags
// @Tags tags
// @Produce json
// @Security BasicAuth
// @Success 200 {array} db.Tag
// @Failure 400 {object} map[string]string
// @Router /tags [get]
func GetAllTags(c *gin.Context) {
	tags, err := db.GetAllTags("")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	} else {
		c.JSON(200, tags)
	}
}

// GetTagByID handles the get tag by id request.
// @Summary Get a tag
// @Tags tags
// @Produce json
// @Security BasicAuth
// @Param id path string true "Tag ID"
// @Success 200 {object} db.Tag
// @Failure 400 {object} map[string]string
// @Router /tags/{id} [get]
func GetTagByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery
	if c.ShouldBindUri(&searchByIDQuery) == nil {
		tag, err := db.GetTagByID(searchByIDQuery.ID)
		if err == nil {
			c.JSON(200, tag)
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

func getBaseURL(c *gin.Context) string {
	setting, ok := c.MustGet("setting").(*db.Setting)
	if !ok {
		return ""
	}
	if setting.BaseURL == "" {
		url := location.Get(c)
		return fmt.Sprintf("%s://%s", url.Scheme, url.Host)
	}
	return setting.BaseURL
}

func createRss(items []db.PodcastItem, title, description, image string, c *gin.Context) model.RssPodcastData {
	rssItems := make([]model.RssItem, 0, len(items))
	url := getBaseURL(c)
	setting := db.GetOrCreateSetting()
	for i := range items {
		itemGUID := items[i].ID
		if setting.PassthroughPodcastGUID && strings.TrimSpace(items[i].GUID) != "" {
			itemGUID = items[i].GUID
		}
		rssItem := model.RssItem{
			Title:       items[i].Title,
			Description: items[i].Summary,
			Summary:     items[i].Summary,
			Image: model.RssItemImage{
				Text: items[i].Title,
				Href: fmt.Sprintf("%s/podcastitems/%s/image", url, items[i].ID),
			},
			EpisodeType: items[i].EpisodeType,
			Enclosure: model.RssItemEnclosure{
				URL:    fmt.Sprintf("%s/podcastitems/%s/file", url, items[i].ID),
				Length: fmt.Sprint(items[i].FileSize),
				Type:   "audio/mpeg",
			},
			PubDate: items[i].PubDate.Format("Mon, 02 Jan 2006 15:04:05 -0700"),
			GUID: model.RssItemGUID{
				IsPermaLink: "false",
				Text:        itemGUID,
			},
			Link:     fmt.Sprintf("%s/allTags", url),
			Text:     items[i].Title,
			Duration: fmt.Sprint(items[i].Duration),
		}
		rssItems = append(rssItems, rssItem)
	}

	imagePath := fmt.Sprintf("%s/webassets/blank.png", url)
	if image != "" {
		imagePath = image
	}

	return model.RssPodcastData{
		Itunes:  "http://www.itunes.com/dtds/podcast-1.0.dtd",
		Media:   "http://search.yahoo.com/mrss/",
		Version: "2.0",
		Atom:    "http://www.w3.org/2005/Atom",
		Psc:     "https://podlove.org/simple-chapters/",
		Content: "http://purl.org/rss/1.0/modules/content/",
		Channel: model.RssChannel{
			Item:        rssItems,
			Title:       title,
			Description: description,
			Summary:     description,
			Author:      "Podgrab Aggregation",
			Link:        fmt.Sprintf("%s/allTags", url),
			Image:       model.RssItemImage{Text: title, URL: imagePath},
		},
	}
}

// GetRssForPodcastByID handles the get rss for podcast by id request.
// @Summary Get a podcast RSS feed
// @Tags podcasts,feeds
// @Produce application/xml
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Success 200 {string} string "RSS XML"
// @Failure 400 {object} map[string]string
// @Router /podcasts/{id}/rss [get]
func GetRssForPodcastByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery
	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcast db.Podcast
		err := db.GetPodcastByID(searchByIDQuery.ID, &podcast)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		}
		podIDs := make([]string, 0, 1)
		podIDs = append(podIDs, searchByIDQuery.ID)
		items := *service.GetAllPodcastItemsByPodcastIDs(podIDs)

		description := podcast.Summary
		title := podcast.Title

		if err == nil {
			c.XML(200, createRss(items, title, description, podcast.Image, c))
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// GetRssForTagByID handles the get rss for tag by id request.
// @Summary Get a tag RSS feed
// @Tags tags,feeds
// @Produce application/xml
// @Security BasicAuth
// @Param id path string true "Tag ID"
// @Success 200 {string} string "RSS XML"
// @Failure 400 {object} map[string]string
// @Router /tags/{id}/rss [get]
func GetRssForTagByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery
	if c.ShouldBindUri(&searchByIDQuery) == nil {
		tag, err := db.GetTagByID(searchByIDQuery.ID)
		podIDs := make([]string, 0, len(tag.Podcasts))
		for i := range tag.Podcasts {
			podIDs = append(podIDs, tag.Podcasts[i].ID)
		}
		items := *service.GetAllPodcastItemsByPodcastIDs(podIDs)

		description := fmt.Sprintf("Playing episodes with tag : %s", tag.Label)
		title := fmt.Sprintf(" %s | Podgrab", tag.Label)

		if err == nil {
			c.XML(200, createRss(items, title, description, "", c))
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// GetRss handles the get rss request.
// @Summary Get the aggregate RSS feed
// @Tags feeds
// @Produce application/xml
// @Security BasicAuth
// @Success 200 {string} string "RSS XML"
// @Failure 400 {object} map[string]string
// @Router /rss [get]
func GetRss(c *gin.Context) {
	var items []db.PodcastItem

	if err := db.GetAllPodcastItems(&items); err != nil {
		logger.Log.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}

	title := "Podgrab"
	description := "Pograb playlist"

	c.XML(200, createRss(items, title, description, "", c))
}

// DeleteTagByID handles the delete tag by id request.
// @Summary Delete a tag
// @Tags tags
// @Produce json
// @Security BasicAuth
// @Param id path string true "Tag ID"
// @Success 204 "Tag deleted"
// @Failure 400 {object} map[string]string
// @Router /tags/{id} [delete]
func DeleteTagByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery
	if c.ShouldBindUri(&searchByIDQuery) == nil {
		err := service.DeleteTag(searchByIDQuery.ID)
		if err == nil {
			c.JSON(http.StatusNoContent, gin.H{})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// AddTag handles the add tag request.
// @Summary Add a tag
// @Tags tags
// @Accept json
// @Produce json
// @Security BasicAuth
// @Param tag body AddTagData true "Tag"
// @Success 200 {object} db.Tag
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /tags [post]
func AddTag(c *gin.Context) {
	var addTagData AddTagData
	err := c.ShouldBindJSON(&addTagData)
	if err == nil {
		tag, tagErr := service.AddTag(addTagData.Label, addTagData.Description)
		if tagErr == nil {
			c.JSON(200, tag)
		} else {
			if v, ok := tagErr.(*model.TagAlreadyExistsError); ok {
				c.JSON(409, gin.H{"message": v.Error()})
			} else {
				logger.Log.Error(tagErr.Error())
				c.JSON(http.StatusBadRequest, gin.H{"message": tagErr.Error()})
			}
		}
	} else {
		logger.Log.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	}
}

// AddTagToPodcast handles the add tag to podcast request.
// @Summary Add a tag to a podcast
// @Tags podcasts,tags
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Param tagID path string true "Tag ID"
// @Success 200 {object} object
// @Failure 400 {object} map[string]string
// @Router /podcasts/{id}/tags/{tagID} [post]
func AddTagToPodcast(c *gin.Context) {
	var addRemoveTagQuery AddRemoveTagQuery

	if c.ShouldBindUri(&addRemoveTagQuery) == nil {
		err := db.AddTagToPodcast(addRemoveTagQuery.ID, addRemoveTagQuery.TagID)
		if err == nil {
			c.JSON(200, gin.H{})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// RemoveTagFromPodcast handles the remove tag from podcast request.
// @Summary Remove a tag from a podcast
// @Tags podcasts,tags
// @Produce json
// @Security BasicAuth
// @Param id path string true "Podcast ID"
// @Param tagID path string true "Tag ID"
// @Success 200 {object} object
// @Failure 400 {object} map[string]string
// @Router /podcasts/{id}/tags/{tagID} [delete]
func RemoveTagFromPodcast(c *gin.Context) {
	var addRemoveTagQuery AddRemoveTagQuery

	if c.ShouldBindUri(&addRemoveTagQuery) == nil {
		err := db.RemoveTagFromPodcast(addRemoveTagQuery.ID, addRemoveTagQuery.TagID)
		if err == nil {
			c.JSON(200, gin.H{})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// UpdateSetting handles the update setting request.
// @Summary Update settings
// @Description Updates Podgrab's runtime settings. maxDownloadConcurrency must be at least 1.
// @Tags settings
// @Accept json
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Security BasicAuth
// @Param settings body SettingModel true "Settings"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /settings [post]
func UpdateSetting(c *gin.Context) {
	var settingModel SettingModel
	err := c.ShouldBind(&settingModel)

	if err == nil {
		if settingModel.MaxDownloadConcurrency < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "maxDownloadConcurrency must be at least 1"})
			return
		}
		err = service.UpdateSettings(
			settingModel.DownloadOnAdd,
			settingModel.InitialDownloadCount,
			settingModel.AutoDownload,
			settingModel.FileNameFormat,
			settingModel.PassthroughPodcastGUID,
			settingModel.DarkMode,
			settingModel.DownloadEpisodeImages,
			settingModel.GenerateNFOFile,
			settingModel.DontDownloadDeletedFromDisk,
			settingModel.BaseURL,
			settingModel.MaxDownloadConcurrency,
			settingModel.MaxDownloadKeep,
			settingModel.UserAgent,
		)
		if err == nil {
			c.JSON(200, gin.H{"message": "Success"})
		} else {
			c.JSON(http.StatusBadRequest, err)
		}
	} else {
		logger.Log.Error(err.Error())
		c.JSON(http.StatusBadRequest, err)
	}
}
