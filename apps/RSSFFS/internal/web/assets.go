package web

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Embed all static assets at build time
//
//go:embed assets/*
var assetsFS embed.FS

// Asset MIME types for proper content serving
var assetMimeTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "application/javascript; charset=utf-8",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".json": "application/json",
	".txt":  "text/plain; charset=utf-8",
	".xyz":  "application/octet-stream", // Ensure consistent behavior across environments
}

// GetAsset retrieves an embedded asset by path
func GetAsset(assetPath string) ([]byte, error) {
	// Clean the path and ensure it's within assets directory
	cleanPath := path.Clean(assetPath)
	if strings.HasPrefix(cleanPath, "../") || strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("invalid asset path: %s", assetPath)
	}

	// Prepend assets/ if not already present
	if !strings.HasPrefix(cleanPath, "assets/") {
		cleanPath = "assets/" + strings.TrimPrefix(cleanPath, "/")
	}

	// Read the embedded file
	data, err := assetsFS.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("asset not found: %s", assetPath)
	}

	return data, nil
}

// GetAssetMimeType determines the MIME type for an asset based on file extension
func GetAssetMimeType(assetPath string) string {
	ext := strings.ToLower(filepath.Ext(assetPath))

	// Check our custom MIME type mappings first
	if mimeType, exists := assetMimeTypes[ext]; exists {
		return mimeType
	}

	// Fall back to Go's built-in MIME type detection
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		return mimeType
	}

	// Default fallback
	return "application/octet-stream"
}

// ServeAsset serves an embedded asset with proper headers and caching
func ServeAsset(w http.ResponseWriter, r *http.Request, assetPath string) {
	// Security check: prevent serving HTML templates as static assets
	// HTML files should be served through the template system to ensure
	// proper escaping and template variable processing
	if strings.ToLower(filepath.Ext(assetPath)) == ".html" {
		http.Error(w, "HTML templates must be served through template system", http.StatusForbidden)
		return
	}

	// Get the asset data
	data, err := GetAsset(assetPath)
	if err != nil {
		http.Error(w, "Asset not found", http.StatusNotFound)
		return
	}

	// Set content type
	mimeType := GetAssetMimeType(assetPath)
	w.Header().Set("Content-Type", mimeType)

	// Set caching headers for static assets
	setCachingHeaders(w, assetPath)

	// Set security headers
	setSecurityHeaders(w)

	// Serve static asset data safely
	// This is safe because:
	// 1. HTML files are explicitly blocked above
	// 2. Data comes from embedded static files, not user input
	// 3. Content-Type is set appropriately for each asset type
	serveStaticAssetData(w, data)
}

// setCachingHeaders sets appropriate caching headers based on asset type
func setCachingHeaders(w http.ResponseWriter, assetPath string) {
	ext := strings.ToLower(filepath.Ext(assetPath))

	switch ext {
	case ".css", ".js", ".svg", ".ico", ".png", ".jpg", ".jpeg", ".gif":
		// Cache static assets for 1 hour in development, longer in production
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, time.Now().Unix()))
	case ".html":
		// Don't cache HTML templates
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	default:
		// Default caching for other assets
		w.Header().Set("Cache-Control", "public, max-age=1800")
	}
}

// setSecurityHeaders sets basic security headers for asset responses
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
}

// serveStaticAssetData safely writes static asset data to the response writer
// This function is explicitly for serving static assets (CSS, JS, images, etc.)
// that do not contain user-generated content and are not HTML templates.
//
// Security considerations:
// - HTML files are blocked at the ServeAsset level to prevent XSS
// - All data comes from embedded static files, not user input
// - Content-Type headers are set appropriately before calling this function
// - This bypasses HTML escaping intentionally for non-HTML static assets
func serveStaticAssetData(w http.ResponseWriter, data []byte) {
	w.WriteHeader(http.StatusOK)
	// Use io.Copy with bytes.Reader for safe static asset serving
	// This is safe for static assets (CSS, JS, images, etc.) as they don't
	// contain user-generated content that needs HTML escaping
	_, _ = io.Copy(w, bytes.NewReader(data))
}

// ListAssets returns a list of all embedded assets (useful for debugging)
func ListAssets() ([]string, error) {
	var assets []string

	err := fs.WalkDir(assetsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			assets = append(assets, path)
		}

		return nil
	})

	return assets, err
}

// AssetExists checks if an asset exists in the embedded filesystem
func AssetExists(assetPath string) bool {
	_, err := GetAsset(assetPath)
	return err == nil
}

// ServeAssetWithFallback serves an asset or falls back to a default asset
func ServeAssetWithFallback(w http.ResponseWriter, r *http.Request, assetPath, fallbackPath string) {
	if AssetExists(assetPath) {
		ServeAsset(w, r, assetPath)
		return
	}

	if fallbackPath != "" && AssetExists(fallbackPath) {
		ServeAsset(w, r, fallbackPath)
		return
	}

	http.Error(w, "Asset not found", http.StatusNotFound)
}
