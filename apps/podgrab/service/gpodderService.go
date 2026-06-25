// Package service implements business logic for podcast management and downloads.
package service

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/toozej/podgrab/internal/logger"
	"github.com/toozej/podgrab/model"
)

// type GoodReadsService struct {
// }

// BASE is the base URL for GPodder API.
const BASE = "https://gpodder.net"

// Query query.
func Query(q string) []*model.CommonSearchResultModel {
	searchURL := fmt.Sprintf("%s/search.json?q=%s", BASE, url.QueryEscape(q))

	body, err := makeQuery(searchURL)
	if err != nil {
		logger.Log.Errorw("making query", "error", err)
		return []*model.CommonSearchResultModel{}
	}
	var response []model.GPodcast
	if err := json.Unmarshal(body, &response); err != nil {
		logger.Log.Errorw("unmarshaling response", "error", err)
	}

	toReturn := make([]*model.CommonSearchResultModel, 0, len(response))

	for i := range response {
		toReturn = append(toReturn, GetSearchFromGpodder(&response[i]))
	}

	return toReturn
}

// ByTag by tag.
func ByTag(tag string, count int) []model.GPodcast {
	tagURL := fmt.Sprintf("%s/api/2/tag/%s/%d.json", BASE, url.QueryEscape(tag), count)

	body, err := makeQuery(tagURL)
	if err != nil {
		logger.Log.Errorw("making query", "error", err)
		return []model.GPodcast{}
	}
	var response []model.GPodcast
	if err := json.Unmarshal(body, &response); err != nil {
		logger.Log.Errorw("unmarshaling response", "error", err)
	}
	return response
}

// Top top.
func Top(count int) []model.GPodcast {
	topURL := fmt.Sprintf("%s/toplist/%d.json", BASE, count)

	body, err := makeQuery(topURL)
	if err != nil {
		logger.Log.Errorw("making query", "error", err)
		return []model.GPodcast{}
	}
	var response []model.GPodcast
	if err := json.Unmarshal(body, &response); err != nil {
		logger.Log.Errorw("unmarshaling response", "error", err)
	}
	return response
}

// Tags tags.
func Tags(count int) []model.GPodcastTag {
	tagsURL := fmt.Sprintf("%s/api/2/tags/%d.json", BASE, count)

	body, err := makeQuery(tagsURL)
	if err != nil {
		logger.Log.Errorw("making query", "error", err)
		return []model.GPodcastTag{}
	}
	var response []model.GPodcastTag
	if err := json.Unmarshal(body, &response); err != nil {
		logger.Log.Errorw("unmarshaling GPodder response", "error", err)
	}
	return response
}
