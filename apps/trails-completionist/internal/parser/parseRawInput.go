package parser

import (
	"bufio"
	"bytes"
	"os"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/toozej/trails-completionist/internal/types"
)

// Fetch the file contents
func fetchFile(filename string) (*os.File, error) {
	f, err := os.Open(filename) // #nosec G304
	if err != nil {
		log.Fatal(err)
	}
	if log.GetLevel() == log.DebugLevel {
		fileContents, _ := os.ReadFile(filename) // #nosec G304
		bytesReader := bytes.NewReader(fileContents)
		bufReader := bufio.NewReader(bytesReader)
		lineOne, _, _ := bufReader.ReadLine()
		log.Printf("first line of file contents:\n %s\n", string(lineOne))
	}
	return f, err
}

// Extract trail information from file contents
func extractTrailInfo(file *os.File) ([]types.Trail, error) {
	var trails []types.Trail
	var currentTrail types.Trail

	// Read file in batches of 3 lines and parse trail information
	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		// Check if the line is not empty
		if line != "" {
			lines = append(lines, line)

			// Check if we have collected 3 lines (one trail object from raw input file)
			if len(lines) == 3 {
				// Parse the trail information
				currentTrail = types.Trail{
					Name:           parseTrailName(lines[0]),
					Park:           parseTrailPark(lines[2]),
					Type:           parseTrailType(lines[1]),
					Length:         parseTrailLength(lines[1]),
					URL:            parseTrailURL(""),
					Completed:      false,
					CompletionDate: "",
				}
				// Check if the current trail is already in the list
				exists := false
				for _, trail := range trails {
					if trail.Name == currentTrail.Name && trail.Park == currentTrail.Park {
						exists = true
						break
					}
				}
				if !exists {
					trails = append(trails, currentTrail)
				}

				// Reset the lines slice for the next batch
				lines = nil
			}
		}
	}

	return trails, nil
}

func parseTrailName(input string) string {
	return strings.TrimSpace(input)
}

func parseTrailType(input string) string {
	// Regular expression to parse trail type and length
	re := regexp.MustCompile(`^(\S+)\s+(\d+(\.\d+)?)\s+miles$`)

	// FindStringSubmatch returns a slice of strings containing the text of the leftmost match
	match := re.FindStringSubmatch(input)

	var trailType string
	if len(match) == 4 {
		trailType = match[1]
	} else {
		trailType = ""
	}

	return trailType
}

func parseTrailLength(input string) string {
	// Regular expression to parse trail type and length
	re := regexp.MustCompile(`^(\S+)\s+(\d+(\.\d+)?)\s+miles$`)

	// FindStringSubmatch returns a slice of strings containing the text of the leftmost match
	match := re.FindStringSubmatch(input)

	var trailLength string
	if len(match) == 4 {
		trailLength = match[2]
	} else {
		trailLength = "0.0 miles"
	}

	return trailLength
}

func parseTrailPark(input string) string {
	// Regular expression to parse trail park
	re := regexp.MustCompile(`>\s*([^>]+)$`)

	// FindStringSubmatch returns a slice of strings containing the text of the leftmost match
	match := re.FindStringSubmatch(input)

	var trailPark string
	if len(match) == 2 {
		trailPark = match[1]
	} else {
		trailPark = ""
	}

	return trailPark
}

// TODO: implement parseTrailURL function once I have a better input dataset which includes URL links to trails
func parseTrailURL(input string) string {
	return input
}

func ParseTrailsFromRawInputFile(filename string) ([]types.Trail, error) {
	f, err := fetchFile(filename)
	if err != nil {
		return []types.Trail{}, err
	}

	trails, err := extractTrailInfo(f)
	if err != nil {
		return []types.Trail{}, err
	}

	return trails, nil
}
