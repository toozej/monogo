package trailscompletionist

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/trails-completionist/internal/generator"
	"github.com/toozej/trails-completionist/internal/matcher"
	"github.com/toozej/trails-completionist/internal/parser"
	"github.com/toozej/trails-completionist/internal/types"
	"github.com/toozej/trails-completionist/pkg/config"
	"github.com/toozej/trails-completionist/pkg/osm"
	"github.com/toozej/trails-completionist/pkg/tcx2gpx"
)

// RunTrailsCompletionist contains the main application logic extracted from rootCmdRun
func RunTrailsCompletionist(config config.Config, debug bool) error {
	if debug {
		fmt.Printf("RunTrailsCompletionist: config struct contains: %v\n", config)
	}

	var osmData *osm.OSMData
	var err error

	if config.OSMRegionFile != "" {
		if debug {
			fmt.Printf("Parsing OSM region file: %s\n", config.OSMRegionFile)
		}
		// Parse OSM region file
		osmData, err = osm.LoadOSMData(config.OSMRegionFile, false)
		if err != nil {
			return fmt.Errorf("error loading OSM region file: %w", err)
		}
		if debug {
			fmt.Printf("Loaded %d nodes and %d ways\n", len(osmData.Nodes), len(osmData.Ways))
		}

	}

	// Process track files if provided
	var foundGPXTrails []types.Trail
	if config.TrackFiles != "" {
		if debug {
			fmt.Printf("Parsing track files: %s\n", config.TrackFiles)
		}

		// Convert TCX-formatted tracks to GPX
		if err := tcx2gpx.ConvertAllTCXToGPX(config.TrackFiles); err != nil {
			return fmt.Errorf("error converting TCX tracks to GPX: %w", err)
		}
		if debug {
			fmt.Printf("Converted TCX tracks to GPX: %s\n", config.TrackFiles)
		}

		// Parse trails out of found GPX files
		foundGPXTrails, err = parser.ParseTrailsFromTrackFiles(config.TrackFiles, true, osmData)
		if err != nil {
			return fmt.Errorf("error parsing trails from track files: %w", err)
		}
		if debug {
			fmt.Printf("Parsed trails from track files:\n %v\n", foundGPXTrails)
		}
	}

	// Process input file if provided
	var rawTrails []types.Trail
	if config.InputFile != "" {
		if debug {
			fmt.Printf("Parsing filename: %s\n", config.InputFile)
		}

		rawTrails, err = parser.ParseTrailsFromRawInputFile(config.InputFile)
		if err != nil {
			return fmt.Errorf("error parsing trails from raw input file: %w", err)
		}
		if debug {
			fmt.Printf("Parsed trails from raw input:\n %v\n", rawTrails)
		}

		// Generate checklist from combined trails
		if debug {
			fmt.Printf("Generating checklist: %s\n", config.ChecklistFile)
		}
	}

	// Merge together rawTrails and foundGPXTrails
	// checking for duplicates, and preferring foundGPXTrails over rawTrails
	// (a.k.a. completed trails over not completed)
	combinedTrails, err := matcher.MatchTrails(foundGPXTrails, rawTrails)
	if err != nil {
		fmt.Println(fmt.Errorf("error matching trails: %w", err))
	}
	if debug {
		fmt.Printf("Combined and de-duplicated list of trails:\n %v\n", combinedTrails)
	}

	if err = generator.GenerateChecklist(config.ChecklistFile, combinedTrails); err != nil {
		return fmt.Errorf("error generating checklist: %w", err)
	}

	// Parse trails from checklist
	trails, err := parser.ParseTrailsFromChecklist(config.ChecklistFile)
	if err != nil {
		return fmt.Errorf("error parsing trails from checklist: %w", err)
	}

	if debug {
		log.Println(trails)
	}

	// Generate HTML table from checklist
	if err = generator.GenerateHTMLOutput(config.HTMLFile, trails); err != nil {
		return fmt.Errorf("error generating HTML output file: %w", err)
	} else if config.Serve {
		return ServeHTMLFile(config.HTMLFile)
	}

	return nil
}

// serveHTMLFile serves the generated HTML file on port 3000
func ServeHTMLFile(htmlFile string) error {
	htmlDir := filepath.Dir(htmlFile)
	http.Handle("/", http.FileServer(http.Dir(htmlDir)))
	server := &http.Server{
		Addr:         ":3000",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Serving HTML file at http://localhost:3000/")
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("error serving generated HTML file: %w", err)
	}

	return nil
}
