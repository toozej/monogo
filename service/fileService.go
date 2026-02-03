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
	"github.com/akhilrex/podgrab/internal/sanitize"
	stringy "github.com/gobeam/stringy"
)

// Download download.
func Download(link string, episodeTitle string, podcastName string, prefix string) (string, error) {
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
		Logger.Errorw("Error creating request: "+link, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		Logger.Errorw("Error getting response: "+link, err)
		return "", err
	}

	// Check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Validate and clean path to prevent directory traversal
	if err := validatePath(finalPath, folder); err != nil {
		return "", err
	}
	cleanPath := filepath.Clean(finalPath)

	file, err := os.Create(cleanPath)
	if err != nil {
		Logger.Errorw("Error creating file"+link, err)
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			Logger.Errorw("Error closing response body", err)
		}
	}()
	_, erra := io.Copy(file, resp.Body)
	// fmt.Println(size)
	defer func() {
		if err := file.Close(); err != nil {
			Logger.Errorw("Error closing file", err)
		}
	}()
	if erra != nil {
		Logger.Errorw("Error saving file"+link, err)
		return "", erra
	}
	changeOwnership(finalPath)
	return finalPath, nil
}

// GetPodcastLocalImagePath get podcast local image path.
func GetPodcastLocalImagePath(link string, podcastName string) string {
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
	return os.WriteFile(finalPath, []byte(toPersist), 0600)
}

// DownloadPodcastCoverImage download podcast cover image.
func DownloadPodcastCoverImage(link string, podcastName string) (string, error) {
	if link == "" {
		return "", errors.New("Download path empty")
	}
	client := httpClient()
	req, err := getRequest(link)
	if err != nil {
		Logger.Errorw("Error creating request: "+link, err)
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		Logger.Errorw("Error getting response: "+link, err)
		return "", err
	}

	fileName := getFileName(link, "folder", ".jpg")
	folder := createDataFolderIfNotExists(podcastName)

	finalPath := path.Join(folder, fileName)

	// Validate and clean path to prevent directory traversal
	if err := validatePath(finalPath, folder); err != nil {
		return "", err
	}
	cleanPath := filepath.Clean(finalPath)

	if _, err := os.Stat(cleanPath); !os.IsNotExist(err) {
		changeOwnership(cleanPath)
		return cleanPath, nil
	}

	file, err := os.Create(cleanPath)
	if err != nil {
		Logger.Errorw("Error creating file"+link, err)
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			Logger.Errorw("Error closing response body", err)
		}
	}()
	_, erra := io.Copy(file, resp.Body)
	// fmt.Println(size)
	defer func() {
		if err := file.Close(); err != nil {
			Logger.Errorw("Error closing file", err)
		}
	}()
	if erra != nil {
		Logger.Errorw("Error saving file"+link, err)
		return "", erra
	}
	changeOwnership(finalPath)
	return finalPath, nil
}

// DownloadImage download image.
func DownloadImage(link string, episodeID string, podcastName string) (string, error) {
	if link == "" {
		return "", errors.New("Download path empty")
	}
	client := httpClient()
	req, err := getRequest(link)
	if err != nil {
		Logger.Errorw("Error creating request: "+link, err)
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		Logger.Errorw("Error getting response: "+link, err)
		return "", err
	}

	fileName := getFileName(link, episodeID, ".jpg")
	folder := createDataFolderIfNotExists(podcastName)
	imageFolder := createFolder("images", folder)
	finalPath := path.Join(imageFolder, fileName)

	// Validate and clean path to prevent directory traversal
	if err := validatePath(finalPath, imageFolder); err != nil {
		return "", err
	}
	cleanPath := filepath.Clean(finalPath)

	if _, err := os.Stat(cleanPath); !os.IsNotExist(err) {
		changeOwnership(cleanPath)
		return cleanPath, nil
	}

	file, err := os.Create(cleanPath)
	if err != nil {
		Logger.Errorw("Error creating file"+link, err)
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			Logger.Errorw("Error closing response body", err)
		}
	}()
	_, erra := io.Copy(file, resp.Body)
	// fmt.Println(size)
	defer func() {
		if err := file.Close(); err != nil {
			Logger.Errorw("Error closing file", err)
		}
	}()
	if erra != nil {
		Logger.Errorw("Error saving file"+link, err)
		return "", erra
	}
	changeOwnership(finalPath)
	return finalPath, nil
}
func changeOwnership(path string) {
	uid, err1 := strconv.Atoi(os.Getenv("PUID"))
	gid, err2 := strconv.Atoi(os.Getenv("PGID"))
	fmt.Println(path)
	if err1 == nil && err2 == nil {
		fmt.Println(path + " : Attempting change")
		if err := os.Chown(path, uid, gid); err != nil {
			fmt.Printf("Error changing ownership: %v\n", err)
		}
	}
}

// DeleteFile delete file.
func DeleteFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(filePath); err != nil {
		return err
	}
	return nil
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
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
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
		fmt.Println(file)
		if err := DeleteFile(file); err != nil {
			fmt.Printf("Error deleting file %s: %v\n", file, err)
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
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
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
		if err := file.Close(); err != nil {
			fmt.Printf("Error closing file: %v\n", err)
		}
	}()

	dbPath := path.Join(configPath, "podgrab.db")
	_, err = os.Stat(dbPath)
	if err != nil {
		return "", fmt.Errorf("could not find db file '%s', got error '%s'", dbPath, err.Error())
	}
	gzipWriter := gzip.NewWriter(file)
	defer func() {
		if err := gzipWriter.Close(); err != nil {
			fmt.Printf("Error closing gzip writer: %v\n", err)
		}
	}()

	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		if err := tarWriter.Close(); err != nil {
			fmt.Printf("Error closing tar writer: %v\n", err)
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
		if err := file.Close(); err != nil {
			fmt.Printf("Error closing file: %v\n", err)
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
			//	r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	return &client
}

func getRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, err
	}

	setting := db.GetOrCreateSetting()
	if len(setting.UserAgent) > 0 {
		req.Header.Add("User-Agent", setting.UserAgent)
	}

	return req, nil
}

func createFolder(folder string, parent string) string {
	folder = cleanFileName(folder)
	// str := stringy.New(folder)
	folderPath := path.Join(parent, folder)
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		if err := os.MkdirAll(folderPath, 0750); err != nil {
			fmt.Printf("Error creating folder: %v\n", err)
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

func getFileName(link string, title string, defaultExtension string) string {
	fileURL, err := url.Parse(link)
	checkError(err)

	parsed := fileURL.Path
	ext := filepath.Ext(parsed)

	if len(ext) == 0 {
		ext = defaultExtension
	}
	// str := stringy.New(title)
	str := stringy.New(cleanFileName(title))
	return str.KebabCase().Get() + ext
}

func cleanFileName(original string) string {
	return sanitize.Name(original)
}

func validatePath(filePath string, baseDir string) error {
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
	parsedURL, err := url.Parse(urlString)
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
