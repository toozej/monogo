package actioninfo

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/go-sort-out-gh-actions/internal/github"
	ver "github.com/toozej/go-sort-out-gh-actions/internal/version"
	"github.com/toozej/go-sort-out-gh-actions/internal/workflow"
)

var IsTTY = checkTTY()

func checkTTY() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func Emoji(emoji, fallback string) string {
	if IsTTY {
		return emoji
	}
	return fallback
}

func WriteActionOutput(key, value string) {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		return
	}
	dir := filepath.Dir(outputFile)
	base := filepath.Base(outputFile)

	root, err := os.OpenRoot(dir)
	if err != nil {
		log.Warnf("Failed to open root directory for GITHUB_OUTPUT: %v", err)
		return
	}
	defer root.Close()

	f, err := root.OpenFile(base, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Warnf("Failed to open GITHUB_OUTPUT file: %v", err)
		return
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s=%s\n", key, value); err != nil {
		log.Warnf("Failed to write to GITHUB_OUTPUT file: %v", err)
	}
}

func SanitizeStaleDays(days int) int {
	if days <= 0 {
		return DefaultStaleDays
	}
	if days > MaxStaleDays {
		return MaxStaleDays
	}
	return days
}

func GetOwnerRepos(allActionRefs []workflow.ActionRef) []string {
	ownerRepos := make([]string, 0, len(allActionRefs))
	seen := make(map[string]bool)
	for _, ref := range allActionRefs {
		if !seen[ref.OwnerRepo] {
			seen[ref.OwnerRepo] = true
			ownerRepos = append(ownerRepos, ref.OwnerRepo)
		}
	}
	return ownerRepos
}

func GetNonArchivedRepos(allActionRefs []workflow.ActionRef, archived map[string]bool) []string {
	var nonArchivedRepos []string
	for _, ref := range allActionRefs {
		if isArchived, exists := archived[ref.OwnerRepo]; !exists || !isArchived {
			nonArchivedRepos = append(nonArchivedRepos, ref.OwnerRepo)
		}
	}
	return RemoveDuplicates(nonArchivedRepos)
}

func RemoveDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	return result
}

func ExpandPath(path, workDir string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get user home directory: %v", err)
		}
		return filepath.Join(home, path[2:])
	}

	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(workDir, path)
}

func GetRepoName(workDir string) string {
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		return repo
	}
	return "current-repo"
}

func LogWorkflowInfo(w io.Writer, verbose bool, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef) {
	if verbose {
		fmt.Fprintf(w, "Found %d workflow files\n", len(workflowFiles))
		for _, wf := range workflowFiles {
			fmt.Fprintf(w, " - %s (%d uses)\n", wf.Path, len(wf.UsesWithVersions))
		}
		fmt.Fprintf(w, "Extracted %d unique action references\n", len(allActionRefs))
		if len(allActionRefs) > 0 {
			for _, ref := range allActionRefs {
				if ref.FullRef != "" {
					fmt.Fprintf(w, " - %s\n", ref.FullRef)
					continue
				}
				fmt.Fprintf(w, " - %s@%s\n", ref.OwnerRepo, ref.Version)
			}
		}
	}
}

func CheckOutdatedActions(ctx context.Context, ghClient *github.Client, workflowFiles []*workflow.WorkflowFile, archived map[string]bool, releases map[string]*github.ReleaseInfo, verbose bool) []OutdatedActionInfo {
	var outdatedActions []OutdatedActionInfo
	seenOutdated := make(map[string]bool)

	for _, wf := range workflowFiles {
		for _, ref := range wf.UsesWithVersions {
			if isArchived, exists := archived[ref.OwnerRepo]; exists && isArchived {
				continue
			}

			release, hasRelease := releases[ref.OwnerRepo]
			if !hasRelease {
				continue
			}

			latestTag := release.TagName
			latestURL := release.HTMLURL

			if _, err := ver.IsVersionOutdated(ref.Version, latestTag); err != nil {
				fallbackTag, fallbackErr := ghClient.GetLatestSemverTag(ctx, ref.OwnerRepo)
				if fallbackErr == nil && fallbackTag != "" && fallbackTag != latestTag {
					if verbose {
						fmt.Printf(" Using semver tag fallback for %s: %s -> %s\n", ref.OwnerRepo, latestTag, fallbackTag)
					}
					latestTag = fallbackTag
					latestURL = fmt.Sprintf("https://github.com/%s/releases/tag/%s", ref.OwnerRepo, latestTag)
				}
			}

			cacheKey := wf.Path + ":" + ref.FullRef + ":" + latestTag
			if seenOutdated[cacheKey] {
				continue
			}

			if ver.IsMajorVersionTag(ref.Version) {
				if ver.SameMajorVersion(ref.Version, latestTag) {
					same, _, _, err := ghClient.CompareRefSHAs(ctx, ref.OwnerRepo, ref.Version, latestTag)
					if err != nil {
						if verbose {
							fmt.Printf(" Cannot compare SHAs for %s@%s vs %s: %v\n", ref.OwnerRepo, ref.Version, latestTag, err)
						}
						seenOutdated[cacheKey] = true
						continue
					}
					if same {
						seenOutdated[cacheKey] = true
						continue
					}
					outdatedActions = append(outdatedActions, OutdatedActionInfo{
						OwnerRepo:  ref.OwnerRepo,
						ActionPath: ref.ActionPath,
						CurrentRef: ref.Version,
						LatestTag:  latestTag,
						LatestURL:  latestURL,
						Workflow:   filepath.Base(wf.Path),
						FullRef:    ref.FullRef,
					})
					seenOutdated[cacheKey] = true
					continue
				}
			}

			isOutdated, err := ver.IsVersionOutdated(ref.Version, latestTag)
			if err != nil {
				if verbose {
					fmt.Printf(" Cannot compare versions for %s: %v\n", ref.OwnerRepo, err)
				}
				seenOutdated[cacheKey] = true
				continue
			}

			if isOutdated {
				outdatedActions = append(outdatedActions, OutdatedActionInfo{
					OwnerRepo:  ref.OwnerRepo,
					ActionPath: ref.ActionPath,
					CurrentRef: ref.Version,
					LatestTag:  latestTag,
					LatestURL:  latestURL,
					Workflow:   filepath.Base(wf.Path),
					FullRef:    ref.FullRef,
				})
				seenOutdated[cacheKey] = true
				continue
			}
		}

	}

	return outdatedActions
}

func WriteOutdatedActions(ctx context.Context, ghClient *github.Client, workflowFiles []*workflow.WorkflowFile, outdatedActions []OutdatedActionInfo, releases map[string]*github.ReleaseInfo, useSemver bool, verbose bool) OutdatedUpdateReport {
	report := OutdatedUpdateReport{
		UpdatedByFile: make(map[string][]FileUpdate),
	}

	if len(outdatedActions) == 0 {
		return report
	}

	type update struct {
		ownerRepo string
		latestTag string
	}

	uniqueUpdates := make(map[string]update)
	for _, action := range outdatedActions {
		if _, exists := uniqueUpdates[action.OwnerRepo+"@"+action.LatestTag]; exists {
			continue
		}
		uniqueUpdates[action.OwnerRepo+"@"+action.LatestTag] = update{
			ownerRepo: action.OwnerRepo,
			latestTag: action.LatestTag,
		}
	}

	shaCache := make(map[string]string)
	shaResolveErrs := make(map[string]string)
	if !useSemver {
		for key, upd := range uniqueUpdates {
			sha, err := ghClient.GetRefSHA(ctx, upd.ownerRepo, upd.latestTag)
			if err != nil {
				reason := fmt.Sprintf("failed to resolve %s@%s to a commit SHA: %v", upd.ownerRepo, upd.latestTag, err)
				shaResolveErrs[key] = reason
				log.Warn(reason)
				continue
			}
			shaCache[key] = sha
			if verbose {
				fmt.Printf(" Resolved %s@%s -> %s\n", upd.ownerRepo, upd.latestTag, sha)
			}
		}
	}

	pendingByFile := make(map[string][]FileUpdate)
	for _, action := range outdatedActions {
		key := action.OwnerRepo + "@" + action.LatestTag
		oldUse := action.FullRef
		if oldUse == "" {
			oldUse = workflow.ActionRef{OwnerRepo: action.OwnerRepo, ActionPath: action.ActionPath, Version: action.CurrentRef}.Key()
		}

		var newUse string
		if useSemver {
			newUse = workflow.ActionRef{OwnerRepo: action.OwnerRepo, ActionPath: action.ActionPath, Version: action.LatestTag}.Key()
		} else {
			sha, ok := shaCache[key]
			if !ok {
				reason := shaResolveErrs[key]
				if reason == "" {
					reason = fmt.Sprintf("failed to resolve %s to a commit SHA", key)
				}
				report.FailedUpdates = append(report.FailedUpdates, OutdatedUpdateFailure{
					WorkflowFile: action.Workflow,
					OldUse:       oldUse,
					NewUse:       workflow.ActionRef{OwnerRepo: action.OwnerRepo, ActionPath: action.ActionPath, Version: action.LatestTag}.Key(),
					Reason:       reason,
				})
				continue
			}
			newUse = workflow.ActionRef{OwnerRepo: action.OwnerRepo, ActionPath: action.ActionPath, Version: sha}.Key() + " # " + action.LatestTag
		}

		matched := false
		for _, wf := range workflowFiles {
			if filepath.Base(wf.Path) == action.Workflow || wf.Path == action.Workflow {
				pendingByFile[wf.Path] = append(pendingByFile[wf.Path], FileUpdate{
					OldUse: oldUse,
					NewUse: newUse,
				})
				matched = true
			}
		}

		if !matched {
			report.FailedUpdates = append(report.FailedUpdates, OutdatedUpdateFailure{
				WorkflowFile: action.Workflow,
				OldUse:       oldUse,
				NewUse:       newUse,
				Reason:       "workflow file could not be matched to a writable file path",
			})
		}
	}

	filePaths := make([]string, 0, len(pendingByFile))
	for filePath := range pendingByFile {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths)

	for _, filePath := range filePaths {
		updates := pendingByFile[filePath]
		if err := ApplyUpdatesToFile(filePath, updates); err != nil {
			reason := fmt.Sprintf("failed to write updates to file: %v", err)
			for _, update := range updates {
				report.FailedUpdates = append(report.FailedUpdates, OutdatedUpdateFailure{
					WorkflowFile: filePath,
					OldUse:       update.OldUse,
					NewUse:       update.NewUse,
					Reason:       reason,
				})
			}
			continue
		}

		report.UpdatedByFile[filePath] = updates
	}

	return report
}

func OutdatedUpdateCount(report OutdatedUpdateReport) int {
	total := 0
	for _, updates := range report.UpdatedByFile {
		total += len(updates)
	}

	return total
}

func OutdatedUpdateFailureCount(report OutdatedUpdateReport) int {
	return len(report.FailedUpdates)
}

func PrintOutdatedUpdateReport(w io.Writer, report OutdatedUpdateReport) {
	updatedCount := OutdatedUpdateCount(report)
	failureCount := OutdatedUpdateFailureCount(report)

	if updatedCount > 0 {
		fmt.Fprintf(w, "\n%s Updated %d GitHub Action use(s):\n", Emoji("✅ ", "[OK] "), updatedCount)
		filePaths := make([]string, 0, len(report.UpdatedByFile))
		for filePath := range report.UpdatedByFile {
			filePaths = append(filePaths, filePath)
		}
		sort.Strings(filePaths)

		for _, filePath := range filePaths {
			fmt.Fprintf(w, "  %s:\n", filePath)
			for _, update := range report.UpdatedByFile[filePath] {
				fmt.Fprintf(w, "    %s -> %s\n", update.OldUse, update.NewUse)
			}
		}
	}

	if failureCount > 0 {
		fmt.Fprintf(w, "\n%s Could not update %d GitHub Action use(s):\n", Emoji("⚠️ ", "[WARN] "), failureCount)
		failures := append([]OutdatedUpdateFailure(nil), report.FailedUpdates...)
		sort.Slice(failures, func(i, j int) bool {
			if failures[i].WorkflowFile != failures[j].WorkflowFile {
				return failures[i].WorkflowFile < failures[j].WorkflowFile
			}
			return failures[i].OldUse < failures[j].OldUse
		})

		for _, failure := range failures {
			fmt.Fprintf(w, "  %s (%s)\n", failure.OldUse, failure.WorkflowFile)
			fmt.Fprintf(w, "    target: %s\n", failure.NewUse)
			fmt.Fprintf(w, "    reason: %s\n", failure.Reason)
		}
	}
}

func BuildOutdatedUpdateSummary(report OutdatedUpdateReport) string {
	updatedCount := OutdatedUpdateCount(report)
	failureCount := OutdatedUpdateFailureCount(report)

	switch {
	case updatedCount > 0 && failureCount == 0:
		return "\n" + Emoji("✅ ", "[OK] ") + fmt.Sprintf("Updated %d GitHub Action use(s) to the latest available refs.", updatedCount)
	case updatedCount > 0 && failureCount > 0:
		return "\n" + Emoji("⚠️ ", "[WARN] ") + fmt.Sprintf("Updated %d GitHub Action use(s), but %d could not be updated.", updatedCount, failureCount)
	case failureCount > 0:
		return "\n" + Emoji("❌ ", "[X] ") + fmt.Sprintf("%d GitHub Action use(s) could not be updated automatically.", failureCount)
	default:
		return "\n" + Emoji("⚠️ ", "[WARN] ") + "Outdated actions were detected, but no updates were applied."
	}
}

func ApplyUpdatesToFile(filePath string, updates []FileUpdate) error {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("failed to open root directory: %w", err)
	}
	defer root.Close()

	f, err := root.Open(base)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	defer f.Close()

	content, err := readAll(f)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	result := content
	for _, upd := range updates {
		result = ReplaceUsesLine(result, upd.OldUse, upd.NewUse)
	}

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if err := root.WriteFile(base, result, info.Mode()); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func readAll(f *os.File) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ReplaceUsesLine(content []byte, oldUse, newUse string) []byte {
	endsWithNewline := len(content) > 0 && content[len(content)-1] == '\n'
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var buf bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if matchesUsesLine(line, oldUse) {
			line = buildReplacementLine(line, oldUse, newUse)
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return content
	}

	result := buf.Bytes()
	if !endsWithNewline && len(result) > 0 {
		result = result[:len(result)-1]
	}

	return result
}

// parseNewUse separates newUse into the reference portion and an optional
// semver comment. For SHA mode newUse like "owner/repo@sha # v1.2.3",
// it returns ("owner/repo@sha", "v1.2.3"). For semver mode newUse like
// "owner/repo@v1.2.3" (no inline comment), it returns ("owner/repo@v1.2.3", "").
func parseNewUse(newUse string) (refPart, semverComment string) {
	if idx := strings.Index(newUse, " # "); idx != -1 {
		return newUse[:idx], newUse[idx+3:]
	}
	return newUse, ""
}

// buildReplacementLine replaces oldUse with newUse in a matched line,
// preserving any existing trailing inline comment.
//
// It handles three scenarios:
//  1. Line has no trailing comment → use newUse as-is (e.g. "owner/repo@sha # v1.2.3")
//  2. Line has an existing semver comment (# vN.N.N from a prior SHA-pin) →
//     replace it with the new semver comment, preserve other comments
//  3. Line has a non-semver comment (e.g. # nosemgrep: ...) →
//     insert the new semver comment before the existing comment
func buildReplacementLine(line, oldUse, newUse string) string {
	target := "uses: " + oldUse
	idx := strings.Index(line, target)
	indent := ""
	if idx > 0 {
		indent = line[:idx]
	}

	// Extract the trailing part of the line after oldUse.
	afterIdx := idx + len(target)
	trailing := ""
	if afterIdx < len(line) {
		trailing = line[afterIdx:]
	}

	refPart, semverComment := parseNewUse(newUse)

	// No trailing content on original line — use newUse as-is.
	if trailing == "" {
		return indent + "uses: " + newUse
	}

	// trailing starts with " #" (guaranteed by matchesUsesLine).
	// Strip the leading space so commentText starts with "#".
	commentText := trailing[1:] // e.g. "# nosemgrep: ..." or "# v3 # nosemgrep: ..."

	// If newUse has no semver comment (semver mode like "owner/repo@v4.1.2"),
	// preserve the existing trailing comment as-is.
	if semverComment == "" {
		return indent + "uses: " + refPart + " " + commentText
	}

	// newUse has a semver comment (SHA mode).
	// Check if the existing comment starts with a semver pattern like "# v3" or "# v3.2.1"
	// which would be from a prior SHA-pin operation.
	semverPattern := regexp.MustCompile(`^# v\S+(?:\s|$)`)
	if semverPattern.MatchString(commentText) {
		// Replace the existing semver comment with the new one.
		// e.g. "# v3 # nosemgrep: ..." → "# v4.1.2 # nosemgrep: ..."
		// e.g. "# v3" → "# v4.1.2"
		match := semverPattern.FindStringIndex(commentText)
		if match == nil {
			// Should not happen since MatchString returned true, but handle gracefully.
			return indent + "uses: " + refPart + " # " + semverComment + " " + commentText
		}
		endOfOldSemver := match[1]
		// Skip any trailing whitespace from the match since we'll add our own spacing.
		rest := strings.TrimLeft(commentText[endOfOldSemver:], " ")
		if rest == "" {
			// Entire comment was just the semver annotation (e.g. "# v3") — replace it.
			return indent + "uses: " + refPart + " # " + semverComment
		}
		// There's content after the old semver comment — replace old semver, keep the rest.
		return indent + "uses: " + refPart + " # " + semverComment + " " + rest
	}

	// Existing comment is not a semver annotation (e.g. "# nosemgrep: ...").
	// Insert the new semver comment before the existing comment.
	// e.g. "# nosemgrep: ..." → "# v4.1.2 # nosemgrep: ..."
	return indent + "uses: " + refPart + " # " + semverComment + " " + commentText
}

func matchesUsesLine(line, oldUse string) bool {
	target := "uses: " + oldUse
	idx := strings.Index(line, target)
	if idx < 0 {
		return false
	}
	after := idx + len(target)
	if after < len(line) {
		ch := line[after]
		return ch == ' ' && strings.HasPrefix(line[after:], " #") || ch == '\n' || ch == '\r'
	}
	return true
}
