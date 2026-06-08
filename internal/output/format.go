package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/toozej/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/go-sort-out-gh-actions/internal/issue"
)

type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
	FormatCSV  Format = "csv"
)

func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case FormatText:
		return FormatText, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatCSV:
		return FormatCSV, nil
	default:
		return "", fmt.Errorf("invalid output format %q: valid values are text, json, csv", s)
	}
}

type CheckOutput struct {
	ArchivedActions []issue.ArchivedActionInfo        `json:"archived_actions,omitempty"`
	StaleActions    []actioninfo.StaleActionInfo      `json:"stale_actions,omitempty"`
	RuntimeEOL      []actioninfo.RuntimeEOLActionInfo `json:"runtime_eol_actions,omitempty"`
	OutdatedActions []actioninfo.OutdatedActionInfo   `json:"outdated_actions,omitempty"`
	ArchivedRepos   []string                          `json:"archived_repos,omitempty"`
	HasIssues       bool                              `json:"has_issues"`
	Summary         string                            `json:"summary,omitempty"`
	NoIssuesMessage string                            `json:"-"`
}

type Writer struct {
	Format    Format
	Output    io.Writer
	CSVConfig *CSVConfig
}

func NewWriter(format Format) *Writer {
	return &Writer{
		Format: format,
		Output: os.Stdout,
	}
}

func NewWriterWithCSVConfig(format Format, cfg *CSVConfig) *Writer {
	return &Writer{
		Format:    format,
		Output:    os.Stdout,
		CSVConfig: cfg,
	}
}

// NewWriterWithOptionalCSV returns a Writer, using CSVConfig if provided.
// This is the single canonical entry point for Writer creation to avoid
// divergent logic across call sites.
func NewWriterWithOptionalCSV(format Format, csvCfg *CSVConfig) *Writer {
	if csvCfg != nil {
		return NewWriterWithCSVConfig(format, csvCfg)
	}
	return NewWriter(format)
}

func (w *Writer) WriteCheckResult(co *CheckOutput) {
	switch w.Format {
	case FormatJSON:
		w.writeJSON(co)
	case FormatCSV:
		FprintCSV(w.Output, co, w.CSVConfig)
	default:
		w.writeText(co)
	}
}

func (w *Writer) writeJSON(co *CheckOutput) {
	enc := json.NewEncoder(w.Output)
	enc.SetIndent("", "  ")
	if err := enc.Encode(co); err != nil {
		fmt.Fprintf(w.Output, "error encoding JSON output: %v\n", err)
	}
}

func (w *Writer) writeText(co *CheckOutput) {
	if !co.HasIssues && co.NoIssuesMessage != "" {
		fmt.Fprintln(w.Output, co.NoIssuesMessage)
		return
	}
	if len(co.ArchivedActions) > 0 {
		FprintArchivedText(w.Output, co.ArchivedActions, co.ArchivedRepos)
	}
	if len(co.StaleActions) > 0 {
		FprintStaleText(w.Output, co.StaleActions)
	}
	if len(co.RuntimeEOL) > 0 {
		FprintRuntimeEOLText(w.Output, co.RuntimeEOL)
	}
	if len(co.OutdatedActions) > 0 {
		FprintOutdatedText(w.Output, co.OutdatedActions)
	}
	if co.Summary != "" {
		fmt.Fprintln(w.Output, co.Summary)
	}
}

func PrintArchivedText(actions []issue.ArchivedActionInfo, repos []string) {
	if len(actions) == 0 {
		return
	}
	FprintArchivedText(os.Stdout, actions, repos)
}

func FprintArchivedText(w io.Writer, actions []issue.ArchivedActionInfo, repos []string) {
	fmt.Fprintf(w, "\n%sFound %d archived GitHub Actions in %d uses:\n\n", actioninfo.Emoji("🚨 ", "[ARCHIVED] "), len(repos), len(actions))

	workflowMap := make(map[string][]issue.ArchivedActionInfo)
	for _, action := range actions {
		workflowMap[action.Workflow] = append(workflowMap[action.Workflow], action)
	}

	workflows := make([]string, 0, len(workflowMap))
	for wf := range workflowMap {
		workflows = append(workflows, wf)
	}
	sort.Strings(workflows)

	for _, wf := range workflows {
		actions := workflowMap[wf]
		fmt.Fprintf(w, "%s%s:\n", actioninfo.Emoji("📄 ", "[FILE] "), wf)
		for _, action := range actions {
			fmt.Fprintf(w, " %s%s\n", actioninfo.Emoji("❌ ", "[X] "), action.Uses)
		}
		fmt.Fprintln(w)
	}
}

func PrintStaleText(actions []actioninfo.StaleActionInfo) {
	if len(actions) == 0 {
		return
	}
	FprintStaleText(os.Stdout, actions)
}

func FprintStaleText(w io.Writer, actions []actioninfo.StaleActionInfo) {
	uniqueStale := make(map[string]bool)
	for _, action := range actions {
		uniqueStale[action.OwnerRepo] = true
	}

	fmt.Fprintf(w, "\n%sFound %d stale/deprecated GitHub Actions in %d uses:\n\n", actioninfo.Emoji("⏳ ", "[STALE] "), len(uniqueStale), len(actions))

	staleWorkflowMap := make(map[string][]actioninfo.StaleActionInfo)
	for _, action := range actions {
		staleWorkflowMap[action.Workflow] = append(staleWorkflowMap[action.Workflow], action)
	}

	workflows := make([]string, 0, len(staleWorkflowMap))
	for wf := range staleWorkflowMap {
		workflows = append(workflows, wf)
	}
	sort.Strings(workflows)

	for _, wf := range workflows {
		actions := staleWorkflowMap[wf]
		fmt.Fprintf(w, "%s%s:\n", actioninfo.Emoji("📄 ", "[FILE] "), wf)
		for _, action := range actions {
			if action.Deprecated {
				msg := action.DeprecationMessage
				if msg != "" {
					msg = ": " + msg
				}
				fmt.Fprintf(w, " %s%s (DEPRECATED%s)\n", actioninfo.Emoji("⏳ ", "[STALE] "), action.FullRef, msg)
			} else if action.StaleByAge {
				fmt.Fprintf(w, " %s%s (not updated since %s)\n", actioninfo.Emoji("⏳ ", "[STALE] "), action.FullRef, action.LastUpdated.Format("2006-01-02"))
			}
		}
		fmt.Fprintln(w)
	}
}

func PrintRuntimeEOLText(actions []actioninfo.RuntimeEOLActionInfo) {
	if len(actions) == 0 {
		return
	}
	FprintRuntimeEOLText(os.Stdout, actions)
}

func FprintRuntimeEOLText(w io.Writer, actions []actioninfo.RuntimeEOLActionInfo) {
	uniqueRuntimeEOL := make(map[string]bool)
	for _, action := range actions {
		uniqueRuntimeEOL[action.OwnerRepo] = true
	}

	fmt.Fprintf(w, "\n%sFound %d actions using EOL runtimes in %d uses:\n\n", actioninfo.Emoji("🖥️ ", "[RUNTIME] "), len(uniqueRuntimeEOL), len(actions))

	runtimeWorkflowMap := make(map[string][]actioninfo.RuntimeEOLActionInfo)
	for _, action := range actions {
		runtimeWorkflowMap[action.Workflow] = append(runtimeWorkflowMap[action.Workflow], action)
	}

	workflows := make([]string, 0, len(runtimeWorkflowMap))
	for wf := range runtimeWorkflowMap {
		workflows = append(workflows, wf)
	}
	sort.Strings(workflows)

	for _, wf := range workflows {
		actions := runtimeWorkflowMap[wf]
		fmt.Fprintf(w, "%s%s:\n", actioninfo.Emoji("📄 ", "[FILE] "), wf)
		for _, action := range actions {
			eolDateStr := action.EOLDate.Format("2006-01-02")
			if action.EOLDate.IsZero() {
				eolDateStr = "unknown"
			}
			fmt.Fprintf(w, " %s%s (uses %s%s, EOL since %s)\n", actioninfo.Emoji("🖥️ ", "[RUNTIME] "), action.FullRef, action.Runtime, action.Version, eolDateStr)
		}
		fmt.Fprintln(w)
	}
}

func PrintOutdatedText(actions []actioninfo.OutdatedActionInfo) {
	if len(actions) == 0 {
		return
	}
	FprintOutdatedText(os.Stdout, actions)
}

func FprintOutdatedText(w io.Writer, actions []actioninfo.OutdatedActionInfo) {
	uniqueOutdated := make(map[string]bool)
	for _, action := range actions {
		uniqueOutdated[action.OwnerRepo] = true
	}

	fmt.Fprintf(w, "\n%sFound %d outdated GitHub Actions in %d uses:\n\n", actioninfo.Emoji("⚠️ ", "[WARN] "), len(uniqueOutdated), len(actions))

	outdatedWorkflowMap := make(map[string][]actioninfo.OutdatedActionInfo)
	for _, action := range actions {
		outdatedWorkflowMap[action.Workflow] = append(outdatedWorkflowMap[action.Workflow], action)
	}

	workflows := make([]string, 0, len(outdatedWorkflowMap))
	for wf := range outdatedWorkflowMap {
		workflows = append(workflows, wf)
	}
	sort.Strings(workflows)

	for _, wf := range workflows {
		actions := outdatedWorkflowMap[wf]
		fmt.Fprintf(w, "%s%s:\n", actioninfo.Emoji("📄 ", "[FILE] "), wf)
		for _, action := range actions {
			refLabel := action.FullRef
			if refLabel == "" {
				refLabel = fmt.Sprintf("%s@%s", action.OwnerRepo, action.CurrentRef)
			}
			fmt.Fprintf(w, " %s%s (latest: %s)\n", actioninfo.Emoji("⚠️ ", "[WARN] "), refLabel, action.LatestTag)
		}
		fmt.Fprintln(w)
	}
}
