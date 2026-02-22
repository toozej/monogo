// Package service implements business logic for podcast management and downloads.
package service

import (
	"archive/tar"
	"compress/gzip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/akhilrex/podgrab/db"
	"github.com/akhilrex/podgrab/internal/logger"
	"github.com/akhilrex/podgrab/internal/sanitize"
	stringy "github.com/gobeam/stringy"
)

// Download download.
func Download(link, episodeTitle, podcastName, prefix string) (string, error) {
	if link == "" {
		return "", errors.New("Download path empty")
	}

	// Calculate file path first
	fileName := getFileName(link, episodeTitle, ".mp3")
	if prefix != "" {
		fileName = fmt.Sprintf("%s-%s", prefix, fileName)
	}
	folder := createDataFolderIfNotExists(podcastName)
	finalPath := path.Join(folder, fileName)

	// Check if file already exists - skip download if it does
	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		changeOwnership(finalPath)
		return finalPath, nil
	}

	// File doesn't exist, proceed with download
	client := httpClient()

	req, err := getRequest(link)
	if err != nil {
		logger.Log.Errorw("Error creating request: "+link, err)
	}

	resp, err := client.Do(req) //nolint:gosec // G704: URL comes from user-provided podcast RSS feeds
	if err != nil {
		logger.Log.Errorw("Error getting response: "+link, err)
		return "", err
	}

	// Check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Validate and clean path to prevent directory traversal
	if validateErr := validatePath(finalPath, folder); validateErr != nil {
		return "", validateErr
	}
	cleanPath := filepath.Clean(finalPath)

	file, err := os.Create(cleanPath)
	if err != nil {
		logger.Log.Errorw("Error creating file"+link, err)
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log.Errorw("Error closing response body", closeErr)
		}
	}()
	_, erra := io.Copy(file, resp.Body)
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Log.Errorw("Error closing file", closeErr)
		}
	}()
	if erra != nil {
		logger.Log.Errorw("Error saving file"+link, err)
		return "", erra
	}
	changeOwnership(finalPath)
	return finalPath, nil
}

// GetPodcastLocalImagePath get podcast local image path.
func GetPodcastLocalImagePath(link, podcastName string) string {
	fileName := getFileName(link, "folder", ".jpg")
	folder := createDataFolderIfNotExists(podcastName)

	finalPath := path.Join(folder, fileName)
	return finalPath
}

// CreateNfoFile create nfo file.
func CreateNfoFile(podcast *db.Podcast) error {
	fileName := "album.nfo"
	folder := createDataFolderIfNotExists(podcast.Title)

	finalPath := path.Join(folder, fileName)

	type NFO struct {
		XMLName xml.Name `xml:"album"`
		Title   string   `xml:"title"`
		Type    string   `xml:"type"`
		Thumb   string   `xml:"thumb"`
	}

	toSave := NFO{
		Title: podcast.Title,
		Type:  "Broadcast",
		Thumb: podcast.Image,
	}
	out, err := xml.MarshalIndent(toSave, " ", "  ")
	if err != nil {
		return err
	}
	toPersist := xml.Header + string(out)
	return os.WriteFile(finalPath, []byte(toPersist), 0o600)
}

// DownloadPodcastCoverImage download podcast cover image.
func DownloadPodcastCoverImage(link, podcastName string) (string, error) {
	if link == "" {
		return "", errors.New("Download path empty")
	}
	client := httpClient()
	req, err := getRequest(link)
	if err != nil {
		logger.Log.Errorw("Error creating request: "+link, err)
		return "", err
	}

	resp, err := client.Do(req) //nolint:gosec // G704: URL comes from user-provided podcast RSS feeds
	if err != nil {
		logger.Log.Errorw("Error getting response: "+link, err)
		return "", err
	}

	fileName := getFileName(link, "folder", ".jpg")
	folder := createDataFolderIfNotExists(podcastName)

	finalPath := path.Join(folder, fileName)

	// Validate and clean path to prevent directory traversal
	if validateErr := validatePath(finalPath, folder); validateErr != nil {
		return "", validateErr
	}
	cleanPath := filepath.Clean(finalPath)

	if _, statErr := os.Stat(cleanPath); !os.IsNotExist(statErr) {
		changeOwnership(cleanPath)
		return cleanPath, nil
	}

	file, err := os.Create(cleanPath)
	if err != nil {
		logger.Log.Errorw("Error creating file"+link, err)
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log.Errorw("Error closing response body", closeErr)
		}
	}()
	_, erra := io.Copy(file, resp.Body)
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Log.Errorw("Error closing file", closeErr)
		}
	}()
	if erra != nil {
		logger.Log.Errorw("Error saving file"+link, err)
		return "", erra
	}
	changeOwnership(finalPath)
	return finalPath, nil
}

// DownloadImage download image.
func DownloadImage(link, episodeID, podcastName string) (string, error) {
	if link == "" {
		return "", errors.New("Download path empty")
	}
	client := httpClient()
	req, err := getRequest(link)
	if err != nil {
		logger.Log.Errorw("Error creating request: "+link, err)
		return "", err
	}

	resp, err := client.Do(req) //nolint:gosec // G704: URL comes from user-provided podcast RSS feeds
	if err != nil {
		logger.Log.Errorw("Error getting response: "+link, err)
		return "", err
	}

	fileName := getFileName(link, episodeID, ".jpg")
	folder := createDataFolderIfNotExists(podcastName)
	imageFolder := createFolder("images", folder)
	finalPath := path.Join(imageFolder, fileName)

	// Validate and clean path to prevent directory traversal
	if validateErr := validatePath(finalPath, imageFolder); validateErr != nil {
		return "", validateErr
	}
	cleanPath := filepath.Clean(finalPath)

	if _, statErr := os.Stat(cleanPath); !os.IsNotExist(statErr) {
		changeOwnership(cleanPath)
		return cleanPath, nil
	}

	file, err := os.Create(cleanPath)
	if err != nil {
		logger.Log.Errorw("Error creating file"+link, err)
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log.Errorw("Error closing response body", closeErr)
		}
	}()
	_, erra := io.Copy(file, resp.Body)
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Log.Errorw("Error closing file", closeErr)
		}
	}()
	if erra != nil {
		logger.Log.Errorw("Error saving file"+link, err)
		return "", erra
	}
	changeOwnership(finalPath)
	return finalPath, nil
}
func changeOwnership(filePath string) {
	uid, err1 := strconv.Atoi(os.Getenv("PUID"))
	gid, err2 := strconv.Atoi(os.Getenv("PGID"))
	logger.Log.Debugw("Debug", "value", filePath)
	if err1 == nil && err2 == nil {
		logger.Log.Debugw("Debug", "value", filePath+" : Attempting change")
		if err := os.Chown(filePath, uid, gid); err != nil { //nolint:gosec // G703: filePath validated via validatePath() before calling changeOwnership
			logger.Log.Errorw("changing ownership", "error", err)
		}
	}
}

// DeleteFile delete file.
func DeleteFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err
	}
	return os.Remove(filePath)
}

// FileExists file exists.
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// GetAllBackupFiles get all backup files.
func GetAllBackupFiles() ([]string, error) {
	var files []string
	folder := createConfigFolderIfNotExists("backups")
	err := filepath.Walk(folder, func(path string, info os.FileInfo, _ error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	return files, err
}

// GetFileSize get file size.
func GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func deleteOldBackup() {
	files, err := GetAllBackupFiles()
	if err != nil {
		return
	}
	if len(files) <= 5 {
		return
	}

	toDelete := files[5:]
	for _, file := range toDelete {
		logger.Log.Debugw("Debug", "value", file)
		if err := DeleteFile(file); err != nil {
			logger.Log.Errorw("deleting file %s", "error", file, err)
		}
	}
}

// GetFileSizeFromURL get file size from url.
func GetFileSizeFromURL(urlString string) (int64, error) {
	// Validate URL to prevent SSRF attacks
	if err := validateURL(urlString); err != nil {
		return 0, err
	}

	resp, err := http.Head(urlString) //nolint:gosec // G107: URL validated by validateURL function above
	if err != nil {
		return 0, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log.Errorw("closing response body", "error", closeErr)
		}
	}()

	// Is our request ok?

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("did not receive 200")
	}

	size, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return 0, err
	}

	return int64(size), nil
}

// CreateBackup create backup.
func CreateBackup() (string, error) {
	backupFileName := "podgrab_backup_" + time.Now().Format("2006.01.02_150405") + ".tar.gz"
	folder := createConfigFolderIfNotExists("backups")
	configPath := os.Getenv("CONFIG")
	tarballFilePath := path.Join(folder, backupFileName)
	file, err := os.Create(tarballFilePath) //nolint:gosec // G304: path constructed from config folder and timestamp
	if err != nil {
		return "", fmt.Errorf("could not create tarball file '%s', got error '%s'", tarballFilePath, err.Error())
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Log.Errorw("closing file", "error", closeErr)
		}
	}()

	dbPath := path.Join(configPath, "podgrab.db")
	_, err = os.Stat(dbPath) //nolint:gosec // G703: dbPath constructed from CONFIG env var and fixed filename
	if err != nil {
		return "", fmt.Errorf("could not find db file '%s', got error '%s'", dbPath, err.Error())
	}
	gzipWriter := gzip.NewWriter(file)
	defer func() {
		if closeErr := gzipWriter.Close(); closeErr != nil {
			logger.Log.Errorw("closing gzip writer", "error", closeErr)
		}
	}()

	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		if closeErr := tarWriter.Close(); closeErr != nil {
			logger.Log.Errorw("closing tar writer", "error", closeErr)
		}
	}()

	err = addFileToTarWriter(dbPath, tarWriter)
	if err == nil {
		deleteOldBackup()
	}
	return backupFileName, err
}

func addFileToTarWriter(filePath string, tarWriter *tar.Writer) error {
	file, err := os.Open(filePath) //nolint:gosec // G304: filePath is from backup process, constructed from config path
	if err != nil {
		return fmt.Errorf("could not open file '%s', got error '%s'", filePath, err.Error())
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Log.Errorw("closing file", "error", closeErr)
		}
	}()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("could not get stat for file '%s', got error '%s'", filePath, err.Error())
	}

	header := &tar.Header{
		Name:    filePath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return fmt.Errorf("could not write header for file '%s', got error '%s'", filePath, err.Error())
	}

	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return fmt.Errorf("could not copy the file '%s' data to the tarball, got error '%s'", filePath, err.Error())
	}

	return nil
}
func httpClient() *http.Client {
	client := http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return nil
		},
	}

	return &client
}

func getRequest(urlStr string) (*http.Request, error) {
	req, err := http.NewRequest("GET", urlStr, http.NoBody)
	if err != nil {
		return nil, err
	}

	setting := db.GetOrCreateSetting()
	if setting.UserAgent != "" {
		req.Header.Add("User-Agent", setting.UserAgent)
	}

	return req, nil
}

func createFolder(folder, parent string) string {
	folder = cleanFileName(folder)
	folderPath := path.Join(parent, folder)
	if _, err := os.Stat(folderPath); os.IsNotExist(err) { //nolint:gosec // G703: folderPath constructed from sanitized folder name via cleanFileName()
		if err := os.MkdirAll(folderPath, 0o750); err != nil { //nolint:gosec // G703: folderPath constructed from sanitized folder name via cleanFileName()
			logger.Log.Errorw("creating folder", "error", err)
		}
		changeOwnership(folderPath)
	}
	return folderPath
}

func createDataFolderIfNotExists(folder string) string {
	dataPath := os.Getenv("DATA")
	return createFolder(folder, dataPath)
}
func createConfigFolderIfNotExists(folder string) string {
	dataPath := os.Getenv("CONFIG")
	return createFolder(folder, dataPath)
}

func deletePodcastFolder(folder string) error {
	return os.RemoveAll(createDataFolderIfNotExists(folder))
}

func getFileName(link, title, defaultExtension string) string {
	fileURL, err := url.Parse(link)
	checkError(err)

	parsed := fileURL.Path
	ext := filepath.Ext(parsed)

	if ext == "" {
		ext = defaultExtension
	}
	str := stringy.New(cleanFileName(title))
	return str.KebabCase().Get() + ext
}

func cleanFileName(original string) string {
	return sanitize.Name(original)
}

func validatePath(filePath, baseDir string) error {
	cleanPath := filepath.Clean(filePath)
	cleanBase := filepath.Clean(baseDir)

	// Ensure the path is within the base directory
	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	// Check for path traversal attempts
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("path traversal detected: %s", filePath)
	}

	return nil
}

func validateURL(urlString string) error {
	parsedURL, err := url.Parse(urlString) //nolint:all // Named return intentionally shadows import
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTP and HTTPS schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: %s", parsedURL.Scheme)
	}

	return nil
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}
