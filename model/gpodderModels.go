// Package model defines data structures for external API responses and RSS feeds.
package model

// GPodcast represents g podcast data.
type GPodcast struct {
	URL                 string `json:"url"`
	Title               string `json:"title"`
	Author              string `json:"author"`
	Description         string `json:"description"`
	LogoURL             string `json:"logo_url"`
	ScaledLogoURL       string `json:"scaled_logo_url"`
	Website             string `json:"website"`
	MygpoLink           string `json:"mygpo_link"`
	Subscribers         int    `json:"subscribers"`
	SubscribersLastWeek int    `json:"subscribers_last_week"`
	AlreadySaved        bool   `json:"already_saved"`
}

// GPodcastTag represents g podcast tag data.
type GPodcastTag struct {
	Tag   string `json:"tag"`
	Title string `json:"title"`
	Usage int    `json:"usage"`
}
