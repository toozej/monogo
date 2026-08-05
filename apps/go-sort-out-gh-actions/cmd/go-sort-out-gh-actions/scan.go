package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/securityscan"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
)

func newScanCmd() *cobra.Command {
	var minSeverity string

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan GitHub Actions workflows for security risks",
		Long: `Scan GitHub Actions workflows for dangerous triggers, poisoned pipeline execution,
unsafe secret or artifact handling, excessive permissions, runner exposure, and unpinned actions.
The scanner works locally and supports --workflow, --workflows-dir, and --repos-dir.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			runScan(minSeverity)
		},
	}

	cmd.Flags().StringVar(&minSeverity, "min-severity", string(securityscan.SeverityLow), "Only report findings at or above: low, medium, high, critical")
	return cmd
}

func runScan(minSeverity string) {
	minimum, err := parseSeverity(minSeverity)
	if err != nil {
		log.Error(err)
		exitCode = 1
		return
	}

	workDir, err := os.Getwd()
	if err != nil {
		log.Errorf("Failed to get working directory: %v", err)
		exitCode = 1
		return
	}

	scanner := securityscan.NewScanner()
	if reposDir != "" {
		reposDir = actioninfo.ExpandPath(reposDir, workDir)
		repos, findErr := workflow.NewParser().FindReposWithWorkflows(reposDir)
		if findErr != nil {
			log.Errorf("Failed to find repositories with workflows: %v", findErr)
			exitCode = 1
			return
		}
		if len(repos) == 0 {
			fmt.Println("No repositories with .github/workflows found in the specified directory")
			return
		}

		hasIssues := false
		for _, repo := range repos {
			fmt.Printf("\n%sScanning: %s\n", actioninfo.Emoji("📁 ", "[SCAN] "), repo)
			findings, scanErr := scanWorkflowPaths(scanner, workflow.NewParser(), repo, "", "")
			if scanErr != nil {
				log.Errorf("Failed to scan workflows in %s: %v", repo, scanErr)
				hasIssues = true
				continue
			}
			findings = filterFindings(findings, minimum)
			writeScanFindings(findings)
			hasIssues = hasIssues || len(findings) > 0
		}
		if hasIssues {
			exitCode = 1
		}
		return
	}

	findings, err := scanWorkflowPaths(scanner, workflow.NewParser(), workDir, workflowPath, workflowsDir)
	if err != nil {
		log.Errorf("Failed to scan workflows: %v", err)
		exitCode = 1
		return
	}
	findings = filterFindings(findings, minimum)
	writeScanFindings(findings)
	if len(findings) > 0 {
		exitCode = 1
	}
}

func scanWorkflowPaths(scanner *securityscan.Scanner, parser *workflow.WorkflowParser, workDir, filePath, directory string) ([]securityscan.Finding, error) {
	var paths []string
	var err error
	switch {
	case filePath != "":
		paths = []string{actioninfo.ExpandPath(filePath, workDir)}
	case directory != "":
		paths, err = parser.FindWorkflowFilesInDir(actioninfo.ExpandPath(directory, workDir))
	default:
		paths, err = parser.FindWorkflowFiles(workDir)
	}
	if err != nil {
		return nil, err
	}

	var findings []securityscan.Finding
	for _, path := range paths {
		fileFindings, scanErr := scanner.ScanFile(path)
		if scanErr != nil {
			return nil, scanErr
		}
		findings = append(findings, fileFindings...)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].RuleID < findings[j].RuleID
	})
	return findings, nil
}

func parseSeverity(value string) (securityscan.Severity, error) {
	severity := securityscan.Severity(value)
	switch severity {
	case securityscan.SeverityLow, securityscan.SeverityMedium, securityscan.SeverityHigh, securityscan.SeverityCritical:
		return severity, nil
	default:
		return "", fmt.Errorf("invalid --min-severity %q: valid values are low, medium, high, critical", value)
	}
}

func filterFindings(findings []securityscan.Finding, minimum securityscan.Severity) []securityscan.Finding {
	minimumRank := severityRank(minimum)
	filtered := make([]securityscan.Finding, 0, len(findings))
	for _, finding := range findings {
		if severityRank(finding.Severity) >= minimumRank {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

func severityRank(severity securityscan.Severity) int {
	switch severity {
	case securityscan.SeverityCritical:
		return 4
	case securityscan.SeverityHigh:
		return 3
	case securityscan.SeverityMedium:
		return 2
	default:
		return 1
	}
}

func writeScanFindings(findings []securityscan.Finding) {
	if len(findings) == 0 {
		fmt.Println(actioninfo.Emoji("✅ ", "[OK] ") + "No GitHub Actions workflow security findings found!")
		return
	}

	switch outputFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(findings); err != nil {
			log.Errorf("Failed to write JSON findings: %v", err)
		}
	case "csv":
		writer := csv.NewWriter(os.Stdout)
		_ = writer.Write([]string{"rule_id", "severity", "path", "line", "message", "remediation"})
		for _, finding := range findings {
			_ = writer.Write([]string{finding.RuleID, string(finding.Severity), finding.Path, fmt.Sprintf("%d", finding.Line), finding.Message, finding.Remediation})
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			log.Errorf("Failed to write CSV findings: %v", err)
		}
	default:
		for _, finding := range findings {
			fmt.Printf("%s%s %s:%d %s\n  Remediation: %s\n", actioninfo.Emoji("🚨 ", "[SECURITY] "), finding.RuleID, finding.Path, finding.Line, finding.Message, finding.Remediation)
		}
	}
}
