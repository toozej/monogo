// Package service implements business logic for podcast management and downloads.
package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/TheHippo/podcastindex"
	"github.com/toozej/monogo/apps/podgrab/internal/logger"
	"github.com/toozej/monogo/apps/podgrab/model"
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
	searchURL := fmt.Sprintf("%s/search?term=%s&entity=podcast", ItunesBase, url.QueryEscape(q))

	body, err := makeQuery(searchURL)
	if err != nil {
		logger.Log.Errorw("making iTunes query", "error", err)
		return []*model.CommonSearchResultModel{}
	}
	var response model.ItunesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		logger.Log.Errorw("unmarshaling iTunes response", "error", err)
	}

	toReturn := make([]*model.CommonSearchResultModel, 0, len(response.Results))

	for i := range response.Results {
		toReturn = append(toReturn, GetSearchFromItunes(&response.Results[i]))
	}

	return toReturn
}

// PodcastIndexService represents podcast index service data.
type PodcastIndexService struct {
}

func getPodcastIndexCredentials() (apiKey, apiSecret string) {
	apiKey = os.Getenv("PODCASTINDEX_KEY")
	apiSecret = os.Getenv("PODCASTINDEX_SECRET")
	return apiKey, apiSecret
}

// Query searches for podcasts using the Podcast Index API.
func (service PodcastIndexService) Query(q string) []*model.CommonSearchResultModel {
	key, secret := getPodcastIndexCredentials()
	if key == "" || secret == "" {
		logger.Log.Error("Podcast Index search requires PODCASTINDEX_KEY and PODCASTINDEX_SECRET")
		return []*model.CommonSearchResultModel{}
	}
	c := podcastindex.NewClient(key, secret)
	var toReturn []*model.CommonSearchResultModel
	podcasts, err := c.Search(q)
	if err != nil {
		logger.Log.Errorw("searching Podcast Index", "error", err)
		return toReturn
	}

	for _, obj := range podcasts {
		toReturn = append(toReturn, GetSearchFromPodcastIndex(obj))
	}

	return toReturn
}
