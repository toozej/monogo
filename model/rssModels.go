// Package model defines data structures for external API responses and RSS feeds.
package model

import "encoding/xml"

// RssPodcastData represents the root RSS feed structure for podcast data
type RssPodcastData struct {
	XMLName    xml.Name   `xml:"rss"`
	Text       string     `xml:",chardata"`
	Itunes     string     `xml:"itunes,attr"`
	Atom       string     `xml:"atom,attr"`
	Media      string     `xml:"media,attr"`
	Psc        string     `xml:"psc,attr"`
	Omny       string     `xml:"omny,attr"`
	Content    string     `xml:"content,attr"`
	Googleplay string     `xml:"googleplay,attr"`
	Acast      string     `xml:"acast,attr"`
	Version    string     `xml:"version,attr"`
	Channel    RssChannel `xml:"channel"`
}

// RssChannel represents rss channel data.
type RssChannel struct {
	Image       RssItemImage `xml:"image"`
	Text        string       `xml:",chardata"`
	Language    string       `xml:"language"`
	Link        string       `xml:"link"`
	Title       string       `xml:"title"`
	Description string       `xml:"description"`
	Type        string       `xml:"type"`
	Summary     string       `xml:"summary"`
	Author      string       `xml:"author"`
	Item        []RssItem    `xml:"item"`
}

// RssItem represents rss item data.
type RssItem struct {
	Text        string           `xml:",chardata"`
	Title       string           `xml:"title"`
	Description string           `xml:"description"`
	Encoded     string           `xml:"encoded"`
	Summary     string           `xml:"summary"`
	EpisodeType string           `xml:"episodeType"`
	Author      string           `xml:"author"`
	Image       RssItemImage     `xml:"image"`
	GUID        RssItemGUID      `xml:"guid"`
	ClipID      string           `xml:"clipId"`
	PubDate     string           `xml:"pubDate"`
	Duration    string           `xml:"duration"`
	Enclosure   RssItemEnclosure `xml:"enclosure"`
	Link        string           `xml:"link"`
	Episode     string           `xml:"episode"`
}

// RssItemEnclosure represents rss item enclosure data.
type RssItemEnclosure struct {
	Text   string `xml:",chardata"`
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

// RssItemImage represents rss item image data.
type RssItemImage struct {
	Text string `xml:",chardata"`
	Href string `xml:"href,attr"`
	URL  string `xml:"url"`
}

// RssItemGUID represents rss item guid data.
type RssItemGUID struct {
	Text        string `xml:",chardata"`
	IsPermaLink string `xml:"isPermaLink,attr"`
}
