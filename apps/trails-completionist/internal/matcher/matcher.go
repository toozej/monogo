package matcher

import (
	"fmt"

	"github.com/toozej/trails-completionist/internal/types"
)

// Match completed / GPX Trails with raw input Trails, creating a combined list of Trails
func MatchTrails(completedTrails []types.Trail, rawTrails []types.Trail) ([]types.Trail, error) {
	var combinedTrails []types.Trail

	// If completedTrails is empty, return rawTrails as is
	if len(completedTrails) == 0 {
		return rawTrails, fmt.Errorf("no completed trails found, therefore no matches to be made. Returning raw trails")
	}

	// Iterate over raw trails
	for _, raw := range rawTrails {
		matched := false
		for _, completed := range completedTrails {
			// Check if the names match
			if completed.Name == raw.Name {
				// Replace rawTrail with completedTrail's details
				combinedTrails = append(combinedTrails, types.Trail{
					Name:           raw.Name, // Keep the raw trail's name
					Park:           raw.Park, // Keep the raw trail's park
					Type:           completed.Type,
					Length:         completed.Length,
					URL:            raw.URL, // Keep the raw trail's URL
					Completed:      completed.Completed,
					CompletionDate: completed.CompletionDate,
				})
				matched = true
				break
			}
		}
		if !matched {
			// If no match, add the raw trail as is
			combinedTrails = append(combinedTrails, raw)
		}
	}

	return combinedTrails, nil
}
