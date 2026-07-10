package tcx2gpx

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TCX structures
type TrainingCenterDatabase struct {
	XMLName    xml.Name   `xml:"TrainingCenterDatabase"`
	Activities Activities `xml:"Activities"`
}

type Activities struct {
	Activity []Activity `xml:"Activity"`
}

type Activity struct {
	Sport   string   `xml:"Sport,attr"`
	Id      string   `xml:"Id"`
	Lap     []Lap    `xml:"Lap"`
	Creator *Creator `xml:"Creator,omitempty"`
}

type Lap struct {
	StartTime           string        `xml:"StartTime,attr"`
	TotalTimeSeconds    float64       `xml:"TotalTimeSeconds"`
	DistanceMeters      float64       `xml:"DistanceMeters"`
	Calories            int           `xml:"Calories"`
	AverageHeartRateBpm *HeartRateBpm `xml:"AverageHeartRateBpm,omitempty"`
	MaximumHeartRateBpm *HeartRateBpm `xml:"MaximumHeartRateBpm,omitempty"`
	Track               Track         `xml:"Track"`
}

type HeartRateBpm struct {
	Value int `xml:"Value"`
}

type Track struct {
	Trackpoint []Trackpoint `xml:"Trackpoint"`
}

type Trackpoint struct {
	Time           string        `xml:"Time"`
	Position       *Position     `xml:"Position,omitempty"`
	AltitudeMeters *float64      `xml:"AltitudeMeters,omitempty"`
	HeartRateBpm   *HeartRateBpm `xml:"HeartRateBpm,omitempty"`
}

type Position struct {
	LatitudeDegrees  float64 `xml:"LatitudeDegrees"`
	LongitudeDegrees float64 `xml:"LongitudeDegrees"`
}

type Creator struct {
	Name string `xml:"Name"`
}

// GPX structures
type GPX struct {
	XMLName xml.Name   `xml:"gpx"`
	Version string     `xml:"version,attr"`
	Creator string     `xml:"creator,attr"`
	Xmlns   string     `xml:"xmlns,attr"`
	Time    string     `xml:"metadata>time"`
	Tracks  []GPXTrack `xml:"trk"`
}

type GPXTrack struct {
	Name     string         `xml:"name"`
	Segments []TrackSegment `xml:"trkseg"`
}

type TrackSegment struct {
	Points []TrackPoint `xml:"trkpt"`
}

type TrackPoint struct {
	Lat       float64  `xml:"lat,attr"`
	Lon       float64  `xml:"lon,attr"`
	Ele       *float64 `xml:"ele,omitempty"`
	Time      string   `xml:"time"`
	HeartRate *int     `xml:"extensions>gpxtpx:TrackPointExtension>gpxtpx:hr,omitempty"`
}

// func ConvertAllTCXToGPX(inputDir string) error {
// 	// Walk through the directory recursively and convert TCX files to GPX
//
// 	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return err
// 		}
//
// 		// Skip directories
// 		if info.IsDir() {
// 			return nil
// 		}
//
// 		// Check if file is a TCX file
// 		if strings.ToLower(filepath.Ext(path)) == ".tcx" {
// 			fmt.Printf("Converting: %s\n", path)
//
// 			err := convertTCXToGPX(path)
// 			if err != nil {
// 				fmt.Printf("Error converting %s: %v\n", path, err)
// 				return nil // Continue with other files
// 			}
//
// 			// Remove original TCX file
// 			err = os.Remove(path)
// 			if err != nil {
// 				fmt.Printf("Error removing original file %s: %v\n", path, err)
// 			} else {
// 				fmt.Printf("Successfully removed original file: %s\n", path)
// 			}
// 		}
//
// 		return nil
// 	})
//
// 	if err != nil {
// 		fmt.Printf("Error walking directory: %v\n", err)
// 		return err
// 	}
// 	fmt.Println("All TCX files converted to GPX successfully.")
// 	return nil
// }

// ConvertAllTCXToGPX walks inputDir safely and converts all .tcx files to .gpx
func ConvertAllTCXToGPX(inputDir string) error {
	root, err := os.OpenRoot(inputDir)
	if err != nil {
		return fmt.Errorf("open root: %w", err)
	}
	defer func() { _ = root.Close() }()

	var conversionErrors []error
	err = fs.WalkDir(root.FS(), ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !strings.EqualFold(filepath.Ext(rel), ".tcx") {
			return nil
		}

		fmt.Printf("Converting: %s\n", rel)
		src, err := root.Open(rel)
		if err != nil {
			conversionErrors = append(conversionErrors, fmt.Errorf("open %s: %w", rel, err))
			return nil
		}
		gpxData, convertErr := convertTCX(src)
		closeErr := src.Close()
		if convertErr != nil {
			conversionErrors = append(conversionErrors, fmt.Errorf("convert %s: %w", rel, convertErr))
			return nil
		}
		if closeErr != nil {
			conversionErrors = append(conversionErrors, fmt.Errorf("close %s: %w", rel, closeErr))
			return nil
		}

		gpxRel := strings.TrimSuffix(rel, filepath.Ext(rel)) + ".gpx"
		tempRel, err := temporaryName(gpxRel)
		if err != nil {
			conversionErrors = append(conversionErrors, fmt.Errorf("temporary name for %s: %w", rel, err))
			return nil
		}
		dst, err := root.OpenFile(tempRel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			conversionErrors = append(conversionErrors, fmt.Errorf("create temporary GPX for %s: %w", rel, err))
			return nil
		}
		if _, err := dst.Write(gpxData); err != nil {
			_ = dst.Close()
			_ = root.Remove(tempRel)
			conversionErrors = append(conversionErrors, fmt.Errorf("write %s: %w", gpxRel, err))
			return nil
		}
		if err := dst.Sync(); err != nil {
			_ = dst.Close()
			_ = root.Remove(tempRel)
			conversionErrors = append(conversionErrors, fmt.Errorf("sync %s: %w", gpxRel, err))
			return nil
		}
		if err := dst.Close(); err != nil {
			_ = root.Remove(tempRel)
			conversionErrors = append(conversionErrors, fmt.Errorf("close %s: %w", gpxRel, err))
			return nil
		}
		if err := root.Rename(tempRel, gpxRel); err != nil {
			_ = root.Remove(tempRel)
			conversionErrors = append(conversionErrors, fmt.Errorf("publish %s: %w", gpxRel, err))
			return nil
		}
		if err := root.Remove(rel); err != nil {
			conversionErrors = append(conversionErrors, fmt.Errorf("remove converted source %s: %w", rel, err))
		} else {
			fmt.Printf("Removed original file: %s\n", rel)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walk directory: %w", err)
	}
	if len(conversionErrors) > 0 {
		return errors.Join(conversionErrors...)
	}

	fmt.Println("All TCX files converted to GPX successfully.")
	return nil
}

func temporaryName(gpxRel string) (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return gpxRel + ".tmp-" + hex.EncodeToString(random), nil
}

func convertTCX(reader io.Reader) ([]byte, error) {
	var tcx TrainingCenterDatabase
	if err := xml.NewDecoder(reader).Decode(&tcx); err != nil {
		return nil, fmt.Errorf("failed to parse TCX data: %w", err)
	}
	gpxOutput, err := xml.MarshalIndent(convertToGPX(&tcx), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to generate GPX data: %w", err)
	}
	return append([]byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"), gpxOutput...), nil
}

func convertTCXToGPX(tcxFilePath string) error {
	// Read TCX file
	tcxFile, err := os.Open(tcxFilePath) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to open TCX file: %v", err)
	}
	defer func() { _ = tcxFile.Close() }()

	gpxData, err := convertTCX(tcxFile)
	if err != nil {
		return err
	}
	gpxFilePath := strings.TrimSuffix(tcxFilePath, filepath.Ext(tcxFilePath)) + ".gpx"
	gpxFile, err := os.CreateTemp(filepath.Dir(gpxFilePath), ".gpx-*") // #nosec G304 -- destination directory derives from the explicit input path
	if err != nil {
		return fmt.Errorf("failed to create temporary GPX file: %w", err)
	}
	tempName := gpxFile.Name()
	defer os.Remove(tempName)
	if err := gpxFile.Chmod(0o600); err != nil {
		_ = gpxFile.Close()
		return err
	}
	if _, err := gpxFile.Write(gpxData); err != nil {
		_ = gpxFile.Close()
		return fmt.Errorf("write GPX file: %w", err)
	}
	if err := gpxFile.Sync(); err != nil {
		_ = gpxFile.Close()
		return fmt.Errorf("sync GPX file: %w", err)
	}
	if err := gpxFile.Close(); err != nil {
		return fmt.Errorf("close GPX file: %w", err)
	}
	if err := os.Rename(tempName, gpxFilePath); err != nil {
		return fmt.Errorf("publish GPX file: %w", err)
	}

	fmt.Printf("Successfully created: %s\n", gpxFilePath)
	return nil
}

func convertToGPX(tcx *TrainingCenterDatabase) *GPX {
	gpx := &GPX{
		Version: "1.1",
		Creator: "TCX to GPX Converter",
		Xmlns:   "http://www.topografix.com/GPX/1/1",
		Time:    time.Now().Format(time.RFC3339),
		Tracks:  make([]GPXTrack, 0),
	}

	for _, activity := range tcx.Activities.Activity {
		gpxTrack := GPXTrack{
			Name:     "Activity " + activity.Id,
			Segments: make([]TrackSegment, 0),
		}

		for _, lap := range activity.Lap {
			segment := TrackSegment{
				Points: make([]TrackPoint, 0),
			}

			for _, tp := range lap.Track.Trackpoint {
				// Skip points without position data
				if tp.Position == nil {
					continue
				}

				gpxPoint := TrackPoint{
					Lat:  tp.Position.LatitudeDegrees,
					Lon:  tp.Position.LongitudeDegrees,
					Ele:  tp.AltitudeMeters,
					Time: tp.Time,
				}

				// Add heart rate if available
				if tp.HeartRateBpm != nil {
					hr := tp.HeartRateBpm.Value
					gpxPoint.HeartRate = &hr
				}

				segment.Points = append(segment.Points, gpxPoint)
			}

			// Only add non-empty segments
			if len(segment.Points) > 0 {
				gpxTrack.Segments = append(gpxTrack.Segments, segment)
			}
		}

		// Add track to GPX
		gpx.Tracks = append(gpx.Tracks, gpxTrack)
	}

	return gpx
}
