package checkrunner

import (
	"context"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/toozej/go-sort-out-gh-actions/internal/github"
	"github.com/toozej/go-sort-out-gh-actions/internal/issue"
	"github.com/toozej/go-sort-out-gh-actions/internal/notification"
	"github.com/toozej/go-sort-out-gh-actions/internal/output"
	"github.com/toozej/go-sort-out-gh-actions/internal/workflow"
	"github.com/toozej/go-sort-out-gh-actions/pkg/config"
)

func NewRunContext(token string, conf config.Config, initNotifier, initIssueCreator bool, outputFormat output.Format, noCache, refreshCache bool, cacheTTL time.Duration) *RunContext {
	ctx := context.Background()
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	parser := workflow.NewParser()

	cacheEnabled := !noCache
	ghClient := github.NewClient(token, github.WithCache(cacheEnabled, refreshCache, cacheTTL))

	rc := &RunContext{
		Ctx:          ctx,
		WorkDir:      workDir,
		Parser:       parser,
		GHClient:     ghClient,
		OutputWriter: output.NewWriter(outputFormat),
	}

	if initNotifier {
		manager, nerr := notification.NewNotificationManager(conf.Notification)
		if nerr != nil {
			log.Fatalf("Failed to initialize notification manager: %v", nerr)
		}
		rc.Notifier = manager
	}

	if initIssueCreator {
		rc.IssueCreator = issue.NewIssueCreator(token)
	}

	return rc
}

// Close flushes any persistent caches and releases resources.
func (rc *RunContext) Close() error {
	if rc.GHClient != nil {
		return rc.GHClient.Close()
	}
	return nil
}
