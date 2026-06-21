package output

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/issue"
)

func readCSVRecords(t *testing.T, buf *bytes.Buffer) [][]string {
	t.Helper()
	reader := csv.NewReader(buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV output: %v\noutput:\n%s", err, buf.String())
	}
	return records
}

func TestCSVHeaders_NilConfig(t *testing.T) {
	got := csvHeaders(nil)
	want := []string{"Summary", "Category", "Workflow", "ActionRef", "OwnerRepo", "CurrentVersion", "LatestVersion", "Runtime", "RuntimeVersion", "EOLDate", "Description"}
	if len(got) != len(want) {
		t.Errorf("csvHeaders(nil) returned %d headers, want %d", len(got), len(want))
	}
	for i, h := range got {
		if h != want[i] {
			t.Errorf("csvHeaders(nil)[%d] = %q, want %q", i, h, want[i])
		}
	}
}

func TestCSVHeaders_WithExtraColumns(t *testing.T) {
	cfg := &CSVConfig{
		ExtraColumns: map[string]string{"Project": "PROJ", "Assignee": "user", "IssueType": "Task", "Priority": "High", "Labels": "archived,actions"},
	}
	got := csvHeaders(cfg)
	want := []string{
		"Summary", "Category", "Workflow", "ActionRef", "OwnerRepo",
		"CurrentVersion", "LatestVersion", "Runtime", "RuntimeVersion", "EOLDate", "Description",
		"Assignee", "IssueType", "Labels", "Priority", "Project",
	}
	if len(got) != len(want) {
		t.Errorf("csvHeaders with extra columns returned %d headers, want %d\ngot: %v", len(got), len(want), got)
	}
	for i, h := range got {
		if h != want[i] {
			t.Errorf("csvHeaders with extra columns[%d] = %q, want %q", i, h, want[i])
		}
	}
}

func TestCSVHeaders_EmptyExtraColumns(t *testing.T) {
	cfg := &CSVConfig{
		ExtraColumns: map[string]string{},
	}
	got := csvHeaders(cfg)
	if len(got) != 11 {
		t.Errorf("csvHeaders with empty ExtraColumns returned %d headers, want 11\ngot: %v", len(got), got)
	}
	base := []string{"Summary", "Category", "Workflow", "ActionRef", "OwnerRepo", "CurrentVersion", "LatestVersion", "Runtime", "RuntimeVersion", "EOLDate", "Description"}
	for i, h := range got {
		if h != base[i] {
			t.Errorf("csvHeaders with empty ExtraColumns[%d] = %q, want %q", i, h, base[i])
		}
	}
}

func TestCSVHeaders_ExtraColumns(t *testing.T) {
	cfg := &CSVConfig{
		ExtraColumns: map[string]string{"Custom2": "val2", "Custom1": "val1"},
	}
	got := csvHeaders(cfg)
	want := []string{
		"Summary", "Category", "Workflow", "ActionRef", "OwnerRepo",
		"CurrentVersion", "LatestVersion", "Runtime", "RuntimeVersion", "EOLDate", "Description",
		"Custom1", "Custom2",
	}
	if len(got) != len(want) {
		t.Errorf("csvHeaders with ExtraColumns returned %d headers, want %d\ngot: %v", len(got), len(want), got)
	}
	for i, h := range got {
		if h != want[i] {
			t.Errorf("csvHeaders with ExtraColumns[%d] = %q, want %q", i, h, want[i])
		}
	}
}

func TestCSVHeaders_AllColumnsCombined(t *testing.T) {
	cfg := &CSVConfig{
		ExtraColumns: map[string]string{"AColumn": "aval", "Labels": "archived", "Project": "PROJ", "ZColumn": "zval"},
	}
	got := csvHeaders(cfg)
	want := []string{
		"Summary", "Category", "Workflow", "ActionRef", "OwnerRepo",
		"CurrentVersion", "LatestVersion", "Runtime", "RuntimeVersion", "EOLDate", "Description",
		"AColumn", "Labels", "Project", "ZColumn",
	}
	if len(got) != len(want) {
		t.Errorf("csvHeaders all columns returned %d headers, want %d\ngot: %v", len(got), len(want), got)
	}
	for i, h := range got {
		if h != want[i] {
			t.Errorf("csvHeaders all columns[%d] = %q, want %q", i, h, want[i])
		}
	}
}

func TestCSVRowValues_NilConfig(t *testing.T) {
	row := csvRow{
		Summary:        "test summary",
		Category:       "archived",
		Workflow:       "ci.yml",
		ActionRef:      "owner/repo@v1",
		OwnerRepo:      "owner/repo",
		CurrentVersion: "v1",
		LatestVersion:  "v2",
		Runtime:        "node",
		RuntimeVersion: "14",
		EOLDate:        "2023-04-30",
		Description:    "test desc",
	}
	got := csvRowValues(row, nil)
	if len(got) != 11 {
		t.Errorf("csvRowValues with nil config returned %d values, want 11", len(got))
	}
	if got[0] != "test summary" {
		t.Errorf("csvRowValues[0] = %q, want %q", got[0], "test summary")
	}
	if got[1] != "archived" {
		t.Errorf("csvRowValues[1] = %q, want %q", got[1], "archived")
	}
}

func TestCSVRowValues_WithConfig(t *testing.T) {
	row := csvRow{
		Summary:   "test summary",
		Category:  "archived",
		Workflow:  "ci.yml",
		ActionRef: "owner/repo@v1",
	}
	cfg := &CSVConfig{
		ExtraColumns: map[string]string{"Env": "prod", "Project": "PROJ"},
	}
	got := csvRowValues(row, cfg)
	want := []string{"test summary", "archived", "ci.yml", "owner/repo@v1", "", "", "", "", "", "", "", "prod", "PROJ"}
	if len(got) != len(want) {
		t.Errorf("csvRowValues with config returned %d values, want %d\ngot: %v", len(got), len(want), got)
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("csvRowValues with config[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestFprintCSV_ArchivedOnly(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "archived/repo", Workflow: "ci.yml", Uses: "archived/repo@v1"},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows (header + 1 data), got %d", len(records))
	}
	if records[0][0] != "Summary" {
		t.Errorf("header[0] = %q, want %q", records[0][0], "Summary")
	}
	if !strings.Contains(records[1][0], "Archived action:") {
		t.Errorf("data row Summary = %q, want to contain 'Archived action:'", records[1][0])
	}
	if records[1][1] != "archived" {
		t.Errorf("data row Category = %q, want %q", records[1][1], "archived")
	}
	if records[1][2] != "ci.yml" {
		t.Errorf("data row Workflow = %q, want %q", records[1][2], "ci.yml")
	}
	if records[1][3] != "archived/repo@v1" {
		t.Errorf("data row ActionRef = %q, want %q", records[1][3], "archived/repo@v1")
	}
	if records[1][4] != "archived/repo" {
		t.Errorf("data row OwnerRepo = %q, want %q", records[1][4], "archived/repo")
	}
}

func TestFprintCSV_AllCategories(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "archived/repo", Workflow: "ci.yml", Uses: "archived/repo@v1"},
		},
		StaleActions: []actioninfo.StaleActionInfo{
			{OwnerRepo: "stale/repo", FullRef: "stale/repo@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: "use v2"},
		},
		RuntimeEOL: []actioninfo.RuntimeEOLActionInfo{
			{OwnerRepo: "eol/repo", FullRef: "eol/repo@v1", Workflow: "ci.yml", Runtime: "node", Version: "14", EOLDate: time.Date(2023, 4, 30, 0, 0, 0, 0, time.UTC)},
		},
		OutdatedActions: []actioninfo.OutdatedActionInfo{
			{OwnerRepo: "outdated/repo", CurrentRef: "v1", LatestTag: "v2", Workflow: "ci.yml", FullRef: "outdated/repo@v1"},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 5 {
		t.Fatalf("expected 5 rows (header + 4 data), got %d", len(records))
	}

	wantCategories := []string{"archived", "eol", "outdated", "stale"}
	for i, want := range wantCategories {
		rowIdx := i + 1
		if records[rowIdx][1] != want {
			t.Errorf("row %d Category = %q, want %q", rowIdx, records[rowIdx][1], want)
		}
	}
}

func TestFprintCSV_EmptyOutput(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 1 {
		t.Fatalf("expected 1 row (header only), got %d", len(records))
	}
	if len(records[0]) != 11 {
		t.Errorf("header row has %d columns, want 11", len(records[0]))
	}
}

func TestFprintCSV_StaleDeprecated(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		StaleActions: []actioninfo.StaleActionInfo{
			{OwnerRepo: "stale/repo", FullRef: "stale/repo@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: "use v2"},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	descIdx := 10
	if records[1][descIdx] != "Deprecated: use v2" {
		t.Errorf("Description = %q, want %q", records[1][descIdx], "Deprecated: use v2")
	}
	if records[1][1] != "stale" {
		t.Errorf("Category = %q, want %q", records[1][1], "stale")
	}
}

func TestFprintCSV_StaleByAge(t *testing.T) {
	var buf bytes.Buffer
	lastUpdated := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	co := &CheckOutput{
		StaleActions: []actioninfo.StaleActionInfo{
			{OwnerRepo: "old/repo", FullRef: "old/repo@v2", Workflow: "build.yml", StaleByAge: true, LastUpdated: lastUpdated},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	descIdx := 10
	wantDesc := "Not updated since 2024-06-15"
	if records[1][descIdx] != wantDesc {
		t.Errorf("Description = %q, want %q", records[1][descIdx], wantDesc)
	}
}

func TestFprintCSV_RuntimeEOL(t *testing.T) {
	var buf bytes.Buffer
	eolDate := time.Date(2023, 4, 30, 0, 0, 0, 0, time.UTC)
	co := &CheckOutput{
		RuntimeEOL: []actioninfo.RuntimeEOLActionInfo{
			{OwnerRepo: "eol/repo", FullRef: "eol/repo@v1", Workflow: "ci.yml", Runtime: "node", Version: "14", EOLDate: eolDate},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	dataRow := records[1]
	if dataRow[7] != "node" {
		t.Errorf("Runtime = %q, want %q", dataRow[7], "node")
	}
	if dataRow[8] != "14" {
		t.Errorf("RuntimeVersion = %q, want %q", dataRow[8], "14")
	}
	if dataRow[9] != "2023-04-30" {
		t.Errorf("EOLDate = %q, want %q", dataRow[9], "2023-04-30")
	}
	if !strings.Contains(dataRow[10], "Uses EOL runtime node14") {
		t.Errorf("Description = %q, want to contain 'Uses EOL runtime node14'", dataRow[10])
	}
	if !strings.Contains(dataRow[0], "EOL runtime:") {
		t.Errorf("Summary = %q, want to contain 'EOL runtime:'", dataRow[0])
	}
}

func TestFprintCSV_Outdated(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		OutdatedActions: []actioninfo.OutdatedActionInfo{
			{OwnerRepo: "outdated/repo", CurrentRef: "v1", LatestTag: "v2", Workflow: "ci.yml", FullRef: "outdated/repo@v1"},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	dataRow := records[1]
	if dataRow[5] != "v1" {
		t.Errorf("CurrentVersion = %q, want %q", dataRow[5], "v1")
	}
	if dataRow[6] != "v2" {
		t.Errorf("LatestVersion = %q, want %q", dataRow[6], "v2")
	}
	if !strings.Contains(dataRow[10], "Current version v1 is outdated, latest is v2") {
		t.Errorf("Description = %q, want to contain version info", dataRow[10])
	}
	if !strings.Contains(dataRow[0], "Outdated action:") {
		t.Errorf("Summary = %q, want to contain 'Outdated action:'", dataRow[0])
	}
}

func TestFprintCSV_WithCSVConfig(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "archived/repo", Workflow: "ci.yml", Uses: "archived/repo@v1"},
		},
	}
	cfg := &CSVConfig{
		ExtraColumns: map[string]string{"Assignee": "user", "IssueType": "Task", "Labels": "archived", "Priority": "High", "Project": "PROJ"},
	}
	FprintCSV(&buf, co, cfg)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	headers := records[0]
	if len(headers) != 16 {
		t.Errorf("expected 16 headers, got %d: %v", len(headers), headers)
	}
	headerMap := make(map[string]int)
	for i, h := range headers {
		headerMap[h] = i
	}
	for _, col := range []string{"Project", "Assignee", "IssueType", "Priority", "Labels"} {
		if _, ok := headerMap[col]; !ok {
			t.Errorf("expected header %q not found in %v", col, headers)
		}
	}
	dataRow := records[1]
	if dataRow[headerMap["Project"]] != "PROJ" {
		t.Errorf("data Project = %q, want %q", dataRow[headerMap["Project"]], "PROJ")
	}
	if dataRow[headerMap["Assignee"]] != "user" {
		t.Errorf("data Assignee = %q, want %q", dataRow[headerMap["Assignee"]], "user")
	}
	if dataRow[headerMap["IssueType"]] != "Task" {
		t.Errorf("data IssueType = %q, want %q", dataRow[headerMap["IssueType"]], "Task")
	}
	if dataRow[headerMap["Priority"]] != "High" {
		t.Errorf("data Priority = %q, want %q", dataRow[headerMap["Priority"]], "High")
	}
	if dataRow[headerMap["Labels"]] != "archived" {
		t.Errorf("data Labels = %q, want %q", dataRow[headerMap["Labels"]], "archived")
	}
}

func TestFprintCSV_SortingByWorkflow(t *testing.T) {
	tests := []struct {
		name            string
		archivedActions []issue.ArchivedActionInfo
		wantWorkflows   []string
	}{
		{
			name: "same category sorted by workflow",
			archivedActions: []issue.ArchivedActionInfo{
				{Repo: "z/repo", Workflow: "deploy.yml", Uses: "z/repo@v1"},
				{Repo: "a/repo", Workflow: "build.yml", Uses: "a/repo@v1"},
				{Repo: "m/repo", Workflow: "ci.yml", Uses: "m/repo@v1"},
			},
			wantWorkflows: []string{"build.yml", "ci.yml", "deploy.yml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			co := &CheckOutput{
				ArchivedActions: tt.archivedActions,
			}
			FprintCSV(&buf, co, nil)

			records := readCSVRecords(t, &buf)
			if len(records) != len(tt.wantWorkflows)+1 {
				t.Fatalf("expected %d rows, got %d", len(tt.wantWorkflows)+1, len(records))
			}
			for i, want := range tt.wantWorkflows {
				if records[i+1][2] != want {
					t.Errorf("row %d Workflow = %q, want %q", i+1, records[i+1][2], want)
				}
			}
		})
	}
}

func TestFprintCSV_SortingByActionRef(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "z/repo", Workflow: "ci.yml", Uses: "z/repo@v3"},
			{Repo: "a/repo", Workflow: "ci.yml", Uses: "a/repo@v1"},
			{Repo: "m/repo", Workflow: "ci.yml", Uses: "m/repo@v2"},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(records))
	}
	wantRefs := []string{"a/repo@v1", "m/repo@v2", "z/repo@v3"}
	for i, want := range wantRefs {
		if records[i+1][3] != want {
			t.Errorf("row %d ActionRef = %q, want %q", i+1, records[i+1][3], want)
		}
	}
}

func TestFprintCSV_RuntimeEOLZeroDate(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		RuntimeEOL: []actioninfo.RuntimeEOLActionInfo{
			{OwnerRepo: "eol/repo", FullRef: "eol/repo@v1", Workflow: "ci.yml", Runtime: "node", Version: "12", EOLDate: time.Time{}},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	if records[1][9] != "" {
		t.Errorf("EOLDate = %q, want empty string for zero time", records[1][9])
	}
}

func TestFprintCSV_StaleDeprecatedNoMessage(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		StaleActions: []actioninfo.StaleActionInfo{
			{OwnerRepo: "stale/repo", FullRef: "stale/repo@v1", Workflow: "ci.yml", Deprecated: true, DeprecationMessage: ""},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	if records[1][10] != "Deprecated" {
		t.Errorf("Description = %q, want %q", records[1][10], "Deprecated")
	}
}

func TestFprintCSV_StaleByAgeZeroDate(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		StaleActions: []actioninfo.StaleActionInfo{
			{OwnerRepo: "old/repo", FullRef: "old/repo@v2", Workflow: "build.yml", StaleByAge: true, LastUpdated: time.Time{}},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	if records[1][10] != "" {
		t.Errorf("Description = %q, want empty for stale-by-age with zero LastUpdated", records[1][10])
	}
}

func TestFprintCSV_WithExtraColumns(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "archived/repo", Workflow: "ci.yml", Uses: "archived/repo@v1"},
		},
	}
	cfg := &CSVConfig{
		ExtraColumns: map[string]string{"Environment": "production", "Team": "platform"},
	}
	FprintCSV(&buf, co, cfg)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	headers := records[0]
	if headers[11] != "Environment" {
		t.Errorf("headers[11] = %q, want %q", headers[11], "Environment")
	}
	if headers[12] != "Team" {
		t.Errorf("headers[12] = %q, want %q", headers[12], "Team")
	}
	dataRow := records[1]
	if dataRow[11] != "production" {
		t.Errorf("data Environment = %q, want %q", dataRow[11], "production")
	}
	if dataRow[12] != "platform" {
		t.Errorf("data Team = %q, want %q", dataRow[12], "platform")
	}
}

func TestWriterCSVOutput(t *testing.T) {
	tests := []struct {
		name       string
		co         *CheckOutput
		cfg        *CSVConfig
		wantRows   int
		wantHeader []string
		wantData   []string
	}{
		{
			name: "basic archived with CSV config",
			co: &CheckOutput{
				ArchivedActions: []issue.ArchivedActionInfo{
					{Repo: "archived/repo", Workflow: "ci.yml", Uses: "archived/repo@v1"},
				},
			},
			cfg: &CSVConfig{
				ExtraColumns: map[string]string{"Assignee": "dev", "IssueType": "Bug", "Labels": "archived", "Priority": "High", "Project": "PROJ"},
			},
			wantRows: 2,
			wantHeader: []string{"Summary", "Category", "Workflow", "ActionRef", "OwnerRepo",
				"CurrentVersion", "LatestVersion", "Runtime", "RuntimeVersion", "EOLDate", "Description",
				"Assignee", "IssueType", "Labels", "Priority", "Project"},
			wantData: []string{"Archived action: archived/repo@v1 in ci.yml", "archived", "ci.yml",
				"archived/repo@v1", "archived/repo", "", "", "", "", "", "",
				"dev", "Bug", "archived", "High", "PROJ"},
		},
		{
			name:     "empty output with nil config",
			co:       &CheckOutput{},
			cfg:      nil,
			wantRows: 1,
			wantHeader: []string{"Summary", "Category", "Workflow", "ActionRef", "OwnerRepo",
				"CurrentVersion", "LatestVersion", "Runtime", "RuntimeVersion", "EOLDate", "Description"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := &Writer{Format: FormatCSV, Output: &buf, CSVConfig: tt.cfg}
			w.WriteCheckResult(tt.co)

			records := readCSVRecords(t, &buf)
			if len(records) != tt.wantRows {
				t.Errorf("got %d rows, want %d", len(records), tt.wantRows)
			}
			for i, h := range tt.wantHeader {
				if i >= len(records[0]) {
					t.Errorf("header missing column %d", i)
					continue
				}
				if records[0][i] != h {
					t.Errorf("header[%d] = %q, want %q", i, records[0][i], h)
				}
			}
			if tt.wantData != nil && len(records) > 1 {
				for i, d := range tt.wantData {
					if i >= len(records[1]) {
						t.Errorf("data row missing column %d", i)
						continue
					}
					if records[1][i] != d {
						t.Errorf("data[%d] = %q, want %q", i, records[1][i], d)
					}
				}
			}
		})
	}
}

func TestFprintCSV_MultipleRowsMixedCategories(t *testing.T) {
	var buf bytes.Buffer
	co := &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "z/repo", Workflow: "z.yml", Uses: "z/repo@v1"},
			{Repo: "a/repo", Workflow: "a.yml", Uses: "a/repo@v1"},
		},
		OutdatedActions: []actioninfo.OutdatedActionInfo{
			{OwnerRepo: "outdated/repo", CurrentRef: "v1", LatestTag: "v3", Workflow: "b.yml", FullRef: "outdated/repo@v1"},
			{OwnerRepo: "old/action", CurrentRef: "v2", LatestTag: "v5", Workflow: "a.yml", FullRef: "old/action@v2"},
		},
	}
	FprintCSV(&buf, co, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 5 {
		t.Fatalf("expected 5 rows (1 header + 4 data), got %d", len(records))
	}

	wantOrder := []struct {
		category  string
		workflow  string
		actionRef string
	}{
		{"archived", "a.yml", "a/repo@v1"},
		{"archived", "z.yml", "z/repo@v1"},
		{"outdated", "a.yml", "old/action@v2"},
		{"outdated", "b.yml", "outdated/repo@v1"},
	}
	for i, want := range wantOrder {
		row := records[i+1]
		if row[1] != want.category {
			t.Errorf("row %d Category = %q, want %q", i+1, row[1], want.category)
		}
		if row[2] != want.workflow {
			t.Errorf("row %d Workflow = %q, want %q", i+1, row[2], want.workflow)
		}
		if row[3] != want.actionRef {
			t.Errorf("row %d ActionRef = %q, want %q", i+1, row[3], want.actionRef)
		}
	}
}

func TestFprintCSV_WriterToIOWriter(t *testing.T) {
	var buf bytes.Buffer
	FprintCSV(io.Writer(&buf), &CheckOutput{
		ArchivedActions: []issue.ArchivedActionInfo{
			{Repo: "test/repo", Workflow: "wf.yml", Uses: "test/repo@v1"},
		},
	}, nil)

	records := readCSVRecords(t, &buf)
	if len(records) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(records))
	}
	if records[1][1] != "archived" {
		t.Errorf("Category = %q, want %q", records[1][1], "archived")
	}
}
