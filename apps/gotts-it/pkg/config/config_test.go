package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetEnvVars(t *testing.T) {
	tests := []struct {
		name                      string
		mockEnv                   map[string]string
		mockEnvFile               string
		expectError               bool
		expectURL                 string
		expectFile                string
		expectOutput              string
		expectBaseURL             string
		expectToken               string
		expectModel               string
		expectVoice               string
		expectFormat              string
		expectSpeed               float64
		expectTTSTimeout          time.Duration
		expectFetchTimeout        time.Duration
		expectTTSBackend          string
		expectGoogleTranslateLang string
		expectOutputDir           string
	}{
		{
			name:                      "defaults",
			mockEnv:                   map[string]string{},
			expectError:               false,
			expectBaseURL:             "http://localhost:8000/v1",
			expectModel:               "speaches-ai/Kokoro-82M-v1.0-ONNX",
			expectVoice:               "af_heart",
			expectFormat:              "mp3",
			expectSpeed:               1.0,
			expectTTSTimeout:          5 * time.Minute,
			expectFetchTimeout:        30 * time.Second,
			expectTTSBackend:          "openai",
			expectGoogleTranslateLang: "en",
		},
		{
			name: "URL from env",
			mockEnv: map[string]string{
				"URL": "https://example.com/article",
			},
			expectURL:                 "https://example.com/article",
			expectBaseURL:             "http://localhost:8000/v1",
			expectModel:               "speaches-ai/Kokoro-82M-v1.0-ONNX",
			expectVoice:               "af_heart",
			expectFormat:              "mp3",
			expectSpeed:               1.0,
			expectTTSTimeout:          5 * time.Minute,
			expectFetchTimeout:        30 * time.Second,
			expectTTSBackend:          "openai",
			expectGoogleTranslateLang: "en",
		},
		{
			name: "FILE from env",
			mockEnv: map[string]string{
				"FILE": "/tmp/article.html",
			},
			expectFile:                "/tmp/article.html",
			expectBaseURL:             "http://localhost:8000/v1",
			expectModel:               "speaches-ai/Kokoro-82M-v1.0-ONNX",
			expectVoice:               "af_heart",
			expectFormat:              "mp3",
			expectSpeed:               1.0,
			expectTTSTimeout:          5 * time.Minute,
			expectFetchTimeout:        30 * time.Second,
			expectTTSBackend:          "openai",
			expectGoogleTranslateLang: "en",
		},
		{
			name: "all TTS env vars",
			mockEnv: map[string]string{
				"OPENAI_BASE_URL": "http://speaches:8000/v1",
				"OPENAI_API_KEY":  "test-key",
				"TTS_MODEL":       "custom-model",
				"TTS_VOICE":       "custom-voice",
				"TTS_FORMAT":      "wav",
				"TTS_SPEED":       "1.5",
				"TTS_TIMEOUT":     "2m",
				"FETCH_TIMEOUT":   "10s",
			},
			expectBaseURL:             "http://speaches:8000/v1",
			expectToken:               "test-key",
			expectModel:               "custom-model",
			expectVoice:               "custom-voice",
			expectFormat:              "wav",
			expectSpeed:               1.5,
			expectTTSTimeout:          2 * time.Minute,
			expectFetchTimeout:        10 * time.Second,
			expectTTSBackend:          "openai",
			expectGoogleTranslateLang: "en",
		},
		{
			name:                      "env overrides .env file",
			mockEnv:                   map[string]string{"URL": "https://env.example.com"},
			mockEnvFile:               "URL=https://file.example.com\n",
			expectURL:                 "https://env.example.com",
			expectBaseURL:             "http://localhost:8000/v1",
			expectModel:               "speaches-ai/Kokoro-82M-v1.0-ONNX",
			expectVoice:               "af_heart",
			expectFormat:              "mp3",
			expectSpeed:               1.0,
			expectTTSTimeout:          5 * time.Minute,
			expectFetchTimeout:        30 * time.Second,
			expectTTSBackend:          "openai",
			expectGoogleTranslateLang: "en",
		},
		{
			name: "TTS backend from env",
			mockEnv: map[string]string{
				"TTS_BACKEND": "google",
			},
			expectTTSBackend:          "google",
			expectBaseURL:             "http://localhost:8000/v1",
			expectModel:               "speaches-ai/Kokoro-82M-v1.0-ONNX",
			expectVoice:               "af_heart",
			expectFormat:              "mp3",
			expectSpeed:               1.0,
			expectTTSTimeout:          5 * time.Minute,
			expectFetchTimeout:        30 * time.Second,
			expectGoogleTranslateLang: "en",
		},
		{
			name: "Google Translate lang from env",
			mockEnv: map[string]string{
				"GOOGLE_TRANSLATE_LANG": "fr",
			},
			expectGoogleTranslateLang: "fr",
			expectTTSBackend:          "openai",
			expectBaseURL:             "http://localhost:8000/v1",
			expectModel:               "speaches-ai/Kokoro-82M-v1.0-ONNX",
			expectVoice:               "af_heart",
			expectFormat:              "mp3",
			expectSpeed:               1.0,
			expectTTSTimeout:          5 * time.Minute,
			expectFetchTimeout:        30 * time.Second,
		},
		{
			name: "OutputDir from env",
			mockEnv: map[string]string{
				"OUTPUT_DIR": "/tmp/audio",
			},
			expectOutputDir:           "/tmp/audio",
			expectTTSBackend:          "openai",
			expectGoogleTranslateLang: "en",
			expectBaseURL:             "http://localhost:8000/v1",
			expectModel:               "speaches-ai/Kokoro-82M-v1.0-ONNX",
			expectVoice:               "af_heart",
			expectFormat:              "mp3",
			expectSpeed:               1.0,
			expectTTSTimeout:          5 * time.Minute,
			expectFetchTimeout:        30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current directory: %v", err)
			}

			envVarsToClean := []string{
				"URL", "FILE", "OUTPUT",
				"OPENAI_BASE_URL", "OPENAI_API_KEY",
				"TTS_MODEL", "TTS_VOICE", "TTS_FORMAT",
				"TTS_SPEED", "TTS_INSTRUCTIONS",
				"TTS_TIMEOUT", "FETCH_TIMEOUT",
				"TTS_BACKEND", "GOOGLE_TRANSLATE_LANG", "OUTPUT_DIR",
			}
			origVals := map[string]string{}
			for _, k := range envVarsToClean {
				origVals[k] = os.Getenv(k)
				os.Unsetenv(k)
			}
			defer func() {
				for _, k := range envVarsToClean {
					if origVals[k] != "" {
						os.Setenv(k, origVals[k])
					} else {
						os.Unsetenv(k)
					}
				}
			}()

			tmpDir := t.TempDir()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Logf("warning: failed to restore directory: %v", err)
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

			conf := GetEnvVars()

			if conf.URL != tt.expectURL {
				t.Errorf("expected URL %q, got %q", tt.expectURL, conf.URL)
			}
			if conf.File != tt.expectFile {
				t.Errorf("expected File %q, got %q", tt.expectFile, conf.File)
			}
			if conf.OpenAIBaseURL != tt.expectBaseURL {
				t.Errorf("expected OpenAIBaseURL %q, got %q", tt.expectBaseURL, conf.OpenAIBaseURL)
			}
			if conf.OpenAIToken != tt.expectToken {
				t.Errorf("expected OpenAIToken %q, got %q", tt.expectToken, conf.OpenAIToken)
			}
			if conf.TTSModel != tt.expectModel {
				t.Errorf("expected TTSModel %q, got %q", tt.expectModel, conf.TTSModel)
			}
			if conf.TTSVoice != tt.expectVoice {
				t.Errorf("expected TTSVoice %q, got %q", tt.expectVoice, conf.TTSVoice)
			}
			if conf.TTSFormat != tt.expectFormat {
				t.Errorf("expected TTSFormat %q, got %q", tt.expectFormat, conf.TTSFormat)
			}
			if conf.TTSSpeed != tt.expectSpeed {
				t.Errorf("expected TTSSpeed %v, got %v", tt.expectSpeed, conf.TTSSpeed)
			}
			if conf.TTSTimeout != tt.expectTTSTimeout {
				t.Errorf("expected TTSTimeout %v, got %v", tt.expectTTSTimeout, conf.TTSTimeout)
			}
			if conf.FetchTimeout != tt.expectFetchTimeout {
				t.Errorf("expected FetchTimeout %v, got %v", tt.expectFetchTimeout, conf.FetchTimeout)
			}
			if conf.TTSBackend != tt.expectTTSBackend {
				t.Errorf("expected TTSBackend %q, got %q", tt.expectTTSBackend, conf.TTSBackend)
			}
			if conf.GoogleTranslateLang != tt.expectGoogleTranslateLang {
				t.Errorf("expected GoogleTranslateLang %q, got %q", tt.expectGoogleTranslateLang, conf.GoogleTranslateLang)
			}
			if conf.OutputDir != tt.expectOutputDir {
				t.Errorf("expected OutputDir %q, got %q", tt.expectOutputDir, conf.OutputDir)
			}
		})
	}
}
