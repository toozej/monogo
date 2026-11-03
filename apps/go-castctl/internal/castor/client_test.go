package castor

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	result    commandResult
	err       error
	startErr  error
	runArgs   []string
	runEnv    []string
	startArgs []string
	startEnv  []string
}

func (f *fakeRunner) Run(_ context.Context, args, env []string) commandResult {
	f.runArgs = append([]string(nil), args...)
	f.runEnv = append([]string(nil), env...)
	result := f.result
	result.err = f.err
	return result
}

func (f *fakeRunner) Start(_ context.Context, args, env []string) error {
	f.startArgs = append([]string(nil), args...)
	f.startEnv = append([]string(nil), env...)
	return f.startErr
}

func TestReceivePreservesCastorRanking(t *testing.T) {
	runner := &fakeRunner{result: commandResult{stdout: []byte("900\thttps://cdn.example/low.mp4\n2400\thttps://cdn.example/high.m3u8\n1200\thttps://cdn.example/mid.webm\n")}}
	client := newWithRunner(runner, "/missing/config.yaml", time.Second)
	t.Cleanup(func() { _ = client.Close() })

	stream, err := client.Receive(context.Background(), "https://example.com/watch?id=one two")
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
	if stream.URL != "https://cdn.example/low.mp4" || stream.BitRate != 900 {
		t.Fatalf("unexpected best stream: %#v", stream)
	}
	wantArgs := []string{"--config", "/missing/config.yaml", "cast", "--dry-run", "player", "https://example.com/watch?id=one two"}
	if !reflect.DeepEqual(runner.runArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", runner.runArgs, wantArgs)
	}
	if len(runner.runEnv) != 0 {
		t.Fatalf("dry run must not override device settings: %#v", runner.runEnv)
	}
}

func TestReceiveRejectsMissingOrMalformedStreams(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{name: "empty"},
		{name: "missing tab", output: "123 https://cdn.example/video.mp4\n"},
		{name: "invalid bitrate", output: "fast\thttps://cdn.example/video.mp4\n"},
		{name: "unsupported scheme", output: "123\tfile:///tmp/video.mp4\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newWithRunner(&fakeRunner{result: commandResult{stdout: []byte(tt.output)}}, "", time.Second)
			t.Cleanup(func() { _ = client.Close() })
			if _, err := client.Receive(context.Background(), "https://example.com/watch"); err == nil {
				t.Fatal("Receive returned nil error")
			}
		})
	}
}

func TestSendUsesSelectedDeviceWithoutShellParsing(t *testing.T) {
	runner := &fakeRunner{}
	client := newWithRunner(runner, "config.yaml", time.Second)
	t.Cleanup(func() { _ = client.Close() })
	source := "https://example.com/watch?q=$(touch nope)&title=one two"
	dev := Device{ID: "cast:living-room", Name: "Living Room", Type: "chromecast", Address: "192.0.2.10:8009"}

	if err := client.Send(context.Background(), source, dev); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	want := []string{"--config", "config.yaml", "cast", "player", source}
	if !reflect.DeepEqual(runner.startArgs, want) {
		t.Fatalf("args = %#v, want %#v", runner.startArgs, want)
	}
	for _, item := range []string{
		"CASTOR_DEVICE__NAME=Living Room",
		"CASTOR_DEVICE__TYPE=chromecast",
		"CASTOR_DEVICE__ID=cast:living-room",
		"CASTOR_DEVICE__HOST=",
	} {
		if !contains(runner.startEnv, item) {
			t.Errorf("environment missing %q: %#v", item, runner.startEnv)
		}
	}
}

func TestSendValidationAndStartError(t *testing.T) {
	tests := []struct {
		name   string
		device Device
	}{
		{name: "unknown type", device: Device{Name: "TV", Type: "telepathy"}},
		{name: "missing name", device: Device{Type: "chromecast"}},
		{name: "newline", device: Device{Name: "TV\nINJECTED=yes", Type: "chromecast"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newWithRunner(&fakeRunner{}, "", time.Second)
			t.Cleanup(func() { _ = client.Close() })
			if err := client.Send(context.Background(), "https://example.com", tt.device); err == nil {
				t.Fatal("Send returned nil error")
			}
		})
	}

	client := newWithRunner(&fakeRunner{startErr: errors.New("not found")}, "", time.Second)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Send(context.Background(), "https://example.com", Device{Name: "TV", Type: "dlna"}); err == nil || !strings.Contains(err.Error(), "start castor") {
		t.Fatalf("unexpected start error: %v", err)
	}
}

func TestDevicesParsesScanOutput(t *testing.T) {
	runner := &fakeRunner{result: commandResult{stdout: []byte("Living Room TV\tchromecast\t192.0.2.10\tcast:living-room\r\nBedroom\tdlna\thttp://192.0.2.11:9197/dmr\tuuid:bedroom\nOffice\troku\t192.0.2.12\troku:office\n")}}
	client := newWithRunner(runner, "", time.Second)
	t.Cleanup(func() { _ = client.Close() })
	devices, err := client.Devices(context.Background())
	if err != nil {
		t.Fatalf("Devices returned error: %v", err)
	}
	if len(devices) != 3 || devices[0].ID != "cast:living-room" || devices[2].Type != "roku" || devices[0].Name != "Living Room TV" || devices[1].Type != "dlna" {
		t.Fatalf("unexpected devices: %#v", devices)
	}
	if !reflect.DeepEqual(runner.runArgs, []string{"scan"}) {
		t.Fatalf("scan args = %#v", runner.runArgs)
	}
}

func TestDevicesReportsCastorErrorWithoutUnboundedStderr(t *testing.T) {
	runner := &fakeRunner{err: errors.New("exit 1"), result: commandResult{stderr: []byte(strings.Repeat("x", 2048))}}
	client := newWithRunner(runner, "", time.Second)
	t.Cleanup(func() { _ = client.Close() })
	_, err := client.Devices(context.Background())
	if err == nil {
		t.Fatal("Devices returned nil error")
	}
	if len(err.Error()) > 1200 || !strings.Contains(err.Error(), "...") {
		t.Fatalf("stderr was not bounded: length=%d error=%q", len(err.Error()), err)
	}
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// Rows match Castor v2.5.0 cmd/castor/internal/cast/dryrun.go.
func TestReceiveParsesV2RankedStreams(t *testing.T) {
	for _, output := range []string{
		"unknown\thttps://cdn.example/preferred.m3u8\n9000\thttps://cdn.example/fallback.mp4\tlast resort\n",
		"0\thttps://cdn.example/preferred.m3u8\tlast resort\n",
	} {
		client := newWithRunner(&fakeRunner{result: commandResult{stdout: []byte(output)}}, "", time.Second)
		t.Cleanup(func() { _ = client.Close() })
		stream, err := client.Receive(context.Background(), "https://example.com/watch")
		if err != nil || stream.URL != "https://cdn.example/preferred.m3u8" || stream.BitRate != 0 {
			t.Fatalf("Receive = %#v, %v", stream, err)
		}
	}
}

func TestDeviceEnvironmentOverridesConfiguredTarget(t *testing.T) {
	t.Setenv("CASTOR_DEVICE__ID", "old-id")
	t.Setenv("CASTOR_DEVICE__HOST", "192.0.2.99")
	env := mergedEnvironment(deviceEnvironment(Device{Name: "Selected TV", Type: "dlna"}))
	if !contains(env, "CASTOR_DEVICE__ID=") || !contains(env, "CASTOR_DEVICE__HOST=") || contains(env, "CASTOR_DEVICE__ID=old-id") || contains(env, "CASTOR_DEVICE__HOST=192.0.2.99") {
		t.Fatalf("configured target overrides selection: %#v", env)
	}
}

// Set CASTOR_TEST_BINARY to a v2.5.0 release binary to check the CLI contract.
func TestCastorV2Binary(t *testing.T) {
	binaryPath := os.Getenv("CASTOR_TEST_BINARY")
	if binaryPath == "" {
		t.Skip("CASTOR_TEST_BINARY is not set")
	}
	// Encode only the protobuf fields used by these fixtures.
	field := func(tag byte, value []byte) []byte {
		row := binary.AppendUvarint([]byte{tag}, uint64(len(value)))
		return append(row, value...)
	}
	deviceRow := func(id string, kind byte, address string) []byte {
		row := field(10, []byte(id))
		row = append(row, field(18, []byte("TV"))...)
		row = append(row, 24, kind)
		row = append(row, field(34, []byte(address))...)
		return field(10, row)
	}
	castTarget := make(chan []byte, 1)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/proto")
		switch r.URL.Path {
		case "/castor.v1.DeviceService/ListDevices":
			rows := append(deviceRow("uuid:tv", 1, "192.0.2.1"), deviceRow("roku:tv", 3, "192.0.2.2")...)
			_, _ = w.Write(append(rows, deviceRow("uuid:other-tv", 1, "192.0.2.3")...))
		case "/castor.v1.CastService/Resolve":
			preferred := field(10, field(10, []byte("https://cdn.example/preferred.m3u8")))
			fallback := field(10, []byte("https://cdn.example/fallback.mp4"))
			fallback = binary.AppendUvarint(append(fallback, 16), 9000)
			fallback = append(fallback, 24, 1)
			_, _ = w.Write(append(preferred, field(10, fallback)...))
		case "/castor.v1.CastService/Cast":
			body, _ := io.ReadAll(r.Body)
			castTarget <- body
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPreconditionFailed)
			_, _ = w.Write([]byte(`{"code":"failed_precondition","message":"contract-test-stop"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	t.Chdir(t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("CASTOR_SERVERS__API__URL", api.URL)
	client := New(binaryPath, "", 5*time.Second)
	t.Cleanup(func() { _ = client.Close() })
	devices, err := client.Devices(context.Background())
	if err != nil || len(devices) != 3 || devices[0].ID != "uuid:tv" || devices[1].Type != "roku" {
		t.Fatalf("Devices = %#v, %v", devices, err)
	}
	stream, err := client.Receive(context.Background(), "https://example.com/watch")
	if err != nil || stream.URL != "https://cdn.example/preferred.m3u8" || stream.BitRate != 0 {
		t.Fatalf("Receive = %#v, %v", stream, err)
	}
	// Duplicate names require the scanned ID. File and environment targets must not override it.
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("device:\n  id: uuid:other-tv\n  host: 192.0.2.99\n"), 0600); err != nil {
		t.Fatal(err)
	}
	client.configPath = configPath
	t.Setenv("CASTOR_DEVICE__ID", "uuid:other-tv")
	t.Setenv("CASTOR_DEVICE__HOST", "192.0.2.99")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := client.runner.Run(ctx, client.args("cast", "player", "https://example.com/watch"), deviceEnvironment(devices[0]))
	if result.err == nil || !bytes.Contains(result.stderr, []byte("contract-test-stop")) {
		t.Fatalf("send did not reach the mock API: %v, %s", result.err, result.stderr)
	}
	select {
	case body := <-castTarget:
		// CastRequest.target is field 1. Target.device_id is field 1.
		if !bytes.Contains(body, field(10, field(10, []byte("uuid:tv")))) {
			t.Fatalf("send selected the wrong target: %x", body)
		}
	default:
		t.Fatal("send did not submit a cast")
	}

}
