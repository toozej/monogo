// Package main implements the Podgrab podcast manager application.
package main

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/jasonlvhit/gocron"
	_ "github.com/joho/godotenv/autoload"
	"github.com/toozej/podgrab/controllers"
	"github.com/toozej/podgrab/db"
	"github.com/toozej/podgrab/internal/logger"
	"github.com/toozej/podgrab/pkg/version"
	"github.com/toozej/podgrab/service"
)

var (
	//go:embed client
	clientEmbed embed.FS
	//go:embed webassets
	webAssetsEmbed embed.FS
	initDB         = db.Init
)

func main() {
	os.Exit(run())
}

func run() int {
	defer logger.Sync()

	var err error
	db.DB, err = initDB()
	if err != nil {
		logger.Log.Errorw("Failed to initialize database", "error", err)
		return 1
	}
	db.Migrate()
	r := gin.Default()

	r.Use(setupSettings())
	r.Use(gin.Recovery())
	r.Use(location.Default())

	funcMap := template.FuncMap{
		"intRange": func(start, end int) []int {
			n := end - start + 1
			result := make([]int, n)
			for i := range n {
				result[i] = start + i
			}
			return result
		},
		"removeStartingSlash": func(raw string) string {
			logger.Log.Debugw("Processing path", "path", raw)
			if string(raw[0]) == "/" {
				return raw
			}
			return "/" + raw
		},
		"isDateNull": func(raw time.Time) bool {
			return raw.Equal((time.Time{}))
		},
		"formatDate": func(raw time.Time) string {
			if raw.Equal((time.Time{})) {
				return ""
			}

			return raw.Format("Jan 2 2006")
		},
		"naturalDate": func(raw time.Time) string {
			return service.NatualTime(time.Now(), raw)
		},
		"latestEpisodeDate": func(podcastItems []db.PodcastItem) string {
			var latest time.Time
			for i := range podcastItems {
				if podcastItems[i].PubDate.After(latest) {
					latest = podcastItems[i].PubDate
				}
			}
			return latest.Format("Jan 2 2006")
		},
		"downloadedEpisodes": func(podcastItems []db.PodcastItem) int {
			count := 0
			for i := range podcastItems {
				if podcastItems[i].DownloadStatus == db.Downloaded {
					count++
				}
			}
			return count
		},
		"downloadingEpisodes": func(podcastItems []db.PodcastItem) int {
			count := 0
			for i := range podcastItems {
				if podcastItems[i].DownloadStatus == db.NotDownloaded {
					count++
				}
			}
			return count
		},
		"formatFileSize": func(inputSize int64) string {
			size := float64(inputSize)
			const divisor float64 = 1024
			if size < divisor {
				return fmt.Sprintf("%.0f bytes", size)
			}
			size /= divisor
			if size < divisor {
				return fmt.Sprintf("%.2f KB", size)
			}
			size /= divisor
			if size < divisor {
				return fmt.Sprintf("%.2f MB", size)
			}
			size /= divisor
			if size < divisor {
				return fmt.Sprintf("%.2f GB", size)
			}
			size /= divisor
			return fmt.Sprintf("%.2f TB", size)
		},
		"formatDuration": func(total int) string {
			if total <= 0 {
				return ""
			}
			mins := total / 60
			secs := total % 60
			hrs := 0
			if mins >= 60 {
				hrs = mins / 60
				mins %= 60
			}
			if hrs > 0 {
				return fmt.Sprintf("%02d:%02d:%02d", hrs, mins, secs)
			}
			return fmt.Sprintf("%02d:%02d", mins, secs)
		},
	}
	tmpl := template.Must(template.New("main").Funcs(funcMap).ParseFS(clientEmbed, "client/*"))

	r.SetHTMLTemplate(tmpl)

	pass := os.Getenv("PASSWORD")
	var router *gin.RouterGroup
	if pass != "" {
		router = r.Group("/", gin.BasicAuth(gin.Accounts{
			"podgrab": pass,
		}))
	} else {
		router = &r.RouterGroup
	}

	dataPath := os.Getenv("DATA")
	backupPath := path.Join(os.Getenv("CONFIG"), "backups")

	webAssets, err := fs.Sub(webAssetsEmbed, "webassets")
	if err != nil {
		logger.Log.Errorw("Failed to load web assets", "error", err)
		return 1
	}

	router.StaticFS("/webassets", http.FS(webAssets))
	router.Static("/assets", dataPath)
	router.Static(backupPath, backupPath)
	router.POST("/podcasts", controllers.AddPodcast)
	router.GET("/podcasts", controllers.GetAllPodcasts)
	router.GET("/podcasts/:id", controllers.GetPodcastByID)
	router.GET("/podcasts/:id/image", controllers.GetPodcastImageByID)
	router.DELETE("/podcasts/:id", controllers.DeletePodcastByID)
	router.GET("/podcasts/:id/items", controllers.GetPodcastItemsByPodcastID)
	router.GET("/podcasts/:id/download", controllers.DownloadAllEpisodesByPodcastID)
	router.GET("/podcasts/:id/refresh", controllers.RefreshEpisodesByPodcastID)
	router.DELETE("/podcasts/:id/items", controllers.DeletePodcastEpisodesByID)
	router.DELETE("/podcasts/:id/podcast", controllers.DeleteOnlyPodcastByID)
	router.GET("/podcasts/:id/pause", controllers.PausePodcastByID)
	router.GET("/podcasts/:id/unpause", controllers.UnpausePodcastByID)
	router.GET("/podcasts/:id/rss", controllers.GetRssForPodcastByID)

	router.GET("/podcastitems", controllers.GetAllPodcastItems)
	router.GET("/podcastitems/:id", controllers.GetPodcastItemByID)
	router.GET("/podcastitems/:id/image", controllers.GetPodcastItemImageByID)
	router.GET("/podcastitems/:id/file", controllers.GetPodcastItemFileByID)
	router.GET("/podcastitems/:id/markUnplayed", controllers.MarkPodcastItemAsUnplayed)
	router.GET("/podcastitems/:id/markPlayed", controllers.MarkPodcastItemAsPlayed)
	router.GET("/podcastitems/:id/bookmark", controllers.BookmarkPodcastItem)
	router.GET("/podcastitems/:id/unbookmark", controllers.UnbookmarkPodcastItem)
	router.PATCH("/podcastitems/:id", controllers.PatchPodcastItemByID)
	router.GET("/podcastitems/:id/download", controllers.DownloadPodcastItem)
	router.GET("/podcastitems/:id/delete", controllers.DeletePodcastItem)

	router.GET("/tags", controllers.GetAllTags)
	router.GET("/tags/:id", controllers.GetTagByID)
	router.GET("/tags/:id/rss", controllers.GetRssForTagByID)
	router.DELETE("/tags/:id", controllers.DeleteTagByID)
	router.POST("/tags", controllers.AddTag)
	router.POST("/podcasts/:id/tags/:tagID", controllers.AddTagToPodcast)
	router.DELETE("/podcasts/:id/tags/:tagID", controllers.RemoveTagFromPodcast)

	router.GET("/refreshAll", controllers.RefreshEpisodes)
	router.GET("/add", controllers.AddPage)
	router.GET("/search", controllers.Search)
	router.GET("/", controllers.HomePage)
	router.GET("/podcasts/:id/view", controllers.PodcastPage)
	router.GET("/episodes", controllers.AllEpisodesPage)
	router.GET("/allTags", controllers.AllTagsPage)
	router.GET("/settings", controllers.SettingsPage)
	router.POST("/settings", controllers.UpdateSetting)
	router.GET("/backups", controllers.BackupsPage)
	router.POST("/opml", controllers.UploadOpml)
	router.GET("/opml", controllers.GetOmpl)
	router.GET("/player", controllers.PlayerPage)
	router.GET("/rss", controllers.GetRss)
	router.GET("/version", func(c *gin.Context) {
		data, err := version.JSON()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json", data)
	})

	r.GET("/ws", func(c *gin.Context) {
		controllers.Wshandler(c.Writer, c.Request)
	})
	go controllers.HandleWebsocketMessages()

	go assetEnv()
	go intiCron()

	if err := r.Run(); err != nil {
		logger.Log.Errorw("Failed to start server", "error", err)
		return 1
	}

	return 0
}
func setupSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		setting := db.GetOrCreateSetting()
		c.Set("setting", setting)
		c.Writer.Header().Set("X-Clacks-Overhead", "GNU Terry Pratchett")

		c.Next()
	}
}

func intiCron() {
	freq, err := strconv.ParseUint(os.Getenv("CHECK_FREQUENCY"), 10, 64)
	if err != nil || freq == 0 {
		freq = 30
		logger.Log.Warnw("Invalid CHECK_FREQUENCY, using default", "error", err, "default", 30)
	}
	service.UnlockMissedJobs()
	if err := gocron.Every(freq).Minutes().Do(service.RefreshEpisodes); err != nil {
		logger.Log.Errorw("Failed to schedule RefreshEpisodes", "error", err)
	}
	if err := gocron.Every(freq).Minutes().Do(service.CheckMissingFiles); err != nil {
		logger.Log.Errorw("Failed to schedule CheckMissingFiles", "error", err)
	}
	if err := gocron.Every(freq * 2).Minutes().Do(service.UnlockMissedJobs); err != nil {
		logger.Log.Errorw("Failed to schedule UnlockMissedJobs", "error", err)
	}
	if err := gocron.Every(freq * 3).Minutes().Do(service.UpdateAllFileSizes); err != nil {
		logger.Log.Errorw("Failed to schedule UpdateAllFileSizes", "error", err)
	}
	if err := gocron.Every(freq).Minutes().Do(service.DownloadMissingImages); err != nil {
		logger.Log.Errorw("Failed to schedule DownloadMissingImages", "error", err)
	}
	if err := gocron.Every(freq).Minutes().Do(service.ClearEpisodeFiles); err != nil {
		logger.Log.Errorw("Failed to schedule ClearEpisodeFiles", "error", err)
	}
	if err := gocron.Every(2).Days().Do(service.CreateBackup); err != nil {
		logger.Log.Errorw("Failed to schedule CreateBackup", "error", err)
	}
	<-gocron.Start()
}

func assetEnv() {
	logger.Log.Infow("Configuration",
		"config_dir", os.Getenv("CONFIG"),
		"assets_dir", os.Getenv("DATA"),
		"check_frequency_mins", os.Getenv("CHECK_FREQUENCY"))
}
