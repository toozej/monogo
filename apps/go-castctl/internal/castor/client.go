// Package castor adapts the external Castor CLI to go-castctl's HTTP service.
package castor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Device is a Castor-compatible playback endpoint. Castor selects devices by
// name and type; Address is discovery metadata shown to the user.
type Device struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
}

// Stream is a browser-playable stream candidate reported by Castor.
type Stream struct {
	URL     string `json:"streamURL"`
	BitRate int64  `json:"bitrate"`
}

// Service is the API used by the HTTP server.
type Service interface {
	Devices(context.Context) ([]Device, error)
	Send(context.Context, string, Device) error
	Receive(context.Context, string) (Stream, error)
}

type commandResult struct {
	stdout []byte
	stderr []byte
	err    error
}

type runner interface {
	Run(context.Context, []string, []string) commandResult
	Start(context.Context, []string, []string) error
}

type execRunner struct {
	binary string
}

func (r execRunner) Run(ctx context.Context, args, env []string) commandResult {
	cmd := exec.CommandContext(ctx, r.binary, args...) // #nosec G204 -- executable is configuration; arguments never pass through a shell.
	cmd.Env = mergedEnvironment(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), err: err}
}

func (r execRunner) Start(ctx context.Context, args, env []string) error {
	cmd := exec.CommandContext(ctx, r.binary, args...) // #nosec G204 -- executable is configuration; arguments never pass through a shell.
	cmd.Env = mergedEnvironment(env)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			slog.Error("castor session exited", "error", err)
		}
	}()
	return nil
}

func mergedEnvironment(overrides []string) []string {
	keys := make(map[string]struct{}, len(overrides))
	for _, item := range overrides {
		if key, _, ok := strings.Cut(item, "="); ok {
			keys[key] = struct{}{}
		}
	}
	env := slices.DeleteFunc(slices.Clone(os.Environ()), func(item string) bool {
		key, _, _ := strings.Cut(item, "=")
		_, overridden := keys[key]
		return overridden
	})
	return append(env, overrides...)
}

// Client invokes Castor commands. Long-running send sessions are tied to the
// client lifetime rather than the request that launched them.
type Client struct {
	runner     runner
	configPath string
	timeout    time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
	closeOnce  sync.Once
}

// New creates a Castor client backed by an external executable.
func New(binary, configPath string, timeout time.Duration) *Client {
	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel is retained and called by Client.Close.
	return &Client{
		runner:     execRunner{binary: binary},
		configPath: configPath,
		timeout:    timeout,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func newWithRunner(r runner, configPath string, timeout time.Duration) *Client {
	ctx, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel is retained and called by Client.Close.
	return &Client{runner: r, configPath: configPath, timeout: timeout, ctx: ctx, cancel: cancel}
}

// Close stops any active Castor sessions launched by Send.
func (c *Client) Close() error {
	c.closeOnce.Do(c.cancel)
	return nil
}

// Devices scans the local network through Castor.
func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	result := c.runner.Run(ctx, c.args("scan"), nil)
	if result.err != nil {
		return nil, commandError("scan", ctx, result.stderr, result.err)
	}
	devices, err := parseDevices(result.stdout)
	if err != nil {
		return nil, fmt.Errorf("parse castor scan: %w", err)
	}
	return devices, nil
}

// Send starts a Castor player session and returns as soon as the process starts.
func (c *Client) Send(_ context.Context, sourceURL string, device Device) error {
	if err := validateDevice(device); err != nil {
		return err
	}
	env := deviceEnvironment(device)
	if err := c.runner.Start(c.ctx, c.args("cast", "player", sourceURL), env); err != nil {
		return fmt.Errorf("start castor session: %w", err)
	}
	return nil
}

// Receive asks Castor to extract streams and returns the highest-bitrate result.
func (c *Client) Receive(ctx context.Context, sourceURL string) (Stream, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	// Castor validates device config before reaching its dry-run path. This
	// placeholder name satisfies validation but is never discovered or used.
	env := deviceEnvironment(Device{Name: "go-castctl-dry-run", Type: "chromecast"})
	result := c.runner.Run(ctx, c.args("cast", "--dry-run", "player", sourceURL), env)
	if result.err != nil {
		return Stream{}, commandError("resolve", ctx, result.stderr, result.err)
	}
	streams, err := parseStreams(result.stdout)
	if err != nil {
		return Stream{}, fmt.Errorf("parse castor streams: %w", err)
	}
	best := streams[0]
	for _, stream := range streams[1:] {
		if stream.BitRate > best.BitRate {
			best = stream
		}
	}
	return best, nil
}

func (c *Client) args(args ...string) []string {
	result := make([]string, 0, len(args)+2)
	if c.configPath != "" {
		result = append(result, "--config", c.configPath)
	}
	return append(result, args...)
}

func deviceEnvironment(device Device) []string {
	return []string{
		"CASTOR_DEVICE__NAME=" + device.Name,
		"CASTOR_DEVICE__TYPE=" + device.Type,
	}
}

func validateDevice(device Device) error {
	switch device.Type {
	case "chromecast", "dlna":
	default:
		return fmt.Errorf("unsupported device type %q", device.Type)
	}
	if strings.TrimSpace(device.Name) == "" {
		return errors.New("device name is required")
	}
	if strings.ContainsAny(device.Name, "\r\n\x00") {
		return errors.New("device name contains invalid characters")
	}
	return nil
}

func parseDevices(data []byte) ([]Device, error) {
	var devices []Device
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.EqualFold(line, "no devices found") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected line %q", line)
		}
		device := Device{Name: strings.TrimSpace(fields[0]), Type: strings.TrimSpace(fields[1]), Address: strings.TrimSpace(fields[2])}
		if err := validateDevice(device); err != nil {
			return nil, fmt.Errorf("invalid device %q: %w", device.Name, err)
		}
		devices = append(devices, device)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return devices, nil
}

func parseStreams(data []byte) ([]Stream, error) {
	var streams []Stream
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 {
			return nil, fmt.Errorf("unexpected line %q", line)
		}
		bitRate, err := strconv.ParseInt(strings.TrimSpace(fields[0]), 10, 64)
		if err != nil || bitRate < 0 {
			return nil, fmt.Errorf("invalid bitrate in %q", line)
		}
		streamURL := strings.TrimSpace(fields[1])
		parsed, err := url.Parse(streamURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return nil, fmt.Errorf("invalid stream URL %q", streamURL)
		}
		streams = append(streams, Stream{URL: streamURL, BitRate: bitRate})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, errors.New("castor found no browser-playable streams")
	}
	return streams, nil
}

func commandError(operation string, ctx context.Context, stderr []byte, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("castor %s: %w", operation, ctxErr)
	}
	const maxStderr = 1024
	stderr = bytes.TrimSpace(stderr)
	if len(stderr) > maxStderr {
		stderr = append(stderr[:maxStderr], []byte("...")...)
	}
	if len(stderr) != 0 {
		return fmt.Errorf("castor %s: %w: %s", operation, err, stderr)
	}
	return fmt.Errorf("castor %s: %w", operation, err)
}
