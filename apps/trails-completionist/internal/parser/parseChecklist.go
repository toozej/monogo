package parser

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/toozej/trails-completionist/internal/types"
)

// Extract trail information from file contents
func extractTrailInfoFromChecklist(file *os.File) ([]types.Trail, error) {
	var trails []types.Trail

	// open file
	scanner := bufio.NewScanner(file)

	// scan through file looking for trail info
	var currentTrail types.Trail
	var currentPark string

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "## "):
			currentPark = parseTrailParkFromChecklist(line)
		case strings.HasPrefix(line, "- "):
			if currentTrail.Name != "" {
				trails = append(trails, currentTrail)
			}
			currentTrail = types.Trail{
				Name: parseTrailNameFromChecklist(line),
				Park: currentPark,
			}
		case strings.HasPrefix(line, "    - "):
			switch {
			case strings.Contains(line, "Connector") || strings.Contains(line, "Trail"):
				currentTrail.Type = parseTrailTypeFromChecklist(line)
			case strings.Contains(line, "miles"):
				currentTrail.Length = parseTrailLengthFromChecklist(line)
			case strings.Contains(line, "Completed"):
				currentTrail.Completed = parseTrailCompletedFromChecklist(line)
				currentTrail.CompletionDate = parseTrailCompletionDateFromChecklist(line)
			case strings.Contains(line, "http"):
				currentTrail.URL = parseTrailURLFromChecklist(line)
			}
		}
	}

	return trails, nil
}

func parseTrailNameFromChecklist(input string) string {
	// Regular expression to parse trail park
	re := regexp.MustCompile(`^-\s*(.*$)$`)

	// FindStringSubmatch returns a slice of strings containing the text of the leftmost match
	match := re.FindStringSubmatch(input)

	var trailName string
	if len(match) == 2 {
		trailName = match[1]
	} else {
		trailName = ""
	}

	return trailName
}

func parseTrailTypeFromChecklist(input string) string {
	// Regular expression to parse trail type and length
	re := regexp.MustCompile(`^\s{4}-\s(Connector|Trail)$`)

	// FindStringSubmatch returns a slice of strings containing the text of the leftmost match
	match := re.FindStringSubmatch(input)

	var trailType string
	if len(match) == 2 {
		trailType = match[1]
	} else {
		trailType = ""
	}

	return trailType
}

func parseTrailLengthFromChecklist(input string) string {
	// Regular expression to parse trail type and length
	re := regexp.MustCompile(`^\s{4}-\s(\d+(?:\.\d+)?)\s+miles$`)

	// FindStringSubmatch returns a slice of strings containing the text of the leftmost match
	match := re.FindStringSubmatch(input)

	var trailLength string
	if len(match) == 2 {
		trailLength = match[1]
	} else {
		trailLength = "NaN"
	}

	return trailLength
}

func parseTrailParkFromChecklist(input string) string {
	// Regular expression to parse trail park
	re := regexp.MustCompile(`^##\s*(.*$)$`)

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

func parseTrailCompletedFromChecklist(input string) bool {
	// Regular expression to parse trail completed
	re := regexp.MustCompile(`^\s{4}-\s{1}(?i)Completed`)

	// FindStringSubmatch returns a slice of strings containing the text of the leftmost match
	match := re.FindStringSubmatch(input)

	var trailCompleted bool
	if len(match) == 1 {
		trailCompleted = true
	} else {
		trailCompleted = false
	}

	return trailCompleted
}

func parseTrailCompletionDateFromChecklist(input string) string {
	// Parse trail completion date using time.Parse()

	// remove any non-date junk from input
	d := strings.TrimPrefix(input, "    - Completed ")

	// Define the valid layouts
	// date format must be MM/DD/YYYY, MM/DD/YY, M/D/YYYY, or M/D/YY
	layouts := []string{"01/02/2006", "01/02/06", "1/2/2006", "1/2/06"}

	defaultDate := "01/01/1970"
	var parsedDate time.Time
	var err error
	var trailCompletionDate string

	// Try parsing the date-like input d using the layouts
	for _, layout := range layouts {
		parsedDate, err = time.Parse(layout, d)
		if err == nil {
			break
		}
	}

	// If no valid date is found, use the default date
	if err != nil {
		parsedDate, _ = time.Parse(layouts[0], defaultDate)
	}

	trailCompletionDate = parsedDate.Format(layouts[0])

	return trailCompletionDate
}

func parseTrailURLFromChecklist(input string) string {
	// Regular expression to parse trail URL
	re := regexp.MustCompile(`^\s{4}-\s{1}(http.*)$`)

	// FindStringSubmatch returns a slice of strings containing the text of the leftmost match
	match := re.FindStringSubmatch(input)

	var trailURL string
	if len(match) == 2 {
		trailURL = match[1]
	} else {
		trailURL = ""
	}

	return trailURL
}

func ParseTrailsFromChecklist(filename string) ([]types.Trail, error) {
	f, err := fetchFile(filename)
	if err != nil {
		return []types.Trail{}, err
	}

	trails, err := extractTrailInfoFromChecklist(f)
	if err != nil {
		return []types.Trail{}, err
	}

	return trails, nil
}
