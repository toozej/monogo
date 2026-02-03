// Package model defines data structures for external API responses and RSS feeds.
package model

import "fmt"

// PodcastAlreadyExistsError represents podcast already exists error data.
type PodcastAlreadyExistsError struct {
	URL string
}

func (e *PodcastAlreadyExistsError) Error() string {
	return "Podcast with this url already exists"
}

// TagAlreadyExistsError represents tag already exists error data.
type TagAlreadyExistsError struct {
	Label string
}

func (e *TagAlreadyExistsError) Error() string {
	return fmt.Sprintf("Tag with this label already exists : %s", e.Label)
}
