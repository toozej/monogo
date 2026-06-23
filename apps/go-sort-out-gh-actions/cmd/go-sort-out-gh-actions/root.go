package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/actioninfo"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/checkrunner"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/issue"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/notification"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/monogo/apps/go-sort-out-gh-actions/internal/workflow"
	"github.com/toozej/monogo/pkg/go-sort-out-gh-actions/config"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
)

var (
	conf         config.Config
	debug        bool
	verbose      bool
	workflowPath string
	workflowsDir string
	reposDir     string
	remoteRepo   string
	remoteRef    string
	orgName      string
	userName     string
	includeForks bool
	githubToken  string
	notify       bool
	createIssue  bool
	outputFormat string
	noCache      bool
	refreshCache bool
	cacheTTL     time.Duration

	csvAdditionalData string

	ghAPIBaseURL  string
	ghAPIClient   *http.Client
	eolAPIBaseURL string
	eolAPIClient  *http.Client
)

var rootCmd = &cobra.Command{
	Use:   "go-sort-out-gh-actions",
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
	if exitCode != 0 {
		os.Exit(exitCode)
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
	rootCmd.PersistentFlags().StringVar(&remoteRepo, "remote", "", "GitHub repository to scan remotely (e.g. owner/repo)")
	rootCmd.PersistentFlags().StringVar(&remoteRef, "ref", "", "Git ref (branch/tag/SHA) for remote repo scan (default: default branch)")
	rootCmd.PersistentFlags().StringVar(&orgName, "org", "", "GitHub organization to scan (scans all repos in the org)")
	rootCmd.PersistentFlags().StringVar(&userName, "user", "", "GitHub user to scan (scans all user repos)")
	rootCmd.PersistentFlags().BoolVar(&includeForks, "include-forks", false, "Include forked repositories when scanning org or user repos")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text, json, or csv")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", conf.NoCache, "Disable reading and writing cache files")
	rootCmd.PersistentFlags().BoolVar(&refreshCache, "refresh-cache", conf.RefreshCache, "Ignore existing cache and overwrite after run")
	rootCmd.PersistentFlags().DurationVar(&cacheTTL, "cache-ttl", conf.CacheTTL, "How long cache files remain valid (e.g. 24h)")
	rootCmd.PersistentFlags().StringVar(&csvAdditionalData, "csv-additional-data", "", "Additional CSV columns as key=value pairs, comma-separated (e.g., Project=PROJ,Assignee=user)")

	rootCmd.AddCommand(
		newArchivedCmd(),
		newEOLCmd(),
		newOutdatedCmd(),
		newPinCmd(),
		newCheckCmd(),
		newMCPCmd(),
		newAvatarCmd(),
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

func resolveOutputFormat() output.Format {
	f, err := output.ParseFormat(outputFormat)
	if err != nil {
		log.Fatal(err.Error())
	}
	return f
}

func resolveCSVConfig() *output.CSVConfig {
	if outputFormat != "csv" || csvAdditionalData == "" {
		return nil
	}
	extraColumns := make(map[string]string)
	for _, pair := range strings.Split(csvAdditionalData, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" {
			log.Fatalf("Invalid --csv-additional-data pair %q: expected format key=value", pair)
		}
		extraColumns[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	if len(extraColumns) == 0 {
		return nil
	}
	return &output.CSVConfig{
		ExtraColumns: extraColumns,
	}
}

func newOutputWriter(of output.Format) *output.Writer {
	return output.NewWriterWithOptionalCSV(of, resolveCSVConfig())
}

func newRunContextFromFlags(token string, of output.Format) *checkrunner.RunContext {
	if ghAPIBaseURL != "" && ghAPIClient != nil {
		ghClient := github.NewClientWithHTTP(ghAPIBaseURL, ghAPIClient, github.WithCache(!noCache, refreshCache, cacheTTL))
		if eolAPIBaseURL != "" && eolAPIClient != nil {
			ghClient.SetEOLClientForTest(eolAPIBaseURL, eolAPIClient)
		}
		workDir, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get working directory: %v", err)
		}
		rc := &checkrunner.RunContext{
			Ctx:          context.Background(),
			WorkDir:      workDir,
			Parser:       workflow.NewParser(),
			GHClient:     ghClient,
			OutputWriter: newOutputWriter(of),
		}
		if notify {
			manager, nerr := notification.NewNotificationManager(conf.Notification)
			if nerr != nil {
				log.Fatalf("Failed to initialize notification manager: %v", nerr)
			}
			rc.Notifier = manager
		}
		if createIssue {
			rc.IssueCreator = issue.NewIssueCreator(token)
		}
		return rc
	}
	return checkrunner.NewRunContext(token, conf, notify, createIssue, of, resolveCSVConfig(), noCache, refreshCache, cacheTTL)
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
