package cmd

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/go-find-archived-gh-actions/internal/actioninfo"
	"github.com/toozej/go-find-archived-gh-actions/internal/workflow"
	"github.com/toozej/go-find-archived-gh-actions/pkg/config"
	"github.com/toozej/go-find-archived-gh-actions/pkg/man"
	"github.com/toozej/go-find-archived-gh-actions/pkg/version"
)

var (
	conf         config.Config
	debug        bool
	verbose      bool
	workflowPath string
	workflowsDir string
	reposDir     string
	githubToken  string
	notify       bool
	createIssue  bool
)

var rootCmd = &cobra.Command{
	Use:   "go-find-archived-gh-actions",
	Short: "Detect archived GitHub Actions in repository workflows",
	Long: `A tool to detect if GitHub Actions used in repository workflows have been archived upstream.

The tool scans .github/workflows/**/*.yml and **/*.yaml files, extracts 'uses:' references,
checks the GitHub API for archived status, and reports findings.

Exit codes:
0 - No issues found
1 - Issues found or error occurred`,
	PersistentPreRun: rootCmdPreRun,
}

func rootCmdPreRun(cmd *cobra.Command, args []string) {
	if debug {
		log.SetLevel(log.DebugLevel)
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func init() {
	conf = config.GetEnvVars()

	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug-level logging")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output")
	rootCmd.PersistentFlags().StringVarP(&githubToken, "token", "t", "", "GitHub token (overrides GH_TOKEN/GITHUB_TOKEN env vars)")
	rootCmd.PersistentFlags().BoolVar(&notify, "notify", false, "Send notifications to configured endpoints")
	rootCmd.PersistentFlags().BoolVar(&createIssue, "create-issue", false, "Create GitHub issue when archived actions found")
	rootCmd.PersistentFlags().StringVar(&workflowPath, "workflow", "", "Path to specific workflow file to check")
	rootCmd.PersistentFlags().StringVar(&workflowsDir, "workflows-dir", "", "Path to directory containing workflow yaml files")
	rootCmd.PersistentFlags().StringVar(&reposDir, "repos-dir", "", "Path to base directory containing multiple repos to scan")

	rootCmd.AddCommand(
		newArchivedCmd(),
		newEOLCmd(),
		newOutdatedCmd(),
		newCheckCmd(),
		man.NewManCmd(),
		version.Command(),
	)
}

func resolveToken() string {
	token := conf.GitHubToken
	if token == "" {
		token = conf.GitHubTokenFallback
	}
	if githubToken != "" {
		token = githubToken
	}
	if token == "" {
		log.Fatal("GitHub token not provided. Set GH_TOKEN or GITHUB_TOKEN environment variable, or use --token flag")
	}
	return token
}

func resolveWorkflowFiles(parser *workflow.WorkflowParser, workDir string) ([]*workflow.WorkflowFile, []workflow.ActionRef) {
	var workflowFiles []*workflow.WorkflowFile
	var allActionRefs []workflow.ActionRef

	switch {
	case workflowPath != "":
		workflowPath = actioninfo.ExpandPath(workflowPath, workDir)
		workflowFile, err := parser.ParseWorkflowFile(workflowPath)
		if err != nil {
			log.Fatalf("Failed to parse workflow file %s: %v", workflowPath, err)
		}
		workflowFiles = append(workflowFiles, workflowFile)
		allActionRefs = append(allActionRefs, workflowFile.UsesWithVersions...)
	case workflowsDir != "":
		workflowsDir = actioninfo.ExpandPath(workflowsDir, workDir)
		files, err := parser.FindWorkflowFilesInDir(workflowsDir)
		if err != nil {
			log.Fatalf("Failed to find workflow files in %s: %v", workflowsDir, err)
		}
		workflows, err := parser.ParseWorkflowFiles(files)
		if err != nil {
			log.Fatalf("Failed to parse workflow files: %v", err)
		}
		workflowFiles = workflows
		for _, wf := range workflows {
			allActionRefs = append(allActionRefs, wf.UsesWithVersions...)
		}
	default:
		actionRefs, workflows, err := parser.GetAllUsesFromRepoWithVersions(workDir)
		if err != nil {
			log.Fatalf("Failed to find workflow files: %v", err)
		}
		workflowFiles = workflows
		allActionRefs = actionRefs
	}

	return workflowFiles, allActionRefs
}
