package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/toozej/go-listen/pkg/config"
)

func TestServer_Lifecycle(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 0, // Use random port for testing
		},
		Spotify: config.SpotifyConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
		},
	}

	// Create server instance
	server := NewServer(cfg)

	// Test server creation
	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	if server.config != cfg {
		t.Error("Expected server config to match provided config")
	}

	if server.router == nil {
		t.Error("Expected router to be initialized")
	}

	if server.logger == nil {
		t.Error("Expected logger to be initialized")
	}

	if server.spotify == nil {
		t.Error("Expected spotify service to be initialized")
	}

	if server.playlist == nil {
		t.Error("Expected playlist manager to be initialized")
	}
}

func TestServer_StartStop(t *testing.T) {
	// Create test configuration with port 0 for random port assignment
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 0,
		},
		Spotify: config.SpotifyConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
		},
	}

	server := NewServer(cfg)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start()
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test that server is running by making a request
	// Note: We can't easily test the actual HTTP request without knowing the port
	// since we're using port 0, but we can test the shutdown functionality

	// Test graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		t.Errorf("Expected graceful shutdown to succeed, got error: %v", err)
	}

	// Wait for server to actually stop
	select {
	case err := <-serverErr:
		if err != http.ErrServerClosed {
			t.Errorf("Expected server to close with ErrServerClosed, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Server did not stop within timeout")
	}
}

func TestServer_Configuration(t *testing.T) {
	tests := []struct {
		name   string
		config config.Config
	}{
		{
			name: "default configuration",
			config: config.Config{
				Server: config.ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Spotify: config.SpotifyConfig{
					ClientID:     "test_client",
					ClientSecret: "test_secret",
				},
			},
		},
		{
			name: "custom port configuration",
			config: config.Config{
				Server: config.ServerConfig{
					Host: "0.0.0.0",
					Port: 9090,
				},
				Spotify: config.SpotifyConfig{
					ClientID:     "custom_client",
					ClientSecret: "custom_secret",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(&tt.config)

			if server.config.Server.Host != tt.config.Server.Host {
				t.Errorf("Expected host %s, got %s", tt.config.Server.Host, server.config.Server.Host)
			}

			if server.config.Server.Port != tt.config.Server.Port {
				t.Errorf("Expected port %d, got %d", tt.config.Server.Port, server.config.Server.Port)
			}

			expectedAddr := tt.config.Server.Address()
			if server.config.Server.Address() != expectedAddr {
				t.Errorf("Expected address %s, got %s", expectedAddr, server.config.Server.Address())
			}
		})
	}
}

func TestServer_Routes(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Spotify: config.SpotifyConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
		},
	}

	server := NewServer(cfg)
	server.setupRoutes()

	// Test that routes are properly configured
	// We can't easily test the actual route handling without starting the server,
	// but we can verify the router is configured
	if server.router == nil {
		t.Error("Expected router to be configured")
	}
}
