package trailscompletionist

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/toozej/monogo/apps/trails-completionist/internal/catalog"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/monogo/apps/trails-completionist/internal/config"
	"github.com/toozej/monogo/apps/trails-completionist/internal/generator"
	"github.com/toozej/monogo/apps/trails-completionist/internal/geo"
	"github.com/toozej/monogo/apps/trails-completionist/internal/matcher"
	"github.com/toozej/monogo/apps/trails-completionist/internal/parser"
	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
	"github.com/toozej/monogo/pkg/osm"
	"github.com/toozej/monogo/pkg/tcx2gpx"
)

// RunTrailsCompletionist contains the main application logic extracted from rootCmdRun
func RunTrailsCompletionist(config config.Config, debug bool) error {
	combinedTrails, err := CollectTrails(config)
	if err != nil {
		return err
	}
	if err = generator.GenerateChecklist(config.ChecklistFile, combinedTrails); err != nil {
		return fmt.Errorf("error generating checklist: %w", err)
	}

	trails := combinedTrails
	if debug {
		log.Printf("Generated %d trails", len(trails))
	}

	// Generate HTML table from checklist
	if err = generator.GenerateHTMLOutput(config.HTMLFile, trails); err != nil {
		return fmt.Errorf("error generating HTML output file: %w", err)
	} else if config.Serve {
		return ServeHTMLFile(config.HTMLFile, config.ServeAddress)
	}

	return nil
}

// CollectTrails combines the text list, line inventories, and OSM discoveries.
func CollectTrails(cfg config.Config) ([]types.Trail, error) {
	distance, minimum := cfg.MatchDistance, cfg.MinimumCoverage
	if distance == 0 {
		distance = parser.DefaultDistanceMetres
	}
	if minimum == 0 {
		minimum = parser.DefaultMinimumCoverage
	}
	if math.IsNaN(distance) || math.IsInf(distance, 0) || distance <= 0 || distance > 1000 || math.IsNaN(minimum) || minimum <= 0 || minimum > 1 {
		return nil, fmt.Errorf("matchDistance must be greater than zero and at most 1000. minimumCoverage must be greater than zero and at most one")
	}
	boundary, err := parser.ParseBoundary(cfg.BoundaryFile)
	if err != nil {
		return nil, fmt.Errorf("read boundary: %w", err)
	}
	var input []types.Trail
	if cfg.InputFile != "" {
		raw, err := parser.ParseTrailsFromRawInputFile(cfg.InputFile)
		if err != nil {
			return nil, err
		}
		input = append(input, raw...)
	}
	ids := map[string]bool{}
	for _, path := range cfg.InventoryFiles {
		inventory, err := parser.ParseInventory(path, boundary)
		if err != nil {
			return nil, fmt.Errorf("read inventory: %w", err)
		}
		for _, trail := range inventory {
			if ids[trail.ID] {
				return nil, fmt.Errorf("duplicate inventory ID across files: %s", trail.ID)
			}
			ids[trail.ID] = true
			input = append(input, trail)
		}
	}
	var candidates []types.Trail
	if cfg.OSMRegionFile != "" {
		data, err := osm.LoadTrailData(cfg.OSMRegionFile, false)
		if err != nil {
			return nil, fmt.Errorf("load OSM export: %w", err)
		}
		var diagnostics catalog.Diagnostics
		candidates, diagnostics = catalog.Build(data, boundary)
		var report catalog.Report
		input, report = catalog.Audit(input, candidates, diagnostics, distance, minimum)
		report.OSMFile, report.BoundaryFile = cfg.OSMRegionFile, cfg.BoundaryFile
		report.InputFiles = append([]string(nil), cfg.InventoryFiles...)
		if cfg.InputFile != "" {
			report.InputFiles = append(report.InputFiles, cfg.InputFile)
		}
		fmt.Printf("OSM audit: %v\n", report.Summary)
		if cfg.ReportFile != "" {
			if err := catalog.WriteReport(cfg.ReportFile, report); err != nil {
				return nil, err
			}
		}
	} else if cfg.ReportFile != "" {
		return nil, fmt.Errorf("OSM region data is required for an audit report")
	}
	if cfg.TrackFiles == "" {
		if cfg.InputFile == "" && len(cfg.InventoryFiles) == 0 {
			return candidates, nil
		}
		return input, nil
	}
	if cfg.OSMRegionFile == "" {
		hasGeometry := false
		for _, trail := range input {
			hasGeometry = hasGeometry || (!trail.Incomplete && geo.Length(trail.Geometry) > 0)
		}
		if !hasGeometry {
			return nil, fmt.Errorf("OSM region data is required when track files have no inventory geometry")
		}
	}
	if err := tcx2gpx.ConvertAllTCXToGPX(cfg.TrackFiles); err != nil {
		return nil, err
	}
	targets := append([]types.Trail(nil), input...)
	// Preserve additional OSM discoveries. The source list retains its own identities.
	targets = append(targets, candidates...)
	completed, err := parser.ParseTracksWithTargets(cfg.TrackFiles, true, targets, distance, minimum)
	if err != nil {
		return nil, err
	}
	return matcher.MatchTrailsWithDistance(completed, input, distance)
}

// HTMLHandler serves only the generated page and its two known assets.
func HTMLHandler(htmlFile string) http.Handler {
	htmlFile = filepath.Clean(htmlFile)
	htmlDir := filepath.Dir(htmlFile)
	assets := map[string]string{
		"/":           htmlFile,
		"/index.html": htmlFile,
		"/app.js":     filepath.Join(htmlDir, "app.js"),
		"/styles.css": filepath.Join(htmlDir, "styles.css"),
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		asset, ok := assets[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		info, err := os.Lstat(asset)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, asset)
	})
}

func newHTMLServer(htmlFile, listenAddress string) *http.Server {
	if listenAddress == "" {
		listenAddress = config.DefaultServeAddress
	}
	return &http.Server{
		Addr:         listenAddress,
		Handler:      HTMLHandler(htmlFile),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

// ServeHTMLFile serves the page at the configured address. An empty address uses loopback.
func ServeHTMLFile(htmlFile, listenAddress string) error {
	server := newHTMLServer(htmlFile, listenAddress)
	network := "tcp"
	if host, _, err := net.SplitHostPort(server.Addr); err == nil && net.ParseIP(host).To4() != nil {
		network = "tcp4"
	}
	listener, err := net.Listen(network, server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}
	log.Printf("Serving HTML file on %s", listener.Addr())
	if host, port, err := net.SplitHostPort(listener.Addr().String()); err == nil && net.ParseIP(host).IsUnspecified() {
		log.Printf("Local page: http://localhost:%s/", port)
		log.Printf("LAN page: use this computer's LAN IP address and port %s", port)
	} else {
		log.Printf("Page: http://%s/", listener.Addr())
	}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("error serving generated HTML file: %w", err)
	}

	return nil
}
