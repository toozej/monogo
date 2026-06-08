package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
)

// csvRow represents a single row of CSV output.
type csvRow struct {
	Summary        string
	Category       string
	Workflow       string
	ActionRef      string
	OwnerRepo      string
	CurrentVersion string
	LatestVersion  string
	Runtime        string
	RuntimeVersion string
	EOLDate        string
	Description    string
}

// sortedExtraKeys returns the ExtraColumns keys in sorted order, or nil if
// cfg is nil or has no extra columns. This ensures header/value alignment.
func sortedExtraKeys(cfg *CSVConfig) []string {
	if cfg == nil || len(cfg.ExtraColumns) == 0 {
		return nil
	}
	extraKeys := make([]string, 0, len(cfg.ExtraColumns))
	for k := range cfg.ExtraColumns {
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)
	return extraKeys
}

// csvHeaders returns the full list of CSV column headers including extra columns.
func csvHeaders(cfg *CSVConfig) []string {
	base := []string{"Summary", "Category", "Workflow", "ActionRef", "OwnerRepo", "CurrentVersion", "LatestVersion", "Runtime", "RuntimeVersion", "EOLDate", "Description"}
	extraKeys := sortedExtraKeys(cfg)
	if extraKeys == nil {
		return base
	}
	base = append(base, extraKeys...)
	return base
}

// csvRowValues converts a csvRow to a slice of string values matching the headers.
func csvRowValues(row csvRow, cfg *CSVConfig) []string {
	base := []string{row.Summary, row.Category, row.Workflow, row.ActionRef, row.OwnerRepo, row.CurrentVersion, row.LatestVersion, row.Runtime, row.RuntimeVersion, row.EOLDate, row.Description}
	extraKeys := sortedExtraKeys(cfg)
	if extraKeys == nil {
		return base
	}
	for _, k := range extraKeys {
		base = append(base, cfg.ExtraColumns[k])
	}
	return base
}

// FprintCSV writes check results in CSV format suitable for Jira bulk import
// or other data-related tooling.
func FprintCSV(w io.Writer, co *CheckOutput, cfg *CSVConfig) {
	headers := csvHeaders(cfg)
	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return
	}

	var rows []csvRow

	for _, a := range co.ArchivedActions {
		rows = append(rows, csvRow{
			Summary:   fmt.Sprintf("Archived action: %s in %s", a.Uses, a.Workflow),
			Category:  "archived",
			Workflow:  a.Workflow,
			ActionRef: a.Uses,
			OwnerRepo: a.Repo,
		})
	}

	for _, a := range co.StaleActions {
		desc := ""
		if a.Deprecated {
			desc = "Deprecated"
			if a.DeprecationMessage != "" {
				desc = fmt.Sprintf("Deprecated: %s", a.DeprecationMessage)
			}
		} else if a.StaleByAge {
			if !a.LastUpdated.IsZero() {
				desc = fmt.Sprintf("Not updated since %s", a.LastUpdated.Format("2006-01-02"))
			}
		}
		rows = append(rows, csvRow{
			Summary:     fmt.Sprintf("Stale action: %s in %s", a.FullRef, a.Workflow),
			Category:    "stale",
			Workflow:    a.Workflow,
			ActionRef:   a.FullRef,
			OwnerRepo:   a.OwnerRepo,
			Description: desc,
		})
	}

	for _, a := range co.RuntimeEOL {
		eolDateStr := ""
		if !a.EOLDate.IsZero() {
			eolDateStr = a.EOLDate.Format("2006-01-02")
		}
		rows = append(rows, csvRow{
			Summary:        fmt.Sprintf("EOL runtime: %s uses %s%s", a.FullRef, a.Runtime, a.Version),
			Category:       "eol",
			Workflow:       a.Workflow,
			ActionRef:      a.FullRef,
			OwnerRepo:      a.OwnerRepo,
			Runtime:        a.Runtime,
			RuntimeVersion: a.Version,
			EOLDate:        eolDateStr,
			Description:    fmt.Sprintf("Uses EOL runtime %s%s", a.Runtime, a.Version),
		})
	}

	for _, a := range co.OutdatedActions {
		rows = append(rows, csvRow{
			Summary:        fmt.Sprintf("Outdated action: %s (latest: %s)", a.FullRef, a.LatestTag),
			Category:       "outdated",
			Workflow:       a.Workflow,
			ActionRef:      a.FullRef,
			OwnerRepo:      a.OwnerRepo,
			CurrentVersion: a.CurrentRef,
			LatestVersion:  a.LatestTag,
			Description:    fmt.Sprintf("Current version %s is outdated, latest is %s", a.CurrentRef, a.LatestTag),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Category != rows[j].Category {
			return rows[i].Category < rows[j].Category
		}
		if rows[i].Workflow != rows[j].Workflow {
			return rows[i].Workflow < rows[j].Workflow
		}
		return rows[i].ActionRef < rows[j].ActionRef
	})

	for _, row := range rows {
		if err := cw.Write(csvRowValues(row, cfg)); err != nil {
			return
		}
	}

	cw.Flush()
}
