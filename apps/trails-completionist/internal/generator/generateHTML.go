package generator

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

const initialPageSize = 24

type webTrail struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Label          string `json:"label,omitempty"`
	Reference      string `json:"reference,omitempty"`
	Park           string `json:"park"`
	Type           string `json:"type"`
	Length         string `json:"length"`
	URL            string `json:"url,omitempty"`
	MapURL         string `json:"mapURL,omitempty"`
	Completed      bool   `json:"completed"`
	CompletionDate string `json:"date,omitempty"`
	Source         int    `json:"source"`
	Status         string `json:"status,omitempty"`
	Match          string `json:"match,omitempty"`
	Incomplete     bool   `json:"incomplete,omitempty"`
	Key            int    `json:"-"`
}

type webSource struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	URL   string `json:"url,omitempty"`
}

type webData struct {
	Records []webTrail  `json:"records"`
	Sources []webSource `json:"sources"`
}

type pageData struct {
	Initial   []webTrail
	Count     int
	Completed int
	Remaining int
	Parks     int
	Percent   int
	Data      template.JS
}

func organizeTrails(trails []types.Trail) (map[string][]types.Trail, error) {
	byPark := make(map[string][]types.Trail)
	for _, trail := range trails {
		byPark[trail.Park] = append(byPark[trail.Park], trail)
	}
	return byPark, nil
}

func safeWebURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

func sourceLabel(value string) string {
	switch {
	case strings.Contains(value, "Parks_Pathways_and_Trails/"):
		return "Portland Parks"
	case strings.Contains(value, "Basemap_Color/"):
		return "Regional trails"
	case value == "":
		return "Unspecified source"
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return value
}

func makePageData(trails []types.Trail) (pageData, error) {
	// Keep initial order and browser order identical. Source IDs remain unchanged.
	ordered := append([]types.Trail(nil), trails...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Park != ordered[j].Park {
			return ordered[i].Park < ordered[j].Park
		}
		return strings.ToLower(ordered[i].Name) < strings.ToLower(ordered[j].Name)
	})
	data := webData{Records: make([]webTrail, 0, len(trails)), Sources: []webSource{}}
	sources := map[string]int{}
	parks := map[string]bool{}
	page := pageData{Count: len(trails)}
	for i, trail := range ordered {
		source, ok := sources[trail.Source]
		if !ok {
			source = len(data.Sources)
			sources[trail.Source] = source
			data.Sources = append(data.Sources, webSource{Name: sourceLabel(trail.Source), Value: trail.Source, URL: safeWebURL(trail.Source)})
		}
		record := webTrail{ID: trail.ID, Name: trail.Name, Park: trail.Park, Type: trail.Type, Length: trail.Length, URL: safeWebURL(trail.URL), Completed: trail.Completed, CompletionDate: trail.CompletionDate, Source: source, Status: trail.Status, Match: trail.MatchStatus, Incomplete: trail.Incomplete, Key: i}
		if strings.HasPrefix(trail.Name, "Unnamed segment (") && strings.HasSuffix(trail.Name, ")") {
			reference := trail.ID
			if reference == "" {
				reference = strings.TrimSuffix(strings.TrimPrefix(trail.Name, "Unnamed segment ("), ")")
			}
			if slash := strings.LastIndex(reference, "/"); slash >= 0 {
				reference = reference[slash+1:]
			}
			reference = strings.Trim(reference, "{}")
			if len(reference) > 8 {
				reference = reference[:8]
			}
			record.Label = "Unnamed trail segment"
			record.Reference = "Segment " + reference
		}
		for _, path := range trail.Geometry {
			if len(path) > 0 {
				point := path[len(path)/2]
				record.MapURL = fmt.Sprintf("https://www.openstreetmap.org/?mlat=%.6f&mlon=%.6f#map=16/%.6f/%.6f", point.Lat, point.Lon, point.Lat, point.Lon)
				break
			}
		}
		data.Records = append(data.Records, record)
		if trail.Completed {
			page.Completed++
		}
		if trail.Park != "" {
			parks[trail.Park] = true
		}
	}
	page.Parks = len(parks)
	page.Remaining = page.Count - page.Completed
	if page.Count > 0 {
		page.Percent = page.Completed * 100 / page.Count
	}
	page.Initial = data.Records[:min(initialPageSize, len(data.Records))]
	encoded, err := json.Marshal(data)
	if err != nil {
		return page, err
	}
	// json.Marshal escapes HTML delimiters, including closing script tags.
	// noopengrep: go.lang.security.audit.xss.template-html-does-not-escape.unsafe-template-type
	page.Data = template.JS(encoded) // #nosec G203
	return page, nil
}

func copyStaticFiles(tmpl *embed.FS, outputDir string) error {
	for _, file := range []string{"app.js", "styles.css"} {
		data, err := tmpl.ReadFile(file)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(outputDir, file), data, 0o600); err != nil {
			return err
		} // #nosec G304
	}
	return nil
}

func GenerateHTMLOutput(filename string, trails []types.Trail) error {
	page, err := makePageData(trails)
	if err != nil {
		return err
	}
	tmpl, err := template.ParseFS(&Templates, "trails.html.tmpl")
	if err != nil {
		return err
	}
	if err := copyStaticFiles(&Templates, filepath.Dir(filename)); err != nil {
		return err
	}
	file, err := os.Create(filename) // #nosec G304
	if err != nil {
		return err
	}
	renderErr := tmpl.Execute(file, page)
	closeErr := file.Close()
	if renderErr != nil {
		return renderErr
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Println("HTML file generated successfully.")
	return nil
}
