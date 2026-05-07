package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/go-find-archived-gh-actions/internal/github"
	"github.com/toozej/go-find-archived-gh-actions/internal/issue"
	"github.com/toozej/go-find-archived-gh-actions/internal/notification"
	ver "github.com/toozej/go-find-archived-gh-actions/internal/version"
	"github.com/toozej/go-find-archived-gh-actions/internal/workflow"
	"github.com/toozej/go-find-archived-gh-actions/pkg/config"
	"github.com/toozej/go-find-archived-gh-actions/pkg/man"
	"github.com/toozej/go-find-archived-gh-actions/pkg/version"
)

var (
	conf          config.Config
	debug         bool
	verbose       bool
	workflowPath  string
	workflowsDir  string
	reposDir      string
	githubToken   string
	notify        bool
	createIssue   bool
	checkOutdated bool
	checkStale    bool
	staleDays     int
)

const defaultStaleDays = 365

var rootCmd = &cobra.Command{
	Use:   "go-find-archived-gh-actions",
	Short: "Detect archived GitHub Actions in repository workflows",
	Long: `A tool to detect if GitHub Actions used in repository workflows have been archived upstream.

The tool scans .github/workflows/**/*.yml and **/*.yaml files, extracts 'uses:' references,
checks the GitHub API for archived status, and reports findings.

Exit codes:
  0 - No archived actions found
  1 - Archived actions found or error occurred`,
	Args:             cobra.NoArgs,
	PersistentPreRun: rootCmdPreRun,
	Run:              rootCmdRun,
}

func rootCmdRun(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

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

	parser := workflow.NewParser()
	ghClient := github.NewClient(token)

	if debug {
		ghClient.LogRateLimits(ctx)
	}

	var notifier *notification.NotificationManager
	var issueCreator *issue.IssueCreator

	if notify {
		manager, err := notification.NewNotificationManager(conf.Notification)
		if err != nil {
			log.Fatalf("Failed to initialize notification manager: %v", err)
		}
		notifier = manager
	}

	if createIssue {
		issueCreator = issue.NewIssueCreator(token)
	}

	if reposDir != "" {
		reposDir = expandPath(reposDir, workDir)
		runReposMode(ctx, parser, ghClient, notifier, issueCreator, reposDir, workDir)
		return
	}

	var workflowFiles []*workflow.WorkflowFile
	var allActionRefs []workflow.ActionRef

	switch {
	case workflowPath != "":
		workflowPath = expandPath(workflowPath, workDir)
		workflowFile, err := parser.ParseWorkflowFile(workflowPath)
		if err != nil {
			log.Fatalf("Failed to parse workflow file %s: %v", workflowPath, err)
		}
		workflowFiles = append(workflowFiles, workflowFile)
		allActionRefs = append(allActionRefs, workflowFile.UsesWithVersions...)
	case workflowsDir != "":
		workflowsDir = expandPath(workflowsDir, workDir)
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

	processWorkflows(ctx, parser, ghClient, notifier, issueCreator, workflowFiles, allActionRefs, workDir)
}

func runReposMode(ctx context.Context, parser *workflow.WorkflowParser, ghClient *github.Client, notifier *notification.NotificationManager, issueCreator *issue.IssueCreator, reposDir, workDir string) {
	repos, err := parser.FindReposWithWorkflows(reposDir)
	if err != nil {
		log.Fatalf("Failed to find repos with workflows: %v", err)
	}

	if len(repos) == 0 {
		fmt.Println("No repositories with .github/workflows found in the specified directory")
		return
	}

	if verbose {
		fmt.Printf("Found %d repositories with workflow files\n", len(repos))
	}

	hasAnyIssues := false
	for _, repoPath := range repos {
		fmt.Printf("\n📁 Scanning: %s\n", repoPath)
		fmt.Println(strings.Repeat("-", len(repoPath)+10))

		actionRefs, workflows, err := parser.GetAllUsesFromRepoWithVersions(repoPath)
		if err != nil {
			log.Errorf("Failed to find workflow files in %s: %v", repoPath, err)
			continue
		}

		if len(actionRefs) == 0 {
			fmt.Println("No GitHub Actions found in workflows")
			continue
		}

		hasIssues := processWorkflows(ctx, parser, ghClient, notifier, issueCreator, workflows, actionRefs, repoPath)
		if hasIssues {
			hasAnyIssues = true
		}
	}

	if hasAnyIssues {
		os.Exit(1)
	}
}

func processWorkflows(ctx context.Context, parser *workflow.WorkflowParser, ghClient *github.Client, notifier *notification.NotificationManager, issueCreator *issue.IssueCreator, workflowFiles []*workflow.WorkflowFile, allActionRefs []workflow.ActionRef, workDir string) bool {
	if verbose {
		fmt.Printf("Found %d workflow files\n", len(workflowFiles))
		for _, wf := range workflowFiles {
			fmt.Printf(" - %s (%d uses)\n", wf.Path, len(wf.UsesWithVersions))
		}
		fmt.Printf("Extracted %d unique action references\n", len(allActionRefs))
		if len(allActionRefs) > 0 {
			for _, ref := range allActionRefs {
				fmt.Printf(" - %s@%s\n", ref.OwnerRepo, ref.Version)
			}
		}
	}

	if len(allActionRefs) == 0 {
		fmt.Println("No GitHub Actions found in workflows")
		return false
	}

	ownerRepos := make([]string, 0, len(allActionRefs))
	seen := make(map[string]bool)
	for _, ref := range allActionRefs {
		if !seen[ref.OwnerRepo] {
			seen[ref.OwnerRepo] = true
			ownerRepos = append(ownerRepos, ref.OwnerRepo)
		}
	}

	fmt.Printf("Checking %d action repositories for archived status...\n", len(ownerRepos))

	archived, errors := ghClient.CheckMultipleRepos(ctx, ownerRepos)

	if debug {
		ghClient.LogRateLimits(ctx)
	}

	if verbose && len(errors) > 0 {
		fmt.Printf("API errors encountered:\n")
		for repo, err := range errors {
			fmt.Printf(" - %s: %v\n", repo, err)
		}
	}

	var archivedActions []issue.ArchivedActionInfo
	var archivedRepos []string

	for _, wf := range workflowFiles {
		for _, ref := range wf.UsesWithVersions {
			if isArchived, exists := archived[ref.OwnerRepo]; exists && isArchived {
				archivedActions = append(archivedActions, issue.ArchivedActionInfo{
					Repo:     ref.OwnerRepo,
					Workflow: filepath.Base(wf.Path),
					Uses:     ref.FullRef,
				})
				archivedRepos = append(archivedRepos, ref.OwnerRepo)
			}
		}
	}

	archivedRepos = removeDuplicates(archivedRepos)

	var outdatedActions []OutdatedActionInfo
	if checkOutdated {
		var nonArchivedRepos []string
		for _, ref := range allActionRefs {
			if isArchived, exists := archived[ref.OwnerRepo]; !exists || !isArchived {
				nonArchivedRepos = append(nonArchivedRepos, ref.OwnerRepo)
			}
		}
		nonArchivedRepos = removeDuplicates(nonArchivedRepos)

		if len(nonArchivedRepos) > 0 {
			fmt.Printf("Checking %d non-archived action repositories for latest versions...\n", len(nonArchivedRepos))
			releases, releaseErrors := ghClient.CheckMultipleReleases(ctx, nonArchivedRepos)

			if debug {
				ghClient.LogRateLimits(ctx)
			}

			if verbose && len(releaseErrors) > 0 {
				fmt.Printf("Release API errors encountered:\n")
				for repo, err := range releaseErrors {
					fmt.Printf(" - %s: %v\n", repo, err)
				}
			}

			outdatedActions = checkOutdatedActions(ctx, ghClient, workflowFiles, archived, releases)
		}
	}

	var staleActions []StaleActionInfo
	if checkStale {
		var nonArchivedRepos []string
		for _, ref := range allActionRefs {
			if isArchived, exists := archived[ref.OwnerRepo]; !exists || !isArchived {
				nonArchivedRepos = append(nonArchivedRepos, ref.OwnerRepo)
			}
		}
		nonArchivedRepos = removeDuplicates(nonArchivedRepos)

		if len(nonArchivedRepos) > 0 {
			days := staleDays
			if days <= 0 {
				days = defaultStaleDays
			}
			staleThreshold := time.Duration(days) * 24 * time.Hour
			fmt.Printf("Checking %d non-archived action repositories for stale/deprecated status...\n", len(nonArchivedRepos))
			staleResults, staleErrors := ghClient.CheckMultipleStale(ctx, nonArchivedRepos, staleThreshold)

			if verbose && len(staleErrors) > 0 {
				fmt.Printf("Stale check errors encountered:\n")
				for repo, err := range staleErrors {
					fmt.Printf(" - %s: %v\n", repo, err)
				}
			}

			for _, wf := range workflowFiles {
				for _, ref := range wf.UsesWithVersions {
					if staleInfo, exists := staleResults[ref.OwnerRepo]; exists {
						staleActions = append(staleActions, StaleActionInfo{
							OwnerRepo:          ref.OwnerRepo,
							FullRef:            ref.FullRef,
							Workflow:           filepath.Base(wf.Path),
							Deprecated:         staleInfo.Deprecated,
							DeprecationMessage: staleInfo.DeprecationMessage,
							LastUpdated:        staleInfo.LastUpdated,
							StaleByAge:         staleInfo.StaleByAge,
						})
					}
				}
			}
		}
	}

	hasIssues := len(archivedActions) > 0 || len(outdatedActions) > 0 || len(staleActions) > 0

	if !hasIssues {
		fmt.Println("✅ No archived, outdated, or stale GitHub Actions found!")
		return false
	}

	if len(archivedActions) > 0 {
		fmt.Printf("\n🚨 Found %d archived GitHub Actions in %d workflows:\n\n", len(archivedRepos), len(archivedActions))

		workflowMap := make(map[string][]issue.ArchivedActionInfo)
		for _, action := range archivedActions {
			workflowMap[action.Workflow] = append(workflowMap[action.Workflow], action)
		}

		for wf, actions := range workflowMap {
			fmt.Printf("📄 %s:\n", wf)
			for _, action := range actions {
				fmt.Printf(" ❌ %s\n", action.Uses)
			}
			fmt.Println()
		}
	}

	if len(outdatedActions) > 0 {
		uniqueOutdated := make(map[string]bool)
		for _, action := range outdatedActions {
			uniqueOutdated[action.OwnerRepo] = true
		}

		fmt.Printf("\n⚠️ Found %d outdated GitHub Actions in %d uses:\n\n", len(uniqueOutdated), len(outdatedActions))

		outdatedWorkflowMap := make(map[string][]OutdatedActionInfo)
		for _, action := range outdatedActions {
			outdatedWorkflowMap[action.Workflow] = append(outdatedWorkflowMap[action.Workflow], action)
		}

		for wf, actions := range outdatedWorkflowMap {
			fmt.Printf("📄 %s:\n", wf)
			for _, action := range actions {
				fmt.Printf(" ⚠️ %s@%s (latest: %s)\n", action.OwnerRepo, action.CurrentRef, action.LatestTag)
			}
			fmt.Println()
		}
	}

	if len(staleActions) > 0 {
		uniqueStale := make(map[string]bool)
		for _, action := range staleActions {
			uniqueStale[action.OwnerRepo] = true
		}

		fmt.Printf("\n⏳ Found %d stale/deprecated GitHub Actions in %d uses:\n\n", len(uniqueStale), len(staleActions))

		staleWorkflowMap := make(map[string][]StaleActionInfo)
		for _, action := range staleActions {
			staleWorkflowMap[action.Workflow] = append(staleWorkflowMap[action.Workflow], action)
		}

		for wf, actions := range staleWorkflowMap {
			fmt.Printf("📄 %s:\n", wf)
			for _, action := range actions {
				if action.Deprecated {
					msg := action.DeprecationMessage
					if msg != "" {
						msg = ": " + msg
					}
					fmt.Printf(" ⏳ %s (DEPRECATED%s)\n", action.FullRef, msg)
				} else if action.StaleByAge {
					fmt.Printf(" ⏳ %s (not updated since %s)\n", action.FullRef, action.LastUpdated.Format("2006-01-02"))
				}
			}
			fmt.Println()
		}
	}

	repoName := getRepoName(workDir)

	if notifier != nil && len(archivedActions) > 0 {
		var notificationActions []notification.ArchivedActionInfo
		for _, action := range archivedActions {
			notificationActions = append(notificationActions, notification.ArchivedActionInfo{
				Repo:     action.Repo,
				Workflow: action.Workflow,
				Uses:     action.Uses,
			})
		}
		if err := notifier.NotifyArchivedActions(ctx, notificationActions, repoName); err != nil {
			log.Errorf("Failed to send notifications: %v", err)
		}
	}

	if issueCreator != nil && repoName != "" && len(archivedActions) > 0 {
		parts := strings.Split(repoName, "/")
		if len(parts) == 2 {
			owner, repo := parts[0], parts[1]
			if err := issueCreator.CreateArchivedActionIssue(ctx, owner, repo, archivedActions); err != nil {
				log.Errorf("Failed to create GitHub issue: %v", err)
			}
		}
	}

	switch {
	case len(archivedActions) > 0:
		fmt.Println("\n❌ Archived actions detected. Please replace them with actively maintained alternatives.")
		return true
	case len(outdatedActions) > 0:
		fmt.Println("\n⚠️ Outdated actions detected. Consider updating to the latest versions.")
		return true
	case len(staleActions) > 0:
		fmt.Println("\n⏳ Stale or deprecated actions detected. Consider replacing them with actively maintained alternatives.")
		return true
	}

	return false
}

func checkOutdatedActions(ctx context.Context, ghClient *github.Client, workflowFiles []*workflow.WorkflowFile, archived map[string]bool, releases map[string]*github.ReleaseInfo) []OutdatedActionInfo {
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

			cacheKey := ref.OwnerRepo + "@" + ref.Version + ":" + release.TagName
			if seenOutdated[cacheKey] {
				continue
			}

			if ver.IsMajorVersionTag(ref.Version) {
				if ver.SameMajorVersion(ref.Version, release.TagName) {
					same, _, _, err := ghClient.CompareRefSHAs(ctx, ref.OwnerRepo, ref.Version, release.TagName)
					if err != nil {
						if verbose {
							fmt.Printf("  Cannot compare SHAs for %s@%s vs %s: %v\n", ref.OwnerRepo, ref.Version, release.TagName, err)
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
						CurrentRef: ref.Version,
						LatestTag:  release.TagName,
						LatestURL:  release.HTMLURL,
						Workflow:   filepath.Base(wf.Path),
						FullRef:    ref.FullRef,
					})
					seenOutdated[cacheKey] = true
					continue
				}
			}

			isOutdated, err := ver.IsVersionOutdated(ref.Version, release.TagName)
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
					CurrentRef: ref.Version,
					LatestTag:  release.TagName,
					LatestURL:  release.HTMLURL,
					Workflow:   filepath.Base(wf.Path),
					FullRef:    ref.FullRef,
				})
			}
			seenOutdated[cacheKey] = true
		}
	}

	return outdatedActions
}

type OutdatedActionInfo struct {
	OwnerRepo  string
	CurrentRef string
	LatestTag  string
	LatestURL  string
	Workflow   string
	FullRef    string
}

type StaleActionInfo struct {
	OwnerRepo          string
	FullRef            string
	Workflow           string
	Deprecated         bool
	DeprecationMessage string
	LastUpdated        time.Time
	StaleByAge         bool
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

	rootCmd.Flags().StringVarP(&workflowPath, "workflow", "w", "", "Path to specific workflow file to check")
	rootCmd.Flags().StringVar(&workflowsDir, "workflows-dir", "", "Path to directory containing workflow yaml files")
	rootCmd.Flags().StringVar(&reposDir, "repos-dir", "", "Path to base directory containing multiple repos to scan")
	rootCmd.Flags().StringVarP(&githubToken, "token", "t", "", "GitHub token (overrides GH_TOKEN/GITHUB_TOKEN env vars)")
	rootCmd.Flags().BoolVar(&notify, "notify", false, "Send notifications to configured endpoints")
	rootCmd.Flags().BoolVar(&createIssue, "create-issue", false, "Create GitHub issue when archived actions found")
	rootCmd.Flags().BoolVar(&checkOutdated, "check-outdated", false, "Check for outdated action versions")
	rootCmd.Flags().BoolVar(&checkStale, "stale", false, "Check for stale/deprecated actions (not updated in over a year or marked with deprecation warning)")
	rootCmd.Flags().IntVar(&staleDays, "stale-days", defaultStaleDays, "Number of days after which an action is considered stale (default 365)")

	rootCmd.AddCommand(
		man.NewManCmd(),
		version.Command(),
	)
}

func expandPath(path, workDir string) string {
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

func removeDuplicates(slice []string) []string {
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

func getRepoName(workDir string) string {
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		return repo
	}
	return "current-repo"
}
