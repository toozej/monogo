package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// notificationEnvVars lists every env var that belongs to NotificationConfig,
// used to ensure a clean slate before each sub-test.
var notificationEnvVars = []string{
	"GOTIFY_ENDPOINT", "GOTIFY_TOKEN",
	"SLACK_TOKEN", "SLACK_CHANNEL_ID",
	"TELEGRAM_TOKEN", "TELEGRAM_CHAT_ID",
	"DISCORD_TOKEN", "DISCORD_CHANNEL_ID",
	"PUSHOVER_TOKEN", "PUSHOVER_RECIPIENT_ID",
	"PUSHBULLET_TOKEN", "PUSHBULLET_DEVICE_NICKNAME",
	"NOTIFY_CONDENSE",
}

var allConfigEnvKeys = append([]string{
	"GH_TOKEN", "GITHUB_TOKEN", "CREATE_ISSUES",
	"NO_CACHE", "REFRESH_CACHE", "CACHE_TTL",
}, notificationEnvVars...)

func saveAndCleanEnv(t *testing.T) map[string]string {
	t.Helper()
	originalEnv := make(map[string]string, len(allConfigEnvKeys))
	for _, k := range allConfigEnvKeys {
		originalEnv[k] = os.Getenv(k)
	}
	for _, k := range allConfigEnvKeys {
		os.Unsetenv(k)
	}
	return originalEnv
}

func restoreEnv(originalEnv map[string]string) {
	for key, value := range originalEnv {
		if value != "" {
			os.Setenv(key, value)
		} else {
			os.Unsetenv(key)
		}
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name               string
		mockEnv            map[string]string
		mockEnvFile        string
		expectGitHubToken  string
		expectNotification NotificationConfig
		expectCreateIssues bool
		expectError        bool
	}{
		{
			name: "Valid GitHub token from GH_TOKEN",
			mockEnv: map[string]string{
				"GH_TOKEN": "gh_test_token",
			},
			expectGitHubToken:  "gh_test_token",
			expectNotification: NotificationConfig{},
			expectCreateIssues: false,
		},
		{
			name: "Valid GitHub token from GITHUB_TOKEN",
			mockEnv: map[string]string{
				"GITHUB_TOKEN": "github_test_token",
			},
			expectGitHubToken:  "github_test_token",
			expectNotification: NotificationConfig{},
			expectCreateIssues: false,
		},
		{
			name: "GH_TOKEN overrides GITHUB_TOKEN",
			mockEnv: map[string]string{
				"GH_TOKEN":     "gh_priority_token",
				"GITHUB_TOKEN": "github_lower_token",
			},
			expectGitHubToken:  "gh_priority_token",
			expectNotification: NotificationConfig{},
			expectCreateIssues: false,
		},
		{
			name: "Gotify notification config from env",
			mockEnv: map[string]string{
				"GOTIFY_ENDPOINT": "https://gotify.example.com",
				"GOTIFY_TOKEN":    "mytoken",
			},
			expectGitHubToken: "",
			expectNotification: NotificationConfig{
				GotifyEndpoint: "https://gotify.example.com",
				GotifyToken:    "mytoken",
			},
			expectCreateIssues: false,
		},
		{
			name: "Condense flag from env",
			mockEnv: map[string]string{
				"NOTIFY_CONDENSE": "true",
			},
			expectGitHubToken: "",
			expectNotification: NotificationConfig{
				Condense: true,
			},
			expectCreateIssues: false,
		},
		{
			name: "Create issues enabled",
			mockEnv: map[string]string{
				"CREATE_ISSUES": "true",
			},
			expectGitHubToken:  "",
			expectNotification: NotificationConfig{},
			expectCreateIssues: true,
		},
		{
			name:               "No environment variables or .env file",
			expectGitHubToken:  "",
			expectNotification: NotificationConfig{},
			expectCreateIssues: false,
		},
		{
			name: "Environment variables override .env file",
			mockEnv: map[string]string{
				"GH_TOKEN": "env_override_token",
			},
			mockEnvFile:        "GH_TOKEN=file_token\n",
			expectGitHubToken:  "env_override_token",
			expectNotification: NotificationConfig{},
			expectCreateIssues: false,
		},
		{
			name:              "Valid .env file with Slack config",
			mockEnvFile:       "GH_TOKEN=envfile_token\nSLACK_TOKEN=slack_tok\nSLACK_CHANNEL_ID=C999\nCREATE_ISSUES=true\n",
			expectGitHubToken: "envfile_token",
			expectNotification: NotificationConfig{
				SlackToken:     "slack_tok",
				SlackChannelID: "C999",
			},
			expectCreateIssues: true,
		},
		{
			name: "env.Parse error with invalid int for TELEGRAM_CHAT_ID",
			mockEnv: map[string]string{
				"TELEGRAM_CHAT_ID": "not-a-number",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}

			originalEnv := saveAndCleanEnv(t)
			defer restoreEnv(originalEnv)

			tmpDir := t.TempDir()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("Failed to restore original directory: %v", err)
				}
			}()

			if tt.mockEnvFile != "" {
				envPath := filepath.Join(tmpDir, ".env")
				if err := os.WriteFile(envPath, []byte(tt.mockEnvFile), 0644); err != nil {
					t.Fatalf("Failed to write mock .env file: %v", err)
				}
			}

			for key, value := range tt.mockEnv {
				os.Setenv(key, value)
			}

			conf, err := loadConfig()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if conf.GitHubToken != tt.expectGitHubToken {
				t.Errorf("expected GitHubToken %q, got %q", tt.expectGitHubToken, conf.GitHubToken)
			}
			if conf.Notification.GotifyEndpoint != tt.expectNotification.GotifyEndpoint {
				t.Errorf("expected GotifyEndpoint %q, got %q", tt.expectNotification.GotifyEndpoint, conf.Notification.GotifyEndpoint)
			}
			if conf.Notification.GotifyToken != tt.expectNotification.GotifyToken {
				t.Errorf("expected GotifyToken %q, got %q", tt.expectNotification.GotifyToken, conf.Notification.GotifyToken)
			}
			if conf.Notification.SlackToken != tt.expectNotification.SlackToken {
				t.Errorf("expected SlackToken %q, got %q", tt.expectNotification.SlackToken, conf.Notification.SlackToken)
			}
			if conf.Notification.SlackChannelID != tt.expectNotification.SlackChannelID {
				t.Errorf("expected SlackChannelID %q, got %q", tt.expectNotification.SlackChannelID, conf.Notification.SlackChannelID)
			}
			if conf.Notification.Condense != tt.expectNotification.Condense {
				t.Errorf("expected Condense %v, got %v", tt.expectNotification.Condense, conf.Notification.Condense)
			}
			if conf.CreateIssues != tt.expectCreateIssues {
				t.Errorf("expected CreateIssues %v, got %v", tt.expectCreateIssues, conf.CreateIssues)
			}
		})
	}
}

func TestLoadConfigPathTraversal(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		errorSubstr string
		setup       func(t *testing.T, tmpDir string)
	}{
		{
			name:        "path traversal detected when env file resolves outside cwd",
			expectError: true,
			errorSubstr: "path traversal detected",
			setup: func(t *testing.T, tmpDir string) {
				t.Helper()
				workDir := filepath.Join(tmpDir, "app")
				if err := os.MkdirAll(workDir, 0755); err != nil {
					t.Fatalf("Failed to create work directory: %v", err)
				}
				if err := os.Chdir(workDir); err != nil {
					t.Fatalf("Failed to chdir to work directory: %v", err)
				}
				originalFilepathAbs := filepathAbs
				filepathAbs = func(path string) (string, error) {
					if strings.HasSuffix(path, ".env") {
						return filepath.Join(tmpDir, "etc", ".env"), nil
					}
					return filepath.Join(tmpDir, "app"), nil
				}
				t.Cleanup(func() { filepathAbs = originalFilepathAbs })
			},
		},
		{
			name:        "filepathAbs error for env path returns error",
			expectError: true,
			errorSubstr: "error resolving .env file path",
			setup: func(t *testing.T, tmpDir string) {
				t.Helper()
				if err := os.Chdir(tmpDir); err != nil {
					t.Fatalf("Failed to chdir to tmpDir: %v", err)
				}
				originalFilepathAbs := filepathAbs
				filepathAbs = func(path string) (string, error) {
					if strings.HasSuffix(path, ".env") {
						return "", fmt.Errorf("simulated abs failure for env path")
					}
					return filepath.Abs(path)
				}
				t.Cleanup(func() { filepathAbs = originalFilepathAbs })
			},
		},
		{
			name:        "filepathAbs error for cwd returns error",
			expectError: true,
			errorSubstr: "error resolving current directory",
			setup: func(t *testing.T, tmpDir string) {
				t.Helper()
				if err := os.Chdir(tmpDir); err != nil {
					t.Fatalf("Failed to chdir to tmpDir: %v", err)
				}
				originalFilepathAbs := filepathAbs
				filepathAbs = func(path string) (string, error) {
					if strings.HasSuffix(path, ".env") {
						return filepath.Abs(path)
					}
					return "", fmt.Errorf("simulated abs failure for cwd")
				}
				t.Cleanup(func() { filepathAbs = originalFilepathAbs })
			},
		},
		{
			name:        "osGetwd error returns error",
			expectError: true,
			errorSubstr: "error getting current working directory",
			setup: func(t *testing.T, tmpDir string) {
				t.Helper()
				if err := os.Chdir(tmpDir); err != nil {
					t.Fatalf("Failed to chdir to tmpDir: %v", err)
				}
				originalOsGetwd := osGetwd
				osGetwd = func() (string, error) {
					return "", fmt.Errorf("simulated getwd failure")
				}
				t.Cleanup(func() { osGetwd = originalOsGetwd })
			},
		},
		{
			name:        "non-existent .env file is skipped without error",
			expectError: false,
			setup: func(t *testing.T, tmpDir string) {
				t.Helper()
				if err := os.Chdir(tmpDir); err != nil {
					t.Fatalf("Failed to chdir to tmpDir: %v", err)
				}
			},
		},
		{
			name:        "valid .env file in cwd loads successfully",
			expectError: false,
			setup: func(t *testing.T, tmpDir string) {
				t.Helper()
				if err := os.Chdir(tmpDir); err != nil {
					t.Fatalf("Failed to chdir to tmpDir: %v", err)
				}
				envPath := filepath.Join(tmpDir, ".env")
				content := "GH_TOKEN=valid_dotenv_token\n"
				if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to write .env file: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("Failed to restore original directory: %v", err)
				}
			}()

			originalEnv := saveAndCleanEnv(t)
			defer restoreEnv(originalEnv)

			tmpDir := t.TempDir()
			tt.setup(t, tmpDir)

			_, err = loadConfig()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorSubstr)
				} else if !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("expected error containing %q, got %q", tt.errorSubstr, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestLoadConfigUnreadableEnvFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: test requires non-root user for permission denial")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore original directory: %v", err)
		}
	}()

	originalEnv := saveAndCleanEnv(t)
	defer restoreEnv(originalEnv)

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir to tmpDir: %v", err)
	}

	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte("GH_TOKEN=unreadable_token\n"), 0644); err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}
	if err := os.Chmod(envPath, 0000); err != nil {
		t.Fatalf("Failed to chmod .env file: %v", err)
	}
	defer func() {
		_ = os.Chmod(envPath, 0644)
	}()

	_, err = loadConfig()
	if err == nil {
		t.Error("expected error for unreadable .env file, got nil")
	}
}

func TestLoadConfigNotificationFields(t *testing.T) {
	tests := []struct {
		name             string
		mockEnv          map[string]string
		expectConfig     Config
		skipNotification bool
	}{
		{
			name: "Telegram notification config",
			mockEnv: map[string]string{
				"TELEGRAM_TOKEN":   "tele_token",
				"TELEGRAM_CHAT_ID": "123456789",
			},
			expectConfig: Config{
				Notification: NotificationConfig{
					TelegramToken:  "tele_token",
					TelegramChatID: 123456789,
				},
			},
		},
		{
			name: "Discord notification config",
			mockEnv: map[string]string{
				"DISCORD_TOKEN":      "discord_token",
				"DISCORD_CHANNEL_ID": "C111222",
			},
			expectConfig: Config{
				Notification: NotificationConfig{
					DiscordToken:     "discord_token",
					DiscordChannelID: "C111222",
				},
			},
		},
		{
			name: "Pushover notification config",
			mockEnv: map[string]string{
				"PUSHOVER_TOKEN":        "po_token",
				"PUSHOVER_RECIPIENT_ID": "po_recip",
			},
			expectConfig: Config{
				Notification: NotificationConfig{
					PushoverToken:       "po_token",
					PushoverRecipientID: "po_recip",
				},
			},
		},
		{
			name: "Pushbullet notification config",
			mockEnv: map[string]string{
				"PUSHBULLET_TOKEN":           "pb_token",
				"PUSHBULLET_DEVICE_NICKNAME": "mydevice",
			},
			expectConfig: Config{
				Notification: NotificationConfig{
					PushbulletToken:          "pb_token",
					PushbulletDeviceNickname: "mydevice",
				},
			},
		},
		{
			name: "NoCache and RefreshCache flags",
			mockEnv: map[string]string{
				"NO_CACHE":      "true",
				"REFRESH_CACHE": "true",
			},
			expectConfig: Config{
				NoCache:      true,
				RefreshCache: true,
			},
			skipNotification: true,
		},
		{
			name: "CACHE_TTL custom duration",
			mockEnv: map[string]string{
				"CACHE_TTL": "12h",
			},
			expectConfig: Config{
				CacheTTL: 12 * time.Hour,
			},
			skipNotification: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}

			originalEnv := saveAndCleanEnv(t)
			defer func() {
				restoreEnv(originalEnv)
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("Failed to restore original directory: %v", err)
				}
			}()

			tmpDir := t.TempDir()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to chdir to tmpDir: %v", err)
			}

			for key, value := range tt.mockEnv {
				os.Setenv(key, value)
			}

			conf, err := loadConfig()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !tt.skipNotification {
				if conf.Notification != tt.expectConfig.Notification {
					t.Errorf("expected Notification %+v, got %+v", tt.expectConfig.Notification, conf.Notification)
				}
			}
			if tt.expectConfig.NoCache != conf.NoCache {
				t.Errorf("expected NoCache %v, got %v", tt.expectConfig.NoCache, conf.NoCache)
			}
			if tt.expectConfig.RefreshCache != conf.RefreshCache {
				t.Errorf("expected RefreshCache %v, got %v", tt.expectConfig.RefreshCache, conf.RefreshCache)
			}
			if tt.expectConfig.CacheTTL != 0 && tt.expectConfig.CacheTTL != conf.CacheTTL {
				t.Errorf("expected CacheTTL %v, got %v", tt.expectConfig.CacheTTL, conf.CacheTTL)
			}
		})
	}
}

func TestGetEnvVarsPathTraversalExit(t *testing.T) {
	exitCalled := false
	originalOsExit := osExit
	osExit = func(code int) {
		exitCalled = true
		if code != 1 {
			t.Errorf("Expected exit code 1, got %d", code)
		}
	}
	defer func() { osExit = originalOsExit }()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore original directory: %v", err)
		}
	}()

	originalEnv := saveAndCleanEnv(t)
	defer restoreEnv(originalEnv)

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "realwork")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("Failed to create work directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, ".env"), []byte("GH_TOKEN=leaked\n"), 0644); err != nil {
		t.Fatalf("Failed to write .env file: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to chdir to work directory: %v", err)
	}

	originalOsGetwd := osGetwd
	osGetwd = func() (string, error) {
		return filepath.Join(tmpDir, "app"), nil
	}
	defer func() { osGetwd = originalOsGetwd }()

	originalFilepathAbs := filepathAbs
	filepathAbs = func(path string) (string, error) {
		if strings.HasSuffix(path, ".env") {
			return filepath.Join(tmpDir, "etc", ".env"), nil
		}
		return filepath.Join(tmpDir, "app"), nil
	}
	defer func() { filepathAbs = originalFilepathAbs }()

	GetEnvVars()

	if !exitCalled {
		t.Error("Expected osExit to be called for path traversal")
	}
}

func TestGetEnvVars(t *testing.T) {
	originalEnv := saveAndCleanEnv(t)
	defer restoreEnv(originalEnv)

	os.Setenv("GH_TOKEN", "success_token")
	conf := GetEnvVars()
	if conf.GitHubToken != "success_token" {
		t.Errorf("Expected success_token, got %s", conf.GitHubToken)
	}

	os.Setenv("TELEGRAM_CHAT_ID", "not-a-number")

	exitCalled := false
	originalOsExit := osExit
	osExit = func(code int) {
		exitCalled = true
		if code != 1 {
			t.Errorf("Expected exit code 1, got %d", code)
		}
	}
	defer func() { osExit = originalOsExit }()

	GetEnvVars()

	if !exitCalled {
		t.Error("Expected osExit to be called")
	}
}
