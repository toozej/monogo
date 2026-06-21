// Package cmd provides the serve command for starting the RSSFFS web server.
//
// This file implements the "serve" subcommand that starts an HTTP server
// providing a web-based interface for RSS feed discovery and subscription.
// The command integrates with the existing RSSFFS configuration system
// and provides command-line flags for server configuration.
package cmd

import (
	"fmt"
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/toozej/RSSFFS/internal/web"
	"github.com/toozej/RSSFFS/pkg/config"
)

// ServeCommand holds configuration options for the serve command
type ServeCommand struct {
	Host string
	Port int
}

// NewServeCommand creates and returns a new serve command
func NewServeCommand() *cobra.Command {
	serveCmd := &ServeCommand{}

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the RSSFFS web server",
		Long: `Start the RSSFFS web server to provide a browser-based interface
for RSS feed discovery and subscription. The web interface allows users
to submit URLs and categories through a form instead of using command-line arguments.`,
		RunE: serveCmd.runServer,
	}

	// Add command-specific flags
	cmd.Flags().StringVarP(&serveCmd.Host, "host", "H", "127.0.0.1", "Host address to bind the server to")
	cmd.Flags().IntVarP(&serveCmd.Port, "port", "p", 8080, "Port number to listen on")

	return cmd
}

// runServer executes the serve command
func (s *ServeCommand) runServer(cmd *cobra.Command, args []string) error {
	// Load configuration using existing mechanism
	conf := config.GetEnvVars()

	// Use config values as defaults if flags weren't explicitly set
	if !cmd.Flags().Changed("host") {
		s.Host = conf.WebHost
	}
	if !cmd.Flags().Changed("port") {
		s.Port = conf.WebPort
	}

	// Validate port range
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("invalid port number: %d (must be between 1 and 65535)", s.Port)
	}

	// Check if port is available
	if err := s.checkPortAvailability(); err != nil {
		return fmt.Errorf("port %d is not available: %w", s.Port, err)
	}

	// Create and start the web server
	server := web.NewServer(conf, debug)

	log.Infof("Starting RSSFFS web server...")
	log.Infof("Server will be available at: http://%s:%d", s.Host, s.Port)
	log.Infof("Press Ctrl+C to stop the server")

	if debug {
		log.Debugf("Debug mode enabled")
		log.Debugf("RSS Reader Endpoint: %s", conf.RSSReaderEndpoint)
	}

	// Start the server (this blocks until shutdown)
	if err := server.Start(s.Host, s.Port); err != nil {
		return fmt.Errorf("server failed to start: %w", err)
	}

	return nil
}

// checkPortAvailability verifies that the specified port is available for binding
func (s *ServeCommand) checkPortAvailability() error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)

	// Try to listen on the address to check availability
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	// Close the listener immediately since we're just checking availability
	return listener.Close()
}
