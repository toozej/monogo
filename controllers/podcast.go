package controllers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/akhilrex/podgrab/model"
	"github.com/akhilrex/podgrab/service"
	"github.com/gin-contrib/location"

	"github.com/akhilrex/podgrab/db"
	"github.com/gin-gonic/gin"
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

// GetAllPodcasts handles the get all podcasts request.
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

// GetPodcastByID handles the get podcast by id request.
func GetPodcastByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcast db.Podcast

		err := db.GetPodcastByID(searchByIDQuery.ID, &podcast)
		fmt.Println(err)
		c.JSON(200, podcast)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// PausePodcastByID handles the pause podcast by id request.
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
func GetPodcastItemsByPodcastID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcastItems []db.PodcastItem

		err := db.GetAllPodcastItemsByPodcastID(searchByIDQuery.ID, &podcastItems)
		fmt.Println(err)
		c.JSON(200, podcastItems)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// DownloadAllEpisodesByPodcastID handles the download all episodes by podcast id request.
func DownloadAllEpisodesByPodcastID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		err := service.SetAllEpisodesToDownload(searchByIDQuery.ID)
		fmt.Println(err)
		go func() {
			if err := service.RefreshEpisodes(); err != nil {
				fmt.Printf("Error refreshing episodes: %v\n", err)
			}
		}()
		c.JSON(200, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// GetAllPodcastItems handles the get all podcast items request.
func GetAllPodcastItems(c *gin.Context) {
	var filter model.EpisodesFilter
	err := c.ShouldBindQuery(&filter)
	if err != nil {
		fmt.Println(err.Error())
	}
	filter.VerifyPaginationValues()
	if podcastItems, totalCount, err := db.GetPaginatedPodcastItemsNew(filter); err == nil {
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
func GetPodcastItemByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcast db.PodcastItem

		err := db.GetPodcastItemByID(searchByIDQuery.ID, &podcast)
		fmt.Println(err)
		c.JSON(200, podcast)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// GetPodcastItemImageByID handles the get podcast item image by id request.
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
func GetPodcastItemFileByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		var podcast db.PodcastItem

		err := db.GetPodcastItemByID(searchByIDQuery.ID, &podcast)
		if err == nil {
			if _, err = os.Stat(podcast.DownloadPath); !os.IsNotExist(err) {
				c.Header("Content-Description", "File Transfer")
				c.Header("Content-Transfer-Encoding", "binary")
				c.Header("Content-Disposition", "attachment; filename="+path.Base(podcast.DownloadPath))
				c.Header("Content-Type", GetFileContentType(podcast.DownloadPath))
				c.File(podcast.DownloadPath)
			} else {
				c.Redirect(302, podcast.FileURL)
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// GetFileContentType handles the get file content type request.
func GetFileContentType(filePath string) string {
	file, err := os.Open(filePath) //nolint:gosec // G304: filePath is from database, managed by application
	if err != nil {
		return "application/octet-stream"
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Error closing file: %v\n", err)
		}
	}()
	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil {
		return "application/octet-stream"
	}
	return http.DetectContentType(buffer)
}

// MarkPodcastItemAsUnplayed handles the mark podcast item as unplayed request.
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
func DownloadPodcastItem(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		go func() {
			if err := service.DownloadSingleEpisode(searchByIDQuery.ID); err != nil {
				fmt.Printf("Error downloading episode: %v\n", err)
			}
		}()
		c.JSON(200, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// DeletePodcastItem handles the delete podcast item request.
func DeletePodcastItem(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery

	if c.ShouldBindUri(&searchByIDQuery) == nil {
		go func() {
			if err := service.DeleteEpisodeFile(searchByIDQuery.ID); err != nil {
				fmt.Printf("Error deleting episode file: %v\n", err)
			}
		}()
		c.JSON(200, gin.H{})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}
}

// AddPodcast handles the add podcast request.
func AddPodcast(c *gin.Context) {
	var addPodcastData AddPodcastData
	err := c.ShouldBindJSON(&addPodcastData)
	if err == nil {
		pod, err := service.AddPodcast(addPodcastData.URL)
		if err == nil {
			go func() {
				if err := service.RefreshEpisodes(); err != nil {
					fmt.Printf("Error refreshing episodes: %v\n", err)
				}
			}()
			c.JSON(200, pod)
		} else {
			if v, ok := err.(*model.PodcastAlreadyExistsError); ok {
				c.JSON(409, gin.H{"message": v.Error()})
			} else {
				log.Println(err.Error())
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			}
		}
	} else {
		log.Println(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	}
}

// GetAllTags handles the get all tags request.
func GetAllTags(c *gin.Context) {
	tags, err := db.GetAllTags("")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	} else {
		c.JSON(200, tags)
	}
}

// GetTagByID handles the get tag by id request.
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
	setting := c.MustGet("setting").(*db.Setting)
	if setting.BaseURL == "" {
		url := location.Get(c)
		return fmt.Sprintf("%s://%s", url.Scheme, url.Host)
	}
	return setting.BaseURL
}

func createRss(items []db.PodcastItem, title, description, image string, c *gin.Context) model.RssPodcastData {
	rssItems := make([]model.RssItem, 0, len(items))
	url := getBaseURL(c)
	for _, item := range items {
		rssItem := model.RssItem{
			Title:       item.Title,
			Description: item.Summary,
			Summary:     item.Summary,
			Image: model.RssItemImage{
				Text: item.Title,
				Href: fmt.Sprintf("%s/podcastitems/%s/image", url, item.ID),
			},
			EpisodeType: item.EpisodeType,
			Enclosure: model.RssItemEnclosure{
				URL:    fmt.Sprintf("%s/podcastitems/%s/file", url, item.ID),
				Length: fmt.Sprint(item.FileSize),
				Type:   "audio/mpeg",
			},
			PubDate: item.PubDate.Format("Mon, 02 Jan 2006 15:04:05 -0700"),
			GUID: model.RssItemGUID{
				IsPermaLink: "false",
				Text:        item.ID,
			},
			Link:     fmt.Sprintf("%s/allTags", url),
			Text:     item.Title,
			Duration: fmt.Sprint(item.Duration),
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
func GetRssForTagByID(c *gin.Context) {
	var searchByIDQuery SearchByIDQuery
	if c.ShouldBindUri(&searchByIDQuery) == nil {
		tag, err := db.GetTagByID(searchByIDQuery.ID)
		podIDs := make([]string, 0, len(tag.Podcasts))
		for _, pod := range tag.Podcasts {
			podIDs = append(podIDs, pod.ID)
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
func GetRss(c *gin.Context) {
	var items []db.PodcastItem

	if err := db.GetAllPodcastItems(&items); err != nil {
		fmt.Println(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
	}

	title := "Podgrab"
	description := "Pograb playlist"

	c.XML(200, createRss(items, title, description, "", c))
}

// DeleteTagByID handles the delete tag by id request.
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
func AddTag(c *gin.Context) {
	var addTagData AddTagData
	err := c.ShouldBindJSON(&addTagData)
	if err == nil {
		tag, err := service.AddTag(addTagData.Label, addTagData.Description)
		if err == nil {
			c.JSON(200, tag)
		} else {
			if v, ok := err.(*model.TagAlreadyExistsError); ok {
				c.JSON(409, gin.H{"message": v.Error()})
			} else {
				log.Println(err.Error())
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			}
		}
	} else {
		log.Println(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	}
}

// AddTagToPodcast handles the add tag to podcast request.
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
func UpdateSetting(c *gin.Context) {
	var model SettingModel
	err := c.ShouldBind(&model)

	if err == nil {
		err = service.UpdateSettings(model.DownloadOnAdd, model.InitialDownloadCount,
			model.AutoDownload, model.AppendDateToFileName, model.AppendEpisodeNumberToFileName,
			model.DarkMode, model.DownloadEpisodeImages, model.GenerateNFOFile, model.DontDownloadDeletedFromDisk, model.BaseURL,
			model.MaxDownloadConcurrency, model.UserAgent,
		)
		if err == nil {
			c.JSON(200, gin.H{"message": "Success"})
		} else {
			c.JSON(http.StatusBadRequest, err)
		}
	} else {
		fmt.Println(err.Error())
		c.JSON(http.StatusBadRequest, err)
	}
}
