// Package controllers implements HTTP request handlers for web pages and API endpoints.
package controllers

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/toozej/podgrab/db"
	"github.com/toozej/podgrab/internal/logger"
	"github.com/toozej/podgrab/model"
	"github.com/toozej/podgrab/service"
)

// SearchGPodderData represents search g podder data data.
type SearchGPodderData struct {
	Q            string `binding:"required" form:"q" json:"q" query:"q"`
	SearchSource string `binding:"required" form:"searchSource" json:"searchSource" query:"searchSource"`
}

// SettingModel represents setting model data.
type SettingModel struct {
	BaseURL                     string `form:"baseUrl" json:"baseUrl" query:"baseUrl"`
	UserAgent                   string `form:"userAgent" json:"userAgent" query:"userAgent"`
	FileNameFormat              string `form:"fileNameFormat" json:"fileNameFormat" query:"fileNameFormat"`
	InitialDownloadCount        int    `form:"initialDownloadCount" json:"initialDownloadCount" query:"initialDownloadCount"`
	MaxDownloadConcurrency      int    `form:"maxDownloadConcurrency" json:"maxDownloadConcurrency" query:"maxDownloadConcurrency"`
	MaxDownloadKeep             int    `form:"maxDownloadKeep" json:"maxDownloadKeep" query:"maxDownloadKeep"`
	AutoDownload                bool   `form:"autoDownload" json:"autoDownload" query:"autoDownload"`
	DownloadOnAdd               bool   `form:"downloadOnAdd" json:"downloadOnAdd" query:"downloadOnAdd"`
	DarkMode                    bool   `form:"darkMode" json:"darkMode" query:"darkMode"`
	DownloadEpisodeImages       bool   `form:"downloadEpisodeImages" json:"downloadEpisodeImages" query:"downloadEpisodeImages"`
	GenerateNFOFile             bool   `form:"generateNFOFile" json:"generateNFOFile" query:"generateNFOFile"`
	DontDownloadDeletedFromDisk bool   `form:"dontDownloadDeletedFromDisk" json:"dontDownloadDeletedFromDisk" query:"dontDownloadDeletedFromDisk"`
	PassthroughPodcastGUID      bool   `form:"passthroughPodcastGuid" json:"passthroughPodcastGuid" query:"passthroughPodcastGuid"`
}

var searchOptions = map[string]string{
	"itunes":       "iTunes",
	"podcastindex": "PodcastIndex",
}
var searchProvider = map[string]service.SearchService{
	"itunes":       new(service.ItunesService),
	"podcastindex": new(service.PodcastIndexService),
}

// AddPage handles the add page request.
func AddPage(c *gin.Context) {
	setting, ok := c.MustGet("setting").(*db.Setting)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
		return
	}
	c.HTML(http.StatusOK, "addPodcast.html", gin.H{"title": "Add Podcast", "setting": setting, "searchOptions": searchOptions})
}

// HomePage handles the home page request.
func HomePage(c *gin.Context) {
	podcasts := service.GetAllPodcasts("")
	setting, ok := c.MustGet("setting").(*db.Setting)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
		return
	}
	c.HTML(http.StatusOK, "index.html", gin.H{"title": "Podgrab", "podcasts": podcasts, "setting": setting})
}

// PodcastPage handles the podcast page request.
func PodcastPage(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery
	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcast db.Podcast

		if err := db.GetPodcastByID(searchByIDQuery.ID, &podcast); err == nil {
			var pagination model.Pagination
			if c.ShouldBindQuery(&pagination) == nil {
				var page, count int
				if page = pagination.Page; page == 0 {
					page = 1
				}
				if count = pagination.Count; count == 0 {
					count = 10
				}
				setting, ok := c.MustGet("setting").(*db.Setting)
				if !ok {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
					return
				}
				totalCount := len(podcast.PodcastItems)
				totalPages := int(math.Ceil(float64(totalCount) / float64(count)))
				nextPage, previousPage := 0, 0
				if page < totalPages {
					nextPage = page + 1
				}
				if page > 1 {
					previousPage = page - 1
				}

				from := (page - 1) * count
				to := page * count
				if to > totalCount {
					to = totalCount
				}
				c.HTML(http.StatusOK, "episodes.html", gin.H{
					"title":          podcast.Title,
					"podcastItems":   podcast.PodcastItems[from:to],
					"setting":        setting,
					"page":           page,
					"count":          count,
					"totalCount":     totalCount,
					"totalPages":     totalPages,
					"nextPage":       nextPage,
					"previousPage":   previousPage,
					"downloadedOnly": false,
					"podcastID":      searchByIDQuery.ID,
				})
			} else {
				c.JSON(http.StatusBadRequest, err)
			}
		} else {
			c.JSON(http.StatusBadRequest, err)
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

func getItemsToPlay(itemIDs []string, podcastID string, tagIDs []string) []db.PodcastItem {
	var items []db.PodcastItem
	switch {
	case len(itemIDs) > 0:
		toAdd, err := service.GetAllPodcastItemsByIDs(itemIDs)
		if err != nil {
			logger.Log.Errorw("getting podcast items by IDs", "error", err)
			return []db.PodcastItem{}
		}
		items = *toAdd
	case podcastID != "":
		pod := service.GetPodcastByID(podcastID)
		items = pod.PodcastItems
	case len(tagIDs) != 0:
		tags := service.GetTagsByIDs(tagIDs)
		podIDs := make([]string, 0, len(*tags)*5) // Preallocate with estimated capacity
		for i := range *tags {
			for j := range (*tags)[i].Podcasts {
				podIDs = append(podIDs, (*tags)[i].Podcasts[j].ID)
			}
		}
		items = *service.GetAllPodcastItemsByPodcastIDs(podIDs)
	}
	return items
}

// PlayerPage handles the player page request.
func PlayerPage(c *gin.Context) {
	itemIDs, hasItemIDs := c.GetQueryArray("itemIDs")
	podcastID, hasPodcastID := c.GetQuery("podcastID")
	tagIDs, hasTagIDs := c.GetQueryArray("tagIDs")
	title := "Podgrab"
	var items []db.PodcastItem
	var totalCount int64
	switch {
	case hasItemIDs:
		toAdd, err := service.GetAllPodcastItemsByIDs(itemIDs)
		if err != nil {
			logger.Log.Errorw("getting podcast items by IDs", "error", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to load items"})
			return
		}
		items = *toAdd
		totalCount = int64(len(items))
	case hasPodcastID:
		pod := service.GetPodcastByID(podcastID)
		items = pod.PodcastItems
		title = "Playing: " + pod.Title
		totalCount = int64(len(items))
	case hasTagIDs:
		tags := service.GetTagsByIDs(tagIDs)
		tagNames := make([]string, 0, len(*tags))
		podIDs := make([]string, 0, len(*tags)*5) // Preallocate with estimated capacity
		for i := range *tags {
			tagNames = append(tagNames, (*tags)[i].Label)
			for j := range (*tags)[i].Podcasts {
				podIDs = append(podIDs, (*tags)[i].Podcasts[j].ID)
			}
		}
		items = *service.GetAllPodcastItemsByPodcastIDs(podIDs)
		if len(tagNames) == 1 {
			title = fmt.Sprintf("Playing episodes with tag : %s", (tagNames[0]))
		} else {
			title = fmt.Sprintf("Playing episodes with tags : %s", strings.Join(tagNames, ", "))
		}
	default:
		title = "Playing Latest Episodes"
		if err := db.GetPaginatedPodcastItems(1, 20, nil, nil, time.Time{}, &items, &totalCount); err != nil {
			logger.Log.Error(err.Error())
		}
	}
	setting, ok := c.MustGet("setting").(*db.Setting)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
		return
	}

	c.HTML(http.StatusOK, "player.html", gin.H{
		"title":          title,
		"podcastItems":   items,
		"setting":        setting,
		"count":          len(items),
		"totalCount":     totalCount,
		"downloadedOnly": true,
	})
}

// SettingsPage handles the settings page request.
func SettingsPage(c *gin.Context) {
	setting, ok := c.MustGet("setting").(*db.Setting)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
		return
	}
	diskStats, err := db.GetPodcastEpisodeDiskStats()
	if err != nil {
		logger.Log.Errorw("getting disk stats", "error", err)
	}
	c.HTML(http.StatusOK, "settings.html", gin.H{
		"setting":   setting,
		"title":     "Update your preferences",
		"diskStats": diskStats,
	})
}

// BackupsPage handles the backups page request.
func BackupsPage(c *gin.Context) {
	files, err := service.GetAllBackupFiles()
	var allFiles []interface{}
	setting, ok := c.MustGet("setting").(*db.Setting)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
		return
	}

	for _, file := range files {
		arr := strings.Split(file, string(os.PathSeparator))
		name := arr[len(arr)-1]
		subsplit := strings.Split(name, "_")
		dateStr := subsplit[2]
		date, parseErr := time.Parse("2006.01.02", dateStr)
		if parseErr == nil {
			toAdd := map[string]interface{}{
				"date": date,
				"name": name,
				"path": strings.ReplaceAll(file, string(os.PathSeparator), "/"),
			}
			allFiles = append(allFiles, toAdd)
		}
	}

	if err == nil {
		c.HTML(http.StatusOK, "backups.html", gin.H{
			"backups": allFiles,
			"title":   "Backups",
			"setting": setting,
		})
	} else {
		c.JSON(http.StatusBadRequest, err)
	}
}

func getSortOptions() interface{} {
	return []struct {
		Label, Value string
	}{
		{"Release (asc)", "release_asc"},
		{"Release (desc)", "release_desc"},
		{"Duration (asc)", "duration_asc"},
		{"Duration (desc)", "duration_desc"},
	}
}

// AllEpisodesPage handles the all episodes page request.
func AllEpisodesPage(c *gin.Context) {
	var filter model.EpisodesFilter
	// Use default filter values if binding fails
	if err := c.ShouldBindQuery(&filter); err != nil {
		logger.Log.Errorw("binding query parameters", "error", err)
	}
	filter.VerifyPaginationValues()
	setting, ok := c.MustGet("setting").(*db.Setting)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
		return
	}
	podcasts := service.GetAllPodcasts("")
	tags, err := db.GetAllTags("")
	if err != nil {
		logger.Log.Errorw("getting all tags", "error", err)
		tags = &[]db.Tag{}
	}
	toReturn := gin.H{
		"title":        "All Episodes",
		"podcastItems": []db.PodcastItem{},
		"setting":      setting,
		"page":         filter.Page,
		"count":        filter.Count,
		"filter":       filter,
		"podcasts":     podcasts,
		"tags":         tags,
		"sortOptions":  getSortOptions(),
	}
	c.HTML(http.StatusOK, "episodes_new.html", toReturn)
}

// AllTagsPage handles the all tags page request.
func AllTagsPage(c *gin.Context) {
	var pagination model.Pagination
	var page, count int
	// Use default pagination values if binding fails
	if err := c.ShouldBindQuery(&pagination); err != nil {
		logger.Log.Errorw("binding query parameters", "error", err)
	}
	if page = pagination.Page; page == 0 {
		page = 1
	}
	if count = pagination.Count; count == 0 {
		count = 10
	}

	var tags []db.Tag
	var totalCount int64

	if err := db.GetPaginatedTags(page, count,
		&tags, &totalCount); err == nil {
		setting, ok := c.MustGet("setting").(*db.Setting)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
			return
		}
		totalPages := math.Ceil(float64(totalCount) / float64(count))
		nextPage, previousPage := 0, 0
		if float64(page) < totalPages {
			nextPage = page + 1
		}
		if page > 1 {
			previousPage = page - 1
		}
		toReturn := gin.H{
			"title":        "Tags",
			"tags":         tags,
			"setting":      setting,
			"page":         page,
			"count":        count,
			"totalCount":   totalCount,
			"totalPages":   totalPages,
			"nextPage":     nextPage,
			"previousPage": previousPage,
		}
		c.HTML(http.StatusOK, "tags.html", toReturn)
	} else {
		c.JSON(http.StatusBadRequest, err)
	}
}

// Search handles the search request.
func Search(c *gin.Context) {
	var searchQuery SearchGPodderData
	if c.ShouldBindQuery(&searchQuery) == nil {
		var searcher service.SearchService
		var isValidSearchProvider bool
		if searcher, isValidSearchProvider = searchProvider[searchQuery.SearchSource]; !isValidSearchProvider {
			searcher = new(service.PodcastIndexService)
		}

		data := searcher.Query(searchQuery.Q)
		allPodcasts := service.GetAllPodcasts("")

		urls := make(map[string]string, len(*allPodcasts))
		for i := range *allPodcasts {
			urls[(*allPodcasts)[i].URL] = (*allPodcasts)[i].ID
		}
		for i := range data {
			_, ok := urls[data[i].URL]
			data[i].AlreadySaved = ok
		}
		c.JSON(200, data)
	}
}

// GetOmpl handles the get ompl request.
func GetOmpl(c *gin.Context) {
	usePodgrabLink := c.DefaultQuery("usePodgrabLink", "false") == "true"

	data, err := service.ExportOmpl(usePodgrabLink, getBaseURL(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=podgrab-export.opml")
	c.Data(200, "text/xml", data)
}

// UploadOpml handles the upload opml request.
func UploadOpml(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Log.Errorw("closing file", "error", closeErr)
		}
	}()

	buf := bytes.NewBuffer(nil)
	if _, copyErr := io.Copy(buf, file); copyErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}
	content := buf.String()
	err = service.AddOpml(content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	} else {
		c.JSON(200, gin.H{"success": "File uploaded"})
	}
}

// AddNewPodcast handles the add new podcast request.
func AddNewPodcast(c *gin.Context) {
	var addPodcastData AddPodcastData
	err := c.ShouldBind(&addPodcastData)

	if err == nil {
		_, err = service.AddPodcast(addPodcastData.URL)
		if err == nil {
			go func() {
				if refreshErr := service.RefreshEpisodes(); refreshErr != nil {
					logger.Log.Errorw("refreshing episodes", "error", refreshErr)
				}
			}()
			c.Redirect(http.StatusFound, "/")
		} else {
			c.JSON(http.StatusBadRequest, err)
		}
	} else {
		c.JSON(http.StatusBadRequest, err)
	}
}
