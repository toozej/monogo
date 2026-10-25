package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toozej/monogo/apps/trails-completionist/internal/types"
)

func TestHTMLKeepsAllRecordsWithBoundedInitialRows(t *testing.T) {
	var trails []types.Trail
	for i := range 100 {
		trails = append(trails, types.Trail{ID: fmt.Sprintf("city/%d", i), Name: fmt.Sprintf("Trail %03d", i), Park: "Forest Park", Source: "https://www.portlandmaps.com/arcgis/rest/services/Public/Parks_Pathways_and_Trails/MapServer/1", Completed: i%2 == 0, CompletionDate: "07/05/2026"})
	}
	page, err := makePageData(trails)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Initial) != initialPageSize || page.Count != 100 || page.Completed != 50 || page.Parks != 1 {
		t.Fatalf("invalid initial page: %+v", page)
	}
	var data webData
	if err := json.Unmarshal([]byte(page.Data), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Records) != 100 || len(data.Sources) != 1 || data.Sources[0].Name != "Portland Parks" {
		t.Fatal("pagination dropped records or duplicated source metadata")
	}
	path := filepath.Join(t.TempDir(), "trails.html")
	if err := GenerateHTMLOutput(path, trails); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(html), "<tr data-record-key=") != initialPageSize {
		t.Fatal("the initial HTML must not create a row for every inventory record")
	}
	for _, asset := range []string{"app.js", "styles.css"} {
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), asset)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHTMLEscapesEmbeddedDataAndUnsafeLinks(t *testing.T) {
	name := `</script><script>alert("injection")</script>`
	trails := []types.Trail{{ID: "source/1", Name: name, URL: "javascript:alert(1)", Source: "data:text/html,unsafe", Geometry: [][]types.Point{{{Lat: 45.5, Lon: -122.7}, {Lat: 45.51, Lon: -122.71}}}}}
	page, err := makePageData(trails)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(page.Data), "</script>") {
		t.Fatal("embedded JSON can close its script element")
	}
	var data webData
	if err := json.Unmarshal([]byte(page.Data), &data); err != nil {
		t.Fatal(err)
	}
	if data.Records[0].Name != name || data.Records[0].URL != "" || data.Sources[0].URL != "" || !strings.HasPrefix(data.Records[0].MapURL, "https://www.openstreetmap.org/") {
		t.Fatalf("unsafe links or lost data: %+v", data)
	}
	path := filepath.Join(t.TempDir(), "trails.html")
	if err := GenerateHTMLOutput(path, trails); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(html), name) || strings.Contains(string(html), `href="javascript:`) {
		t.Fatal("trail data became executable HTML")
	}
}

func TestEmptyHTMLCollection(t *testing.T) {
	page, err := makePageData(nil)
	if err != nil || page.Percent != 0 || len(page.Initial) != 0 {
		t.Fatalf("empty collection: %+v, %v", page, err)
	}
	var data webData
	if err := json.Unmarshal([]byte(page.Data), &data); err != nil || data.Records == nil {
		t.Fatalf("empty collection must encode an array: %v", err)
	}
}

func TestUnnamedLabelsPreserveFullSourceNamesAndIDs(t *testing.T) {
	name := "Unnamed segment ({12345678-ABCD-0000-0000-123456789012})"
	id := "inventory/{12345678-ABCD-0000-0000-123456789012}"
	page, err := makePageData([]types.Trail{{ID: id, Name: name, Source: "inventory"}})
	if err != nil {
		t.Fatal(err)
	}
	var data webData
	if err := json.Unmarshal([]byte(page.Data), &data); err != nil {
		t.Fatal(err)
	}
	if data.Records[0].Name != name || data.Records[0].ID != id || data.Records[0].Label != "Unnamed trail segment" || data.Records[0].Reference != "Segment 12345678" {
		t.Fatalf("friendly labels changed source identity: %+v", data.Records[0])
	}
}
