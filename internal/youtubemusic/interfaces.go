package youtubemusic

type Artist struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	URI    string   `json:"uri"`
	Genres []string `json:"genres"`
}

type Album struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"album_type"`
}

type Track struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	URI      string   `json:"uri"`
	Artists  []Artist `json:"artists"`
	Duration int      `json:"duration_ms"`
	Album    Album    `json:"album"`
}

type Playlist struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URI        string `json:"uri"`
	TrackCount int    `json:"track_count"`
	EmbedURL   string `json:"embed_url"`
	IsIncoming bool   `json:"is_incoming"`
}
