// Package service implements business logic for podcast management and downloads.
package service

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/akhilrex/podgrab/model"
)

// type GoodReadsService struct {
// }

// BASE is the base URL for GPodder API.
const BASE = "https://gpodder.net"

// Query query.
func Query(q string) []*model.CommonSearchResultModel {
	url := fmt.Sprintf("%s/search.json?q=%s", BASE, url.QueryEscape(q))

	body, _ := makeQuery(url)
	var response []model.GPodcast
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error unmarshaling response: %v\n", err)
	}

	toReturn := make([]*model.CommonSearchResultModel, 0, len(response))

	for _, obj := range response {
		toReturn = append(toReturn, GetSearchFromGpodder(obj))
	}

	return toReturn
}

// ByTag by tag.
func ByTag(tag string, count int) []model.GPodcast {
	url := fmt.Sprintf("%s/api/2/tag/%s/%d.json", BASE, url.QueryEscape(tag), count)

	body, _ := makeQuery(url)
	var response []model.GPodcast
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error unmarshaling response: %v\n", err)
	}
	return response
}

// Top top.
func Top(count int) []model.GPodcast {
	url := fmt.Sprintf("%s/toplist/%d.json", BASE, count)

	body, _ := makeQuery(url)
	var response []model.GPodcast
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error unmarshaling response: %v\n", err)
	}
	return response
}

// Tags tags.
func Tags(count int) []model.GPodcastTag {
	url := fmt.Sprintf("%s/api/2/tags/%d.json", BASE, count)

	body, _ := makeQuery(url)
	var response []model.GPodcastTag
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error unmarshaling GPodder response: %v\n", err)
	}
	return response
}
