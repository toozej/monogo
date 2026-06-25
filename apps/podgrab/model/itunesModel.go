// Package model defines data structures for external API responses and RSS feeds.
package model

import "time"

// ItunesResponse represents itunes response data.
type ItunesResponse struct {
	Results     []ItunesSingleResult `json:"results"`
	ResultCount int                  `json:"resultCount"`
}

// ItunesSingleResult represents itunes single result data.
type ItunesSingleResult struct {
	ReleaseDate            time.Time `json:"releaseDate"`
	ArtworkURL30           string    `json:"artworkUrl30"`
	CollectionViewURL      string    `json:"collectionViewUrl"`
	TrackExplicitness      string    `json:"trackExplicitness"`
	ArtistName             string    `json:"artistName"`
	CollectionName         string    `json:"collectionName"`
	TrackName              string    `json:"trackName"`
	CollectionCensoredName string    `json:"collectionCensoredName"`
	ArtworkURL60           string    `json:"artworkUrl60"`
	Country                string    `json:"country"`
	FeedURL                string    `json:"feedUrl"`
	ArtistViewURL          string    `json:"artistViewUrl,omitempty"`
	TrackViewURL           string    `json:"trackViewUrl"`
	TrackCensoredName      string    `json:"trackCensoredName"`
	ArtworkURL100          string    `json:"artworkUrl100"`
	CollectionExplicitness string    `json:"collectionExplicitness"`
	WrapperType            string    `json:"wrapperType"`
	ArtworkURL600          string    `json:"artworkUrl600"`
	ContentAdvisoryRating  string    `json:"contentAdvisoryRating,omitempty"`
	PrimaryGenreName       string    `json:"primaryGenreName"`
	Currency               string    `json:"currency"`
	Kind                   string    `json:"kind"`
	GenreIDs               []string  `json:"genreIds"`
	Genres                 []string  `json:"genres"`
	TrackPrice             float64   `json:"trackPrice"`
	TrackCount             int       `json:"trackCount"`
	TrackHdRentalPrice     int       `json:"trackHdRentalPrice"`
	TrackHdPrice           int       `json:"trackHdPrice"`
	CollectionHdPrice      int       `json:"collectionHdPrice"`
	TrackRentalPrice       int       `json:"trackRentalPrice"`
	CollectionPrice        float64   `json:"collectionPrice"`
	TrackID                int       `json:"trackId"`
	ArtistID               int       `json:"artistId,omitempty"`
	CollectionID           int       `json:"collectionId"`
}
