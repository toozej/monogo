// Package service implements business logic for podcast management and downloads.
package service

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TheHippo/podcastindex"
	"github.com/akhilrex/podgrab/db"
	"github.com/akhilrex/podgrab/model"
	"github.com/antchfx/xmlquery"
	strip "github.com/grokify/html-strip-tags-go"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Logger represents logger.
var Logger *zap.SugaredLogger

func init() {
	zapper, _ := zap.NewProduction()
	Logger = zapper.Sugar()
	// Note: Sync is called when the program exits, not here in init
	_ = zapper.Sync()
}

// ParseOpml parse opml.
func ParseOpml(content string) (model.OpmlModel, error) {
	var response model.OpmlModel
	err := xml.Unmarshal([]byte(content), &response)
	return response, err
}

// FetchURL is
func FetchURL(url string) (model.PodcastData, []byte, error) {
	body, err := makeQuery(url)
	if err != nil {
		return model.PodcastData{}, nil, err
	}
	var response model.PodcastData
	err = xml.Unmarshal(body, &response)
	return response, body, err
}

// GetPodcastByID get podcast by id.
func GetPodcastByID(id string) *db.Podcast {
	var podcast db.Podcast

	if err := db.GetPodcastByID(id, &podcast); err != nil {
		fmt.Printf("Error getting podcast by ID: %v\n", err)
	}

	return &podcast
}

// GetPodcastItemByID get podcast item by id.
func GetPodcastItemByID(id string) *db.PodcastItem {
	var podcastItem db.PodcastItem

	if err := db.GetPodcastItemByID(id, &podcastItem); err != nil {
		fmt.Printf("Error getting podcast item by ID: %v\n", err)
	}

	return &podcastItem
}

// GetAllPodcastItemsByIDs get all podcast items by ids.
func GetAllPodcastItemsByIDs(podcastItemIDs []string) (*[]db.PodcastItem, error) {
	return db.GetAllPodcastItemsByIDs(podcastItemIDs)
}

// GetAllPodcastItemsByPodcastIDs get all podcast items by podcast ids.
func GetAllPodcastItemsByPodcastIDs(podcastIDs []string) *[]db.PodcastItem {
	var podcastItems []db.PodcastItem

	if err := db.GetAllPodcastItemsByPodcastIDs(podcastIDs, &podcastItems); err != nil {
		fmt.Printf("Error getting podcast items by podcast IDs: %v\n", err)
	}
	return &podcastItems
}

// GetTagsByIDs get tags by ids.
func GetTagsByIDs(ids []string) *[]db.Tag {
	tags, _ := db.GetTagsByIDs(ids)

	return tags
}

// GetAllPodcasts get all podcasts.
func GetAllPodcasts(sorting string) *[]db.Podcast {
	var podcasts []db.Podcast
	if err := db.GetAllPodcasts(&podcasts, sorting); err != nil {
		fmt.Printf("Error getting all podcasts: %v\n", err)
	}

	stats, _ := db.GetPodcastEpisodeStats()

	type Key struct {
		PodcastID      string
		DownloadStatus db.DownloadStatus
	}
	countMap := make(map[Key]int)
	sizeMap := make(map[Key]int64)
	for _, stat := range *stats {
		countMap[Key{stat.PodcastID, stat.DownloadStatus}] = stat.Count
		sizeMap[Key{stat.PodcastID, stat.DownloadStatus}] = stat.Size
	}
	toReturn := make([]db.Podcast, 0, len(podcasts))
	for _, podcast := range podcasts {
		podcast.DownloadedEpisodesCount = countMap[Key{podcast.ID, db.Downloaded}]
		podcast.DownloadingEpisodesCount = countMap[Key{podcast.ID, db.NotDownloaded}]
		podcast.AllEpisodesCount = podcast.DownloadedEpisodesCount + podcast.DownloadingEpisodesCount + countMap[Key{podcast.ID, db.Deleted}]

		podcast.DownloadedEpisodesSize = sizeMap[Key{podcast.ID, db.Downloaded}]
		podcast.DownloadingEpisodesSize = sizeMap[Key{podcast.ID, db.NotDownloaded}]
		podcast.AllEpisodesSize = podcast.DownloadedEpisodesSize + podcast.DownloadingEpisodesSize + sizeMap[Key{podcast.ID, db.Deleted}]

		toReturn = append(toReturn, podcast)
	}
	return &toReturn
}

// AddOpml add opml.
func AddOpml(content string) error {
	model, err := ParseOpml(content)
	if err != nil {
		fmt.Println(err.Error())
		return errors.New("invalid file format")
	}
	var wg sync.WaitGroup
	for _, outline := range model.Body.Outline {
		if outline.XMLURL != "" {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				if _, err := AddPodcast(url); err != nil {
					fmt.Printf("Error adding podcast from OPML: %v\n", err)
				}
			}(outline.XMLURL)
		}

		for _, innerOutline := range outline.Outline {
			if innerOutline.XMLURL != "" {
				wg.Add(1)
				go func(url string) {
					defer wg.Done()
					if _, err := AddPodcast(url); err != nil {
						fmt.Printf("Error adding podcast from OPML: %v\n", err)
					}
				}(innerOutline.XMLURL)
			}
		}
	}
	wg.Wait()
	go func() {
		if err := RefreshEpisodes(); err != nil {
			fmt.Printf("Error refreshing episodes: %v\n", err)
		}
	}()
	return nil
}

// ExportOmpl export ompl.
func ExportOmpl(usePodgrabLink bool, baseURL string) ([]byte, error) {
	podcasts := GetAllPodcasts("")

	outlines := make([]model.OpmlOutline, 0, len(*podcasts))
	for _, podcast := range *podcasts {
		xmlURL := podcast.URL
		if usePodgrabLink {
			xmlURL = fmt.Sprintf("%s/podcasts/%s/rss", baseURL, podcast.ID)
		}

		toAdd := model.OpmlOutline{
			AttrText: podcast.Summary,
			Type:     "rss",
			XMLURL:   xmlURL,
			Title:    podcast.Title,
		}
		outlines = append(outlines, toAdd)
	}

	toExport := model.OpmlExportModel{
		Head: model.OpmlExportHead{
			Title:       "Podgrab Feed Export",
			DateCreated: time.Now(),
		},
		Body: model.OpmlBody{
			Outline: outlines,
		},
		Version: "2.0",
	}

	data, err := xml.MarshalIndent(toExport, "", "    ")
	if err != nil {
		return nil, err
	}
	//	fmt.Println(xml.Header + string(data))
	data = []byte(xml.Header + string(data))
	return data, err
}

func getItunesImageURL(body []byte) string {
	doc, err := xmlquery.Parse(strings.NewReader(string(body)))
	if err != nil {
		return ""
	}
	channel, err := xmlquery.Query(doc, "//channel")
	if err != nil {
		return ""
	}

	iimage := channel.SelectElement("itunes:image")
	if iimage == nil {
		return ""
	}
	for _, attr := range iimage.Attr {
		if attr.Name.Local == "href" {
			return attr.Value
		}
	}
	return ""
}

// AddPodcast add podcast.
func AddPodcast(url string) (db.Podcast, error) {
	var podcast db.Podcast
	err := db.GetPodcastByURL(url, &podcast)
	setting := db.GetOrCreateSetting()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		data, body, err := FetchURL(url)
		if err != nil {
			fmt.Println(err.Error())
			Logger.Errorw("Error adding podcast", err)
			return db.Podcast{}, err
		}

		podcast := db.Podcast{
			Title:   data.Channel.Title,
			Summary: strip.StripTags(data.Channel.Summary),
			Author:  data.Channel.Author,
			Image:   data.Channel.Image.URL,
			URL:     url,
		}

		if podcast.Image == "" {
			podcast.Image = getItunesImageURL(body)
		}

		err = db.CreatePodcast(&podcast)
		go func() {
			if _, err := DownloadPodcastCoverImage(podcast.Image, podcast.Title); err != nil {
				fmt.Printf("Error downloading podcast cover image: %v\n", err)
			}
		}()
		if setting.GenerateNFOFile {
			go func() {
				if err := CreateNfoFile(&podcast); err != nil {
					fmt.Printf("Error creating NFO file: %v\n", err)
				}
			}()
		}
		return podcast, err
	}

	return podcast, &model.PodcastAlreadyExistsError{URL: url}
}

// AddPodcastItems add podcast items.
func AddPodcastItems(podcast *db.Podcast, newPodcast bool) error {
	// fmt.Println("Creating: " + podcast.ID)
	data, _, err := FetchURL(podcast.URL)
	if err != nil {
		// log.Fatal(err)
		return err
	}
	setting := db.GetOrCreateSetting()
	limit := setting.InitialDownloadCount
	// if len(data.Channel.Item) < limit {
	// 	limit = len(data.Channel.Item)
	// }
	var allGuids []string
	for i := 0; i < len(data.Channel.Item); i++ {
		obj := data.Channel.Item[i]
		allGuids = append(allGuids, obj.GUID.Text)
	}

	existingItems, err := db.GetPodcastItemsByPodcastIDAndGUIDs(podcast.ID, allGuids)
	keyMap := make(map[string]int)

	for _, item := range *existingItems {
		keyMap[item.GUID] = 1
	}
	var latestDate = time.Time{}
	var itemsAdded = make(map[string]string)
	for i := 0; i < len(data.Channel.Item); i++ {
		obj := data.Channel.Item[i]
		var podcastItem db.PodcastItem
		_, keyExists := keyMap[obj.GUID.Text]
		if !keyExists {
			duration, _ := strconv.Atoi(obj.Duration)
			toParse := strings.TrimSpace(obj.PubDate)

			pubDate, _ := time.Parse(time.RFC1123Z, toParse)
			if (pubDate.Equal(time.Time{})) {
				pubDate, _ = time.Parse(time.RFC1123, toParse)
			}
			if (pubDate.Equal(time.Time{})) {
				//	RFC1123     = "Mon, 02 Jan 2006 15:04:05 MST"
				modifiedRFC1123 := "Mon, 2 Jan 2006 15:04:05 MST"
				pubDate, _ = time.Parse(modifiedRFC1123, toParse)
			}
			if (pubDate.Equal(time.Time{})) {
				//	RFC1123Z    = "Mon, 02 Jan 2006 15:04:05 -0700" // RFC1123 with numeric zone
				modifiedRFC1123Z := "Mon, 2 Jan 2006 15:04:05 -0700"
				pubDate, _ = time.Parse(modifiedRFC1123Z, toParse)
			}
			if (pubDate.Equal(time.Time{})) {
				//	RFC1123Z    = "Mon, 02 Jan 2006 15:04:05 -0700" // RFC1123 with numeric zone
				modifiedRFC1123Z := "Mon, 02 Jan 2006 15:04:05 -0700"
				pubDate, _ = time.Parse(modifiedRFC1123Z, toParse)
			}

			if (pubDate.Equal(time.Time{})) {
				fmt.Printf("Cant format date : %s", obj.PubDate)
			}

			if latestDate.Before(pubDate) {
				latestDate = pubDate
			}

			var downloadStatus db.DownloadStatus
			if setting.AutoDownload {
				if !newPodcast {
					downloadStatus = db.NotDownloaded
				} else {
					if i < limit {
						downloadStatus = db.NotDownloaded
					} else {
						downloadStatus = db.Deleted
					}
				}
			} else {
				downloadStatus = db.Deleted
			}

			if newPodcast && !setting.DownloadOnAdd {
				downloadStatus = db.Deleted
			}

			if podcast.IsPaused {
				downloadStatus = db.Deleted
			}

			summary := strip.StripTags(obj.Summary)
			if summary == "" {
				summary = strip.StripTags(obj.Description)
			}

			podcastItem = db.PodcastItem{
				PodcastID:      podcast.ID,
				Title:          obj.Title,
				Summary:        summary,
				EpisodeType:    obj.EpisodeType,
				Duration:       duration,
				PubDate:        pubDate,
				FileURL:        obj.Enclosure.URL,
				GUID:           obj.GUID.Text,
				Image:          obj.Image.Href,
				DownloadStatus: downloadStatus,
			}
			if err := db.CreatePodcastItem(&podcastItem); err != nil {
				fmt.Printf("Error creating podcast item: %v\n", err)
			}
			itemsAdded[podcastItem.ID] = podcastItem.FileURL
		}
	}
	if (latestDate != time.Time{}) {
		if err := db.UpdateLastEpisodeDateForPodcast(podcast.ID, latestDate); err != nil {
			fmt.Printf("Error updating last episode date: %v\n", err)
		}
	}
	// go updateSizeFromURL(itemsAdded)
	return err
}

//nolint:unused // Function reserved for future use (see line 387)
func updateSizeFromURL(itemURLMap map[string]string) {
	for id, url := range itemURLMap {
		size, err := GetFileSizeFromURL(url)
		if err != nil {
			size = 1
		}

		if err := db.UpdatePodcastItemFileSize(id, size); err != nil {
			fmt.Printf("Error updating podcast item file size: %v\n", err)
		}
	}
}

// UpdateAllFileSizes update all file sizes.
func UpdateAllFileSizes() {
	items, err := db.GetAllPodcastItemsWithoutSize()
	if err != nil {
		return
	}
	for _, item := range *items {
		var size int64
		if item.DownloadStatus == db.Downloaded {
			size, _ = GetFileSize(item.DownloadPath)
		} else {
			size, _ = GetFileSizeFromURL(item.FileURL)
		}
		if err := db.UpdatePodcastItemFileSize(item.ID, size); err != nil {
			fmt.Printf("Error updating podcast item file size: %v\n", err)
		}
	}
}

// SetPodcastItemAsQueuedForDownload set podcast item as queued for download.
func SetPodcastItemAsQueuedForDownload(id string) error {
	var podcastItem db.PodcastItem
	err := db.GetPodcastItemByID(id, &podcastItem)
	if err != nil {
		return err
	}
	podcastItem.DownloadStatus = db.NotDownloaded

	return db.UpdatePodcastItem(&podcastItem)
}

// DownloadMissingImages download missing images.
func DownloadMissingImages() error {
	setting := db.GetOrCreateSetting()
	if !setting.DownloadEpisodeImages {
		fmt.Println("No Need To Download Images")
		return nil
	}
	items, err := db.GetAllPodcastItemsWithoutImage()
	if err != nil {
		return err
	}
	for _, item := range *items {
		if err := downloadImageLocally(item.ID); err != nil {
			fmt.Printf("Error downloading image locally: %v\n", err)
		}
	}
	return nil
}

func downloadImageLocally(podcastItemID string) error {
	var podcastItem db.PodcastItem
	err := db.GetPodcastItemByID(podcastItemID, &podcastItem)
	if err != nil {
		return err
	}

	path, err := DownloadImage(podcastItem.Image, podcastItem.ID, podcastItem.Podcast.Title)
	if err != nil {
		return err
	}

	podcastItem.LocalImage = path

	return db.UpdatePodcastItem(&podcastItem)
}

// SetPodcastItemBookmarkStatus set podcast item bookmark status.
func SetPodcastItemBookmarkStatus(id string, bookmark bool) error {
	var podcastItem db.PodcastItem
	err := db.GetPodcastItemByID(id, &podcastItem)
	if err != nil {
		return err
	}
	if bookmark {
		podcastItem.BookmarkDate = time.Now()
	} else {
		podcastItem.BookmarkDate = time.Time{}
	}
	return db.UpdatePodcastItem(&podcastItem)
}

// SetPodcastItemAsDownloaded set podcast item as downloaded.
func SetPodcastItemAsDownloaded(id string, location string) error {
	var podcastItem db.PodcastItem

	err := db.GetPodcastItemByID(id, &podcastItem)
	if err != nil {
		fmt.Println("Location", err.Error())
		return err
	}

	size, err := GetFileSize(location)
	if err == nil {
		podcastItem.FileSize = size
	}

	podcastItem.DownloadDate = time.Now()
	podcastItem.DownloadPath = location
	podcastItem.DownloadStatus = db.Downloaded

	return db.UpdatePodcastItem(&podcastItem)
}

// SetPodcastItemAsNotDownloaded set podcast item as not downloaded.
func SetPodcastItemAsNotDownloaded(id string, downloadStatus db.DownloadStatus) error {
	var podcastItem db.PodcastItem
	err := db.GetPodcastItemByID(id, &podcastItem)
	if err != nil {
		return err
	}
	podcastItem.DownloadDate = time.Time{}
	podcastItem.DownloadPath = ""
	podcastItem.DownloadStatus = downloadStatus

	return db.UpdatePodcastItem(&podcastItem)
}

// SetPodcastItemPlayedStatus set podcast item played status.
func SetPodcastItemPlayedStatus(id string, isPlayed bool) error {
	var podcastItem db.PodcastItem
	err := db.GetPodcastItemByID(id, &podcastItem)
	if err != nil {
		return err
	}
	podcastItem.IsPlayed = isPlayed
	return db.UpdatePodcastItem(&podcastItem)
}

// SetAllEpisodesToDownload set all episodes to download.
func SetAllEpisodesToDownload(podcastID string) error {
	var podcast db.Podcast
	err := db.GetPodcastByID(podcastID, &podcast)
	if err != nil {
		return err
	}
	if err := AddPodcastItems(&podcast, false); err != nil {
		fmt.Printf("Error adding podcast items: %v\n", err)
	}
	return db.SetAllEpisodesToDownload(podcastID)
}

// GetPodcastPrefix get podcast prefix.
func GetPodcastPrefix(item *db.PodcastItem, setting *db.Setting) string {
	prefix := ""
	if setting.AppendEpisodeNumberToFileName {
		seq, err := db.GetEpisodeNumber(item.ID, item.PodcastID)
		if err == nil {
			prefix = strconv.Itoa(seq)
		}
	}
	if setting.AppendDateToFileName {
		toAppend := item.PubDate.Format("2006-01-02")
		if prefix == "" {
			prefix = toAppend
		} else {
			prefix = prefix + "-" + toAppend
		}
	}
	return prefix
}

// DownloadMissingEpisodes download missing episodes.
func DownloadMissingEpisodes() error {
	// Early return if database is not available (e.g., during test cleanup)
	if db.DB == nil {
		return nil
	}

	const jobName = "DownloadMissingEpisodes"
	lock := db.GetLock(jobName)
	if lock.IsLocked() {
		fmt.Println(jobName + " is locked")
		return nil
	}
	db.Lock(jobName, 120)
	setting := db.GetOrCreateSetting()

	data, err := db.GetAllPodcastItemsToBeDownloaded()

	fmt.Println("Processing episodes: ", strconv.Itoa(len(*data)))
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for index, item := range *data {
		wg.Add(1)
		go func(item db.PodcastItem, setting db.Setting) {
			defer wg.Done()
			url, _ := Download(item.FileURL, item.Title, item.Podcast.Title, GetPodcastPrefix(&item, &setting))
			if err := SetPodcastItemAsDownloaded(item.ID, url); err != nil {
				fmt.Printf("Error setting podcast item as downloaded: %v\n", err)
			}
		}(item, *setting)

		if index%setting.MaxDownloadConcurrency == 0 {
			wg.Wait()
		}
	}
	wg.Wait()
	db.Unlock(jobName)
	return nil
}

// CheckMissingFiles check missing files.
func CheckMissingFiles() error {
	data, err := db.GetAllPodcastItemsAlreadyDownloaded()
	setting := db.GetOrCreateSetting()

	// fmt.Println("Processing episodes: ", strconv.Itoa(len(*data)))
	if err != nil {
		return err
	}
	for _, item := range *data {
		fileExists := FileExists(item.DownloadPath)
		if !fileExists {
			if setting.DontDownloadDeletedFromDisk {
				if err := SetPodcastItemAsNotDownloaded(item.ID, db.Deleted); err != nil {
					fmt.Printf("Error setting podcast item as not downloaded: %v\n", err)
				}
			} else {
				if err := SetPodcastItemAsNotDownloaded(item.ID, db.NotDownloaded); err != nil {
					fmt.Printf("Error setting podcast item as not downloaded: %v\n", err)
				}
			}
		}
	}
	return nil
}

// DeleteEpisodeFile delete episode file.
func DeleteEpisodeFile(podcastItemID string) error {
	var podcastItem db.PodcastItem
	err := db.GetPodcastItemByID(podcastItemID, &podcastItem)

	// fmt.Println("Processing episodes: ", strconv.Itoa(len(*data)))
	if err != nil {
		return err
	}

	err = DeleteFile(podcastItem.DownloadPath)

	if err != nil && !os.IsNotExist(err) {
		fmt.Println(err.Error())
		return err
	}

	if podcastItem.LocalImage != "" {
		go func() {
			if err := DeleteFile(podcastItem.LocalImage); err != nil {
				fmt.Printf("Error deleting file: %v\n", err)
			}
		}()
	}

	return SetPodcastItemAsNotDownloaded(podcastItem.ID, db.Deleted)
}

// DownloadSingleEpisode download single episode.
func DownloadSingleEpisode(podcastItemID string) error {
	var podcastItem db.PodcastItem
	err := db.GetPodcastItemByID(podcastItemID, &podcastItem)

	// fmt.Println("Processing episodes: ", strconv.Itoa(len(*data)))
	if err != nil {
		return err
	}

	setting := db.GetOrCreateSetting()
	if err := SetPodcastItemAsQueuedForDownload(podcastItemID); err != nil {
		fmt.Printf("Error setting podcast item as queued for download: %v\n", err)
	}

	url, err := Download(podcastItem.FileURL, podcastItem.Title, podcastItem.Podcast.Title, GetPodcastPrefix(&podcastItem, setting))

	if err != nil {
		fmt.Println(err.Error())
		return err
	}
	err = SetPodcastItemAsDownloaded(podcastItem.ID, url)

	if setting.DownloadEpisodeImages {
		if err := downloadImageLocally(podcastItem.ID); err != nil {
			fmt.Printf("Error downloading image locally: %v\n", err)
		}
	}
	return err
}

// RefreshEpisodes refresh episodes.
func RefreshEpisodes() error {
	var data []db.Podcast
	err := db.GetAllPodcasts(&data, "")

	if err != nil {
		return err
	}
	for _, item := range data {
		isNewPodcast := item.LastEpisode == nil
		if isNewPodcast {
			fmt.Println(item.Title)
			db.ForceSetLastEpisodeDate(item.ID)
		}
		if err := AddPodcastItems(&item, isNewPodcast); err != nil {
			fmt.Printf("Error adding podcast items: %v\n", err)
		}
	}

	// Spawn background download (DownloadMissingEpisodes handles nil DB gracefully)
	go func() {
		if err := DownloadMissingEpisodes(); err != nil {
			fmt.Printf("Error downloading missing episodes: %v\n", err)
		}
	}()

	return nil
}

// DeletePodcastEpisodes delete podcast episodes.
func DeletePodcastEpisodes(id string) error {
	var podcast db.Podcast

	err := db.GetPodcastByID(id, &podcast)
	if err != nil {
		return err
	}
	var podcastItems []db.PodcastItem

	err = db.GetAllPodcastItemsByPodcastID(id, &podcastItems)
	if err != nil {
		return err
	}
	for _, item := range podcastItems {
		if err := DeleteFile(item.DownloadPath); err != nil {
			fmt.Printf("Error deleting file: %v\n", err)
		}
		if item.LocalImage != "" {
			if err := DeleteFile(item.LocalImage); err != nil {
				fmt.Printf("Error deleting file: %v\n", err)
			}
		}
		if err := SetPodcastItemAsNotDownloaded(item.ID, db.Deleted); err != nil {
			fmt.Printf("Error setting podcast item as not downloaded: %v\n", err)
		}
	}
	return nil
}

// DeletePodcast delete podcast.
func DeletePodcast(id string, deleteFiles bool) error {
	var podcast db.Podcast

	err := db.GetPodcastByID(id, &podcast)
	if err != nil {
		return err
	}
	var podcastItems []db.PodcastItem

	err = db.GetAllPodcastItemsByPodcastID(id, &podcastItems)
	if err != nil {
		return err
	}
	for _, item := range podcastItems {
		if deleteFiles {
			if err := DeleteFile(item.DownloadPath); err != nil {
				fmt.Printf("Error deleting file: %v\n", err)
			}
			if item.LocalImage != "" {
				if err := DeleteFile(item.LocalImage); err != nil {
					fmt.Printf("Error deleting file: %v\n", err)
				}
			}
		}
		if err := db.DeletePodcastItemByID(item.ID); err != nil {
			fmt.Printf("Error deleting podcast item: %v\n", err)
		}
	}

	err = deletePodcastFolder(podcast.Title)
	if err != nil {
		return err
	}

	err = db.DeletePodcastByID(id)
	if err != nil {
		return err
	}
	return nil
}

// DeleteTag delete tag.
func DeleteTag(id string) error {
	if err := db.UntagAllByTagID(id); err != nil {
		fmt.Printf("Error untagging by tag ID: %v\n", err)
	}
	err := db.DeleteTagByID(id)
	if err != nil {
		return err
	}
	return nil
}

func makeQuery(url string) ([]byte, error) {
	// link := "https://www.goodreads.com/search/index.xml?q=Good%27s+Omens&key=" + "jCmNlIXjz29GoB8wYsrd0w"
	// link := "https://www.goodreads.com/search/index.xml?key=jCmNlIXjz29GoB8wYsrd0w&q=Ender%27s+Game"
	fmt.Println(url)
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}()
	fmt.Println("Response status:", resp.Status)
	body, err := io.ReadAll(resp.Body)

	return body, err
}

// GetSearchFromGpodder get search from gpodder.
func GetSearchFromGpodder(pod model.GPodcast) *model.CommonSearchResultModel {
	p := new(model.CommonSearchResultModel)
	p.URL = pod.URL
	p.Image = pod.LogoURL
	p.Title = pod.Title
	p.Description = pod.Description
	return p
}

// GetSearchFromItunes get search from itunes.
func GetSearchFromItunes(pod model.ItunesSingleResult) *model.CommonSearchResultModel {
	p := new(model.CommonSearchResultModel)
	p.URL = pod.FeedURL
	p.Image = pod.ArtworkURL600
	p.Title = pod.TrackName

	return p
}

// GetSearchFromPodcastIndex get search from podcast index.
func GetSearchFromPodcastIndex(pod *podcastindex.Podcast) *model.CommonSearchResultModel {
	p := new(model.CommonSearchResultModel)
	p.URL = pod.URL
	p.Image = pod.Image
	p.Title = pod.Title
	p.Description = pod.Description

	if pod.Categories != nil {
		values := make([]string, 0, len(pod.Categories))
		for _, val := range pod.Categories {
			values = append(values, val)
		}
		p.Categories = values
	}

	return p
}

// UpdateSettings update settings.
func UpdateSettings(downloadOnAdd bool, initialDownloadCount int, autoDownload bool,
	appendDateToFileName bool, appendEpisodeNumberToFileName bool, darkMode bool, downloadEpisodeImages bool,
	generateNFOFile bool, dontDownloadDeletedFromDisk bool, baseURL string, maxDownloadConcurrency int, userAgent string) error {
	setting := db.GetOrCreateSetting()

	setting.AutoDownload = autoDownload
	setting.DownloadOnAdd = downloadOnAdd
	setting.InitialDownloadCount = initialDownloadCount
	setting.AppendDateToFileName = appendDateToFileName
	setting.AppendEpisodeNumberToFileName = appendEpisodeNumberToFileName
	setting.DarkMode = darkMode
	setting.DownloadEpisodeImages = downloadEpisodeImages
	setting.GenerateNFOFile = generateNFOFile
	setting.DontDownloadDeletedFromDisk = dontDownloadDeletedFromDisk
	setting.BaseURL = baseURL
	setting.MaxDownloadConcurrency = maxDownloadConcurrency
	setting.UserAgent = userAgent

	return db.UpdateSettings(setting)
}

// UnlockMissedJobs unlock missed jobs.
func UnlockMissedJobs() {
	db.UnlockMissedJobs()
}

// AddTag add tag.
func AddTag(label, description string) (db.Tag, error) {
	tag, err := db.GetTagByLabel(label)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		tag := db.Tag{
			Label:       label,
			Description: description,
		}

		err = db.CreateTag(&tag)
		return tag, err
	}

	return *tag, &model.TagAlreadyExistsError{Label: label}
}

// TogglePodcastPause toggle podcast pause.
func TogglePodcastPause(id string, isPaused bool) error {
	var podcast db.Podcast
	err := db.GetPodcastByID(id, &podcast)
	if err != nil {
		return err
	}

	return db.TogglePodcastPauseStatus(id, isPaused)
}
