// Package cmd defines the gocicle command line.
// @title Gocicle API
// @version 1.0
// @description Manage projects, immutable jobs, distributed runners, and run history.
// @BasePath /api/v1
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	_ "github.com/toozej/monogo/apps/gocicle/docs"
	"github.com/toozej/monogo/apps/gocicle/internal/config"
	"github.com/toozej/monogo/apps/gocicle/internal/jobs"
	"github.com/toozej/monogo/apps/gocicle/internal/providers"
	"github.com/toozej/monogo/apps/gocicle/internal/runner"
	containerruntime "github.com/toozej/monogo/apps/gocicle/internal/runtime"
	"github.com/toozej/monogo/apps/gocicle/internal/security"
	"github.com/toozej/monogo/apps/gocicle/internal/server"
	"github.com/toozej/monogo/apps/gocicle/internal/service"
	"github.com/toozej/monogo/apps/gocicle/internal/storage"
	"github.com/toozej/monogo/apps/gocicle/internal/translate"
	"github.com/toozej/monogo/pkg/man"
	"github.com/toozej/monogo/pkg/version"
	"gorm.io/gorm"
)

func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := NewCommand().ExecuteContext(ctx)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func NewCommand() *cobra.Command {
	root := &cobra.Command{Use: "gocicle", Short: "Schedule container jobs across distributed runners", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(version.Command(), man.NewManCmd())
	root.AddCommand(&cobra.Command{Use: "serve", Short: "Run the control plane", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := cfg.ValidateServer(); err != nil {
			return err
		}
		svc, closeDB, err := open(cfg, true)
		if err != nil {
			return err
		}
		defer closeDB()
		if err := storage.Check(svc.DB); err != nil {
			return err
		}
		return server.New(cfg, svc).Run(cmd.Context())
	}})
	root.AddCommand(&cobra.Command{Use: "migrate", Short: "Apply ordered PostgreSQL migrations", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, closeDB, err := open(cfg, false)
		if err != nil {
			return err
		}
		defer closeDB()
		return storage.Migrate(cmd.Context(), svc.DB)
	}})
	root.AddCommand(&cobra.Command{Use: "bootstrap", Short: "Create the first administrator invitation", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, closeDB, err := open(cfg, true)
		if err != nil {
			return err
		}
		defer closeDB()
		token, err := svc.Bootstrap(cmd.Context())
		if err == nil {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), token)
		}
		return err
	}})
	root.AddCommand(&cobra.Command{Use: "rotate-keys", Short: "Re-encrypt secrets with the active key", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, closeDB, err := open(cfg, true)
		if err != nil {
			return err
		}
		defer closeDB()
		return svc.RotateKeys(cmd.Context())
	}})
	var keyOutput string
	keygen := &cobra.Command{Use: "keygen", Short: "Create an external encryption key file", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if keyOutput == "" {
			return errors.New("--output is required")
		}
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return err
		}
		ring := security.Keyring{Active: "v1", Keys: map[string]string{"v1": base64.StdEncoding.EncodeToString(key)}}
		b, _ := json.MarshalIndent(ring, "", "  ")
		f, err := os.OpenFile(keyOutput, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600) // #nosec G304 -- The operator selects a new key file. O_EXCL prevents overwrites.
		if err != nil {
			return err
		}
		_, err = f.Write(append(b, '\n'))
		return errors.Join(err, f.Close())
	}}
	keygen.Flags().StringVar(&keyOutput, "output", "", "Path for the new key file")
	root.AddCommand(keygen)
	root.AddCommand(providerCommand(), runnerCommand(), jobsCommand(), translateCommand(), apiCommand())
	return root
}
func open(cfg config.Config, keys bool) (*service.Service, func(), error) {
	if cfg.DatabaseURL == "" {
		return nil, nil, errors.New("GOCICLE_DATABASE_URL is required")
	}
	var ring security.Keyring
	var err error
	if keys {
		ring, err = security.Load(cfg.KeyFile)
		if err != nil {
			return nil, nil, err
		}
	}
	db, err := storage.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	return service.New(db, ring), func() { _ = sqlDB.Close() }, nil
}
func providerCommand() *cobra.Command {
	return &cobra.Command{Use: "provider-add FILE", Short: "Configure a login provider through local database administration", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		var input struct {
			Name, Kind, ClientSecret string
			Config                   providers.Config
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &input); err != nil {
			return err
		}
		if _, err := providers.New(input.Kind, input.Config, input.ClientSecret); err != nil {
			return err
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		svc, closeDB, err := open(cfg, true)
		if err != nil {
			return err
		}
		defer closeDB()
		return svc.DB.Transaction(func(tx *gorm.DB) error {
			system := storage.User{ID: "gocicle-system", Name: "System", Preferences: storage.Encode(map[string]any{}), Version: 1}
			if err := tx.Exec("INSERT INTO users(id,name) VALUES (?,?) ON CONFLICT DO NOTHING", system.ID, system.Name).Error; err != nil {
				return err
			}
			if input.ClientSecret != "" {
				id := storage.ID()
				encrypted, err := svc.Keys.Encrypt(id, storage.Encode(map[string]string{"value": input.ClientSecret}))
				if err != nil {
					return err
				}
				secret := storage.Secret{ID: id, OwnerID: system.ID, Name: input.Name + " OAuth", Kind: "oauth", Encrypted: storage.Encode(encrypted), Version: 1}
				if err := tx.Create(&secret).Error; err != nil {
					return err
				}
				input.Config.SecretID = id
			}
			p := storage.Provider{ID: storage.ID(), Name: input.Name, Kind: input.Kind, Config: storage.Encode(input.Config), Version: 1}
			if err := tx.Create(&p).Error; err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), p.ID)
			return err
		})
	}}
}
func runnerCommand() *cobra.Command {
	var enrollment string
	command := &cobra.Command{Use: "runner", Short: "Run assigned jobs on a Linux host", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		client, err := runner.NewClient(cfg.APIURL, "")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(cfg.StateDir, 0700); err != nil {
			return err
		}
		path := filepath.Join(cfg.StateDir, "credential.json")
		if enrollment != "" {
			if _, err := os.Stat(path); err == nil {
				return errors.New("runner is already enrolled")
			}
			var reply struct {
				Runner storage.Runner
				Token  string
			}
			if err := client.Call(cmd.Context(), "POST", "/api/v1/runner/enroll", "", map[string]string{"token": enrollment}, &reply); err != nil {
				return err
			}
			if err := runner.SaveEnrollment(path, reply.Runner.ID, reply.Token); err != nil {
				return err
			}
		}
		b, err := os.ReadFile(path) // #nosec G304 -- The operator configures the private runner state directory.
		if err != nil {
			return errors.New("enroll this runner with --enroll TOKEN")
		}
		var credentials struct{ ID, Token string }
		if err := json.Unmarshal(b, &credentials); err != nil {
			return err
		}
		client.Token = credentials.Token
		runtime, err := containerruntime.New(cfg.Runtime, cfg.Socket)
		if err != nil {
			return err
		}
		worker := runner.Runner{Client: client, Runtime: runtime, ID: credentials.ID, StateDir: cfg.StateDir}
		return worker.Run(cmd.Context())
	}}
	command.Flags().StringVar(&enrollment, "enroll", "", "Single-use runner enrollment token")
	return command
}
func apiRequest(ctx context.Context, method, path string, input any, revision, key string) ([]byte, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	client, err := runner.NewClient(cfg.APIURL, cfg.Token)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if input != nil {
		b, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		body = strings.NewReader(string(b))
	}
	req, err := http.NewRequestWithContext(ctx, method, client.URL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", revision)
	if key == "" {
		key = security.Token()
	}
	req.Header.Set("Idempotency-Key", key)
	response, err := client.HTTP.Do(req)
	if err != nil {
		return nil, errors.New("API request failed")
	}
	defer func() { _ = response.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}
func apiCommand() *cobra.Command {
	var file, rev, key string
	cmd := &cobra.Command{Use: "api METHOD PATH", Short: "Call a versioned API endpoint", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if !strings.HasPrefix(args[1], "/api/v1/") {
			return errors.New("path must start with /api/v1/")
		}
		var body any
		if file != "" {
			b, err := os.ReadFile(file) // #nosec G304 -- The CLI reads the file explicitly selected by the operator.
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, &body); err != nil {
				return err
			}
		}
		out, err := apiRequest(cmd.Context(), args[0], args[1], body, rev, key)
		if err == nil {
			_, err = cmd.OutOrStdout().Write(out)
		}
		return err
	}}
	cmd.Flags().StringVar(&file, "data", "", "JSON request file")
	cmd.Flags().StringVar(&rev, "revision", "", "Current record version")
	cmd.Flags().StringVar(&key, "key", "", "Idempotency key")
	return cmd
}
func jobsCommand() *cobra.Command {
	root := &cobra.Command{Use: "jobs", Short: "Manage jobs through the API"}
	root.AddCommand(&cobra.Command{Use: "validate FILE", Short: "Validate a portable job file", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		b, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		d, err := jobs.Parse(b)
		if err == nil {
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Validated %d jobs.\n", len(d.Jobs))
		}
		return err
	}})
	for _, operation := range []string{"list", "export", "run", "cancel", "get"} {
		op := operation
		root.AddCommand(&cobra.Command{Use: op + " ID", Short: strings.ToUpper(op[:1]) + op[1:] + " jobs or runs", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/projects/" + args[0] + "/jobs"
			method := "GET"
			switch op {
			case "export":
				path += "/export"
			case "run":
				path = "/api/v1/jobs/" + args[0] + "/runs"
				method = "POST"
			case "cancel":
				path = "/api/v1/runs/" + args[0] + "/cancel"
				method = "POST"
			case "get":
				path = "/api/v1/jobs/" + args[0]
			}
			out, err := apiRequest(cmd.Context(), method, path, nil, "", "")
			if err == nil {
				_, err = cmd.OutOrStdout().Write(out)
			}
			return err
		}})
	}
	var mapping, key string
	importCmd := &cobra.Command{Use: "import PROJECT_ID FILE", Short: "Import validated jobs in a disabled state", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		b, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		d, err := jobs.Parse(b)
		if err != nil {
			return err
		}
		input := service.Import{Document: d}
		if mapping != "" {
			b, err := os.ReadFile(mapping) // #nosec G304 -- The CLI reads the mapping file explicitly selected by the operator.
			if err != nil {
				return err
			}
			var m struct {
				Credentials map[string]string
				Versions    map[string]int64
			}
			if err := json.Unmarshal(b, &m); err != nil {
				return err
			}
			input.Credentials = m.Credentials
			input.Versions = m.Versions
		}
		out, err := apiRequest(cmd.Context(), "POST", "/api/v1/projects/"+args[0]+"/jobs/import", input, "", key)
		if err == nil {
			_, err = cmd.OutOrStdout().Write(out)
		}
		return err
	}}
	importCmd.Flags().StringVar(&mapping, "mapping", "", "Credential mappings and current job versions as JSON")
	importCmd.Flags().StringVar(&key, "key", "", "Idempotency key")
	root.AddCommand(importCmd)
	return root
}
func translateCommand() *cobra.Command {
	var output, source, branch, image, labelMap, credentialMap string
	var draft bool
	cmd := &cobra.Command{Use: "translate gha|jenkins FILE", Short: "Translate a supported workflow without executing it", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		b, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		o := translate.Options{Project: jobs.Project{Name: "translated", Source: jobs.Source{Git: &jobs.Git{URL: source, Branch: branch}}}, Image: image, Draft: draft}
		if err := json.Unmarshal([]byte(labelMap), &o.Labels); err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(credentialMap), &o.Credentials); err != nil {
			return err
		}
		result, conversionErr := translate.Convert(args[0], args[1], b, o)
		for _, d := range result.Diagnostics {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), d.String())
		}
		if conversionErr != nil && !draft {
			return conversionErr
		}
		if output == "" {
			return errors.New("--output is required for YAML and generated scripts")
		}
		for name, script := range result.Scripts {
			if strings.Contains(name, "..") || filepath.IsAbs(name) {
				return errors.New("translated script path is invalid")
			}
			path := filepath.Join(output, name)
			if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
				return err
			}
			if err := writeNew(path, []byte(script), 0755); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(output, 0750); err != nil {
			return err
		}
		document, err := result.Document.YAML()
		if err != nil {
			return err
		}
		if err := writeNew(filepath.Join(output, ".gocicle.yaml"), document, 0644); err != nil {
			return err
		}
		return conversionErr
	}}
	cmd.Flags().StringVar(&output, "output", "", "New output directory")
	cmd.Flags().StringVar(&source, "source", "", "Project HTTPS or SSH clone URL")
	cmd.Flags().StringVar(&branch, "branch", "main", "Source branch")
	cmd.Flags().StringVar(&image, "image", "", "Explicit fallback image")
	cmd.Flags().StringVar(&labelMap, "labels", "{}", "Runner label mappings as JSON")
	cmd.Flags().StringVar(&credentialMap, "credentials", "{}", "Credential mappings as JSON")
	cmd.Flags().BoolVar(&draft, "draft", false, "Write disabled jobs and return diagnostics for incomplete conversion")
	return cmd
}
func writeNew(path string, b []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode) // #nosec G304 -- The operator selects the output directory. Generated paths are validated and cannot overwrite files.
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return errors.Join(err, f.Close())
}
