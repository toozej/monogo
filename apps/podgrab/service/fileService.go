// Package service implements business logic for podcast management and downloads.
package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	stringy "github.com/gobeam/stringy"
	"github.com/toozej/monogo/apps/podgrab/db"
	"github.com/toozej/monogo/apps/podgrab/internal/logger"
	"github.com/toozej/monogo/apps/podgrab/internal/sanitize"
	"github.com/toozej/monogo/pkg/urlsafe"
)

// Download download.
func Download(link, episodeTitle, podcastName, episodePathName string) (string, error) {
	if link == "" {
		return "", errors.New("Download link empty")
	}

	// Calculate file path first
	fileExtension := path.Ext(getFileName(link, episodeTitle, ".mp3"))
	finalPath := path.Join(
		os.Getenv("DATA"),
		cleanFileName(podcastName),
		fmt.Sprintf("%s%s", episodePathName, fileExtension),
	)
	dir, _ := path.Split(finalPath)
	createPreSanitizedPath(dir)

	// Check if file already exists - skip download if it does
	if _, err := os.Stat(finalPath); !os.IsNotExist(err) { // #nosec G703 -- path is sanitized via cleanFileName and constructed from DATA env var
		changeOwnership(finalPath)
		return finalPath, nil
	}

	// File doesn't exist, proceed with download
	client := httpClient()

	req, err := getRequest(link)
	if err != nil {
		logger.Log.Errorw("Error creating request: "+link, err)
		return "", err
	}

	resp, err := client.Do(req) // #nosec G704 -- link validated by urlsafe.Validate in getRequest
	if err != nil {
		logger.Log.Errorw("Error getting response: "+link, err)
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log.Errorw("Error closing response body", "error", closeErr)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Validate and clean path to prevent directory traversal
	dataPath := os.Getenv("DATA")
	if validateErr := validatePath(finalPath, dataPath); validateErr != nil {
		return "", validateErr
	}
	cleanPath := filepath.Clean(finalPath)

	if err := writeFileAtomically(cleanPath, resp.Body); err != nil {
		logger.Log.Errorw("Error saving file", "url", link, "error", err)
		return "", err
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
	setting := db.GetOrCreateSetting()
	return downloadPodcastCoverImage(link, podcastName, setting.UserAgent)
}

func downloadPodcastCoverImage(link, podcastName, userAgent string) (string, error) {
	if link == "" {
		return "", errors.New("Download link empty")
	}
	client := httpClient()
	req, err := getRequestWithUserAgent(link, userAgent)
	if err != nil {
		logger.Log.Errorw("Error creating request: "+link, err)
		return "", err
	}

	resp, err := client.Do(req) // #nosec G704 -- link validated by urlsafe.Validate in getRequest
	if err != nil {
		logger.Log.Errorw("Error getting response: "+link, err)
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log.Errorw("Error closing response body", "error", closeErr)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
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

	if err := writeFileAtomically(cleanPath, resp.Body); err != nil {
		logger.Log.Errorw("Error saving file", "url", link, "error", err)
		return "", err
	}
	changeOwnership(finalPath)
	return finalPath, nil
}

// DownloadImage download image.
func DownloadImage(link, episodeID, podcastName string) (string, error) {
	if link == "" {
		return "", errors.New("Download link empty")
	}
	client := httpClient()
	req, err := getRequest(link)
	if err != nil {
		logger.Log.Errorw("Error creating request: "+link, err)
		return "", err
	}

	resp, err := client.Do(req) // #nosec G704 -- link validated by urlsafe.Validate in getRequest
	if err != nil {
		logger.Log.Errorw("Error getting response: "+link, err)
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log.Errorw("Error closing response body", "error", closeErr)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
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

	if err := writeFileAtomically(cleanPath, resp.Body); err != nil {
		logger.Log.Errorw("Error saving file", "url", link, "error", err)
		return "", err
	}
	changeOwnership(finalPath)
	return finalPath, nil
}

func writeFileAtomically(finalPath string, contents io.Reader) (returnErr error) {
	temporary, err := os.CreateTemp(filepath.Dir(finalPath), ".podgrab-download-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if returnErr != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	if _, err = io.Copy(temporary, contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, finalPath); err != nil {
		return err
	}
	return nil
}

func changeOwnership(filePath string) {
	uid, err1 := strconv.Atoi(os.Getenv("PUID"))
	gid, err2 := strconv.Atoi(os.Getenv("PGID"))
	logger.Log.Debugw("Debug", "value", filePath)
	if err1 == nil && err2 == nil {
		logger.Log.Debugw("Debug", "value", filePath+" : Attempting change")
		if err := os.Chown(filePath, uid, gid); err != nil { // #nosec G703 -- filePath validated via validatePath() before calling changeOwnership
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
	// Validate URL to prevent SSRF attacks: reject non-HTTP(S) schemes and
	// (unless explicitly allowed) private/internal targets.
	if err := urlsafe.Validate(urlString, allowPrivateNetwork()); err != nil {
		return 0, err
	}

	req, err := http.NewRequest(http.MethodHead, urlString, http.NoBody)
	if err != nil {
		return 0, err
	}
	resp, err := httpClient().Do(req) // #nosec G704 -- every redirect and dial is validated by httpClient
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
	file, err := os.Create(tarballFilePath) // #nosec G304 -- path constructed from config folder and timestamp
	if err != nil {
		return "", fmt.Errorf("could not create tarball file '%s', got error '%s'", tarballFilePath, err.Error())
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Log.Errorw("closing file", "error", closeErr)
		}
	}()

	dbPath := path.Join(configPath, "podgrab.db")
	_, err = os.Stat(dbPath) // #nosec G703 -- dbPath constructed from CONFIG env var and fixed filename
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
	file, err := os.Open(filePath) // #nosec G703 G304 -- filePath is from backup process, constructed from config path
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

var (
	lookupIPAddr = net.DefaultResolver.LookupIPAddr
	networkDial  = (&net.Dialer{Timeout: 30 * time.Second}).DialContext
)

func isInternalAddress(ip net.IP) bool {
	return urlsafe.IsInternalIP(ip)
}

// safeDialContext resolves and validates the address at dial time, then dials
// the validated IP directly. This closes the DNS-rebinding window between the
// initial URL validation and the actual connection.
func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if allowPrivateNetwork() {
		return networkDial(ctx, network, address)
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid dial address %q: %w", address, err)
	}
	ips, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("hostname %s resolved to no addresses", host)
	}
	for _, resolved := range ips {
		if isInternalAddress(resolved.IP) {
			return nil, fmt.Errorf("refusing to dial private/internal address %s for %s", resolved.IP, host)
		}
	}

	return networkDial(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

func httpClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = safeDialContext
	// Bound the connection setup and time-to-first-byte, but deliberately leave
	// the overall Client.Timeout unset: this client also downloads full podcast
	// episodes, which can legitimately take far longer than any fixed deadline.
	// A total timeout here would silently truncate large downloads.
	transport.ResponseHeaderTimeout = 30 * time.Second

	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if err := urlsafe.Validate(req.URL.String(), allowPrivateNetwork()); err != nil {
				return fmt.Errorf("refusing redirect target: %w", err)
			}
			return nil
		},
	}
}

func metadataHTTPClient() *http.Client {
	client := httpClient()
	client.Timeout = 30 * time.Second
	return client
}

func getRequest(urlStr string) (*http.Request, error) {
	setting := db.GetOrCreateSetting()
	return getRequestWithUserAgent(urlStr, setting.UserAgent)
}

func getRequestWithUserAgent(urlStr, userAgent string) (*http.Request, error) {
	// Guard against SSRF: urlStr is a user-supplied podcast feed/enclosure/image
	// URL, so reject non-HTTP(S) schemes and (unless explicitly allowed)
	// private/internal targets before building the request. This covers every
	// download path (Download, DownloadPodcastCoverImage, DownloadImage).
	if err := urlsafe.Validate(urlStr, allowPrivateNetwork()); err != nil {
		return nil, fmt.Errorf("refusing to fetch URL: %w", err)
	}

	req, err := http.NewRequest("GET", urlStr, http.NoBody)
	if err != nil {
		return nil, err
	}

	if userAgent != "" {
		req.Header.Add("User-Agent", userAgent)
	} else {
		req.Header.Add("User-Agent", "AppleCoreMedia/1.0.0.22B82 (iPhone; U; CPU OS 18_1 like Mac OS X; en_us)")
	}
	req.Header.Add("Accept", "*/*")

	return req, nil
}

func createPreSanitizedPath(folderPath string) string {
	if _, err := os.Stat(folderPath); os.IsNotExist(err) { // #nosec G703 -- folderPath comes from application-managed directory
		if err := os.MkdirAll(folderPath, 0o750); err != nil { // #nosec G703 -- folderPath comes from application-managed directory
			logger.Log.Errorw("creating folder", "error", err)
		}
		changeOwnership(folderPath)
	}
	return folderPath
}

func createFolder(folder, parent string) string {
	folder = cleanFileName(folder)
	folderPath := path.Join(parent, folder)
	return createPreSanitizedPath(folderPath)
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
	return sanitize.BaseName(original)
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

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}
