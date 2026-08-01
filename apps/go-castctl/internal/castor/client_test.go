package castor

import (
	"context"
	"errors"
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

func TestReceiveSelectsHighestBitrate(t *testing.T) {
	runner := &fakeRunner{result: commandResult{stdout: []byte("900\thttps://cdn.example/low.mp4\n2400\thttps://cdn.example/high.m3u8\n1200\thttps://cdn.example/mid.webm\n")}}
	client := newWithRunner(runner, "/missing/config.yaml", time.Second)
	t.Cleanup(func() { _ = client.Close() })

	stream, err := client.Receive(context.Background(), "https://example.com/watch?id=one two")
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
	if stream.URL != "https://cdn.example/high.m3u8" || stream.BitRate != 2400 {
		t.Fatalf("unexpected best stream: %#v", stream)
	}
	wantArgs := []string{"--config", "/missing/config.yaml", "cast", "--dry-run", "player", "https://example.com/watch?id=one two"}
	if !reflect.DeepEqual(runner.runArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", runner.runArgs, wantArgs)
	}
	if !contains(runner.runEnv, "CASTOR_DEVICE__TYPE=chromecast") || !contains(runner.runEnv, "CASTOR_DEVICE__NAME=go-castctl-dry-run") {
		t.Fatalf("receive environment missing validation endpoint: %#v", runner.runEnv)
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
	dev := Device{Name: "Living Room", Type: "chromecast", Address: "192.0.2.10:8009"}

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
	runner := &fakeRunner{result: commandResult{stdout: []byte("Living Room TV\tchromecast\t192.0.2.10\r\nBedroom\tdlna\thttp://192.0.2.11:9197/dmr\n")}}
	client := newWithRunner(runner, "", time.Second)
	t.Cleanup(func() { _ = client.Close() })
	devices, err := client.Devices(context.Background())
	if err != nil {
		t.Fatalf("Devices returned error: %v", err)
	}
	if len(devices) != 2 || devices[0].Name != "Living Room TV" || devices[1].Type != "dlna" {
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
