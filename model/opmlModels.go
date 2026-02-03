// Package model defines data structures for external API responses and RSS feeds.
package model

import (
	"encoding/xml"
	"time"
)

// OpmlModel represents opml model data.
type OpmlModel struct {
	XMLName xml.Name `xml:"opml"`
	Text    string   `xml:",chardata"`
	Version string   `xml:"version,attr"`
	Head    OpmlHead `xml:"head"`
	Body    OpmlBody `xml:"body"`
}

// OpmlExportModel represents opml export model data.
type OpmlExportModel struct {
	XMLName xml.Name       `xml:"opml"`
	Text    string         `xml:",chardata"`
	Version string         `xml:"version,attr"`
	Head    OpmlExportHead `xml:"head"`
	Body    OpmlBody       `xml:"body"`
}

// OpmlHead represents opml head data.
type OpmlHead struct {
	Text  string `xml:",chardata"`
	Title string `xml:"title"`
	// DateCreated time.Time `xml:"dateCreated"`
}

// OpmlExportHead represents opml export head data.
type OpmlExportHead struct {
	DateCreated time.Time `xml:"dateCreated"`
	Text        string    `xml:",chardata"`
	Title       string    `xml:"title"`
}

// OpmlBody represents opml body data.
type OpmlBody struct {
	Text    string        `xml:",chardata"`
	Outline []OpmlOutline `xml:"outline"`
}

// OpmlOutline represents opml outline data.
type OpmlOutline struct {
	Title    string        `xml:"title,attr"`
	XMLURL   string        `xml:"xmlUrl,attr"`
	Text     string        `xml:",chardata"`
	AttrText string        `xml:"text,attr"`
	Type     string        `xml:"type,attr"`
	Outline  []OpmlOutline `xml:"outline"`
}
