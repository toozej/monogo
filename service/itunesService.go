// Package service implements business logic for podcast management and downloads.
package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/TheHippo/podcastindex"
	"github.com/akhilrex/podgrab/model"
)

// SearchService defines the interface for podcast search services.
type SearchService interface {
	Query(q string) []*model.CommonSearchResultModel
}

// ItunesService represents itunes service data.
type ItunesService struct {
}

// ItunesBase is the base URL for iTunes API.
const ItunesBase = "https://itunes.apple.com"

// Query searches for podcasts using the iTunes API.
func (service ItunesService) Query(q string) []*model.CommonSearchResultModel {
	url := fmt.Sprintf("%s/search?term=%s&entity=podcast", ItunesBase, url.QueryEscape(q))

	body, _ := makeQuery(url)
	var response model.ItunesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error unmarshaling iTunes response: %v\n", err)
	}

	toReturn := make([]*model.CommonSearchResultModel, 0, len(response.Results))

	for _, obj := range response.Results {
		toReturn = append(toReturn, GetSearchFromItunes(obj))
	}

	return toReturn
}

// PodcastIndexService represents podcast index service data.
type PodcastIndexService struct {
}

func getPodcastIndexCredentials() (string, string) {
	key := os.Getenv("PODCASTINDEX_KEY")
	secret := os.Getenv("PODCASTINDEX_SECRET")

	// Use demo credentials if environment variables are not set
	// These are public demo credentials from podcastindex.org
	if key == "" {
		key = getDefaultPodcastIndexKey()
	}
	if secret == "" {
		secret = getDefaultPodcastIndexSecret()
	}
	return key, secret
}

func getDefaultPodcastIndexKey() string {
	// Public demo key from podcastindex.org documentation
	return "REDACTED_PODCASTINDEX_KEY"
}

func getDefaultPodcastIndexSecret() string {
	// Public demo secret from podcastindex.org documentation
	chars := []byte{REDACTED_PODCASTINDEX_SECRET_BYTES}
	return string(chars)
}

// Query searches for podcasts using the Podcast Index API.
func (service PodcastIndexService) Query(q string) []*model.CommonSearchResultModel {
	key, secret := getPodcastIndexCredentials()
	c := podcastindex.NewClient(key, secret)
	var toReturn []*model.CommonSearchResultModel
	podcasts, err := c.Search(q)
	if err != nil {
		log.Fatal(err.Error())
		return toReturn
	}

	for _, obj := range podcasts {
		toReturn = append(toReturn, GetSearchFromPodcastIndex(obj))
	}

	return toReturn
}
