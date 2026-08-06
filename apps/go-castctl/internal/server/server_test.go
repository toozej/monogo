package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/toozej/monogo/apps/go-castctl/internal/castor"
	"github.com/toozej/monogo/apps/go-castctl/internal/config"
)

type fakeService struct {
	devices       []castor.Device
	devicesErr    error
	sendErr       error
	receiveStream castor.Stream
	receiveErr    error
	sendCalls     []request
	receiveURLs   []string
}

func (f *fakeService) Devices(context.Context) ([]castor.Device, error) {
	return f.devices, f.devicesErr
}

func (f *fakeService) Send(_ context.Context, source string, device castor.Device) error {
	f.sendCalls = append(f.sendCalls, request{URL: source, Device: device})
	return f.sendErr
}

func (f *fakeService) Receive(_ context.Context, source string) (castor.Stream, error) {
	f.receiveURLs = append(f.receiveURLs, source)
	return f.receiveStream, f.receiveErr
}

func testServer(service castor.Service) *Server {
	return New(config.Server{
		Host: "127.0.0.1", Port: 8080,
		ReadTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: time.Second,
	}, service)
}

func TestSendAliases(t *testing.T) {
	for _, path := range []string{"/send", "/cast"} {
		t.Run(path, func(t *testing.T) {
			service := &fakeService{}
			srv := testServer(service)
			body := `{"url":"https://example.com/watch","device":{"name":"TV","type":"chromecast","address":"192.0.2.10"}}`
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			srv.Handler().ServeHTTP(res, req)
			if res.Code != http.StatusAccepted {
				t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
			}
			if len(service.sendCalls) != 1 || service.sendCalls[0].Device.Address != "192.0.2.10" {
				t.Fatalf("unexpected send calls: %#v", service.sendCalls)
			}
		})
	}
}

func TestReceiveAliases(t *testing.T) {
	for _, path := range []string{"/receive", "/view", "/watch"} {
		t.Run(path, func(t *testing.T) {
			service := &fakeService{receiveStream: castor.Stream{URL: "https://cdn.example/video.mp4", BitRate: 1234}}
			srv := testServer(service)
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("url=https%3A%2F%2Fexample.com%2Fwatch"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			res := httptest.NewRecorder()
			srv.Handler().ServeHTTP(res, req)
			if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "https://cdn.example/video.mp4") {
				t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
			}
			if len(service.receiveURLs) != 1 || service.receiveURLs[0] != "https://example.com/watch" {
				t.Fatalf("unexpected receive calls: %#v", service.receiveURLs)
			}
		})
	}
}

func TestAPIMethodValidation(t *testing.T) {
	for _, path := range []string{"/send", "/cast", "/receive", "/view", "/watch"} {
		t.Run(path, func(t *testing.T) {
			res := httptest.NewRecorder()
			testServer(&fakeService{}).Handler().ServeHTTP(res, httptest.NewRequest(http.MethodPut, path, nil))
			if res.Code != http.StatusMethodNotAllowed || res.Header().Get("Allow") != http.MethodPost {
				t.Fatalf("status=%d allow=%q", res.Code, res.Header().Get("Allow"))
			}
		})
	}
}

func TestAPIValidation(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "malformed JSON", path: "/receive", body: `{`},
		{name: "relative URL", path: "/receive", body: `{"url":"/video"}`},
		{name: "credentials", path: "/receive", body: `{"url":"https://user:pass@example.com/video"}`},
		{name: "missing device", path: "/send", body: `{"url":"https://example.com/video"}`},
		{name: "invalid device type", path: "/send", body: `{"url":"https://example.com/video","device":{"name":"TV","type":"other"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			testServer(&fakeService{}).Handler().ServeHTTP(res, req)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
			}
		})
	}
}

func TestServiceErrors(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		fake   *fakeService
		status int
		body   string
	}{
		{name: "send", path: "/send", fake: &fakeService{sendErr: errors.New("secret stderr")}, status: http.StatusBadGateway, body: `{"url":"https://example.com","device":{"name":"TV","type":"chromecast"}}`},
		{name: "receive", path: "/receive", fake: &fakeService{receiveErr: errors.New("secret stderr")}, status: http.StatusBadGateway, body: `{"url":"https://example.com"}`},
		{name: "receive timeout", path: "/receive", fake: &fakeService{receiveErr: context.DeadlineExceeded}, status: http.StatusGatewayTimeout, body: `{"url":"https://example.com"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			testServer(tt.fake).Handler().ServeHTTP(res, req)
			if res.Code != tt.status || strings.Contains(res.Body.String(), "secret stderr") {
				t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
			}
		})
	}
}

func TestDevicesAndGoAppResources(t *testing.T) {
	service := &fakeService{devices: []castor.Device{{Name: "TV", Type: "chromecast", Address: "192.0.2.10"}}}
	srv := testServer(service)

	res := httptest.NewRecorder()
	srv.Handler().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/devices", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "192.0.2.10") {
		t.Fatalf("devices status = %d, body = %s", res.Code, res.Body.String())
	}

	res = httptest.NewRecorder()
	srv.Handler().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "go-castctl") || !strings.Contains(res.Body.String(), "Cast to TV") {
		t.Fatalf("page status = %d, body = %s", res.Code, res.Body.String())
	}

	res = httptest.NewRecorder()
	srv.Handler().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/web/placeholder.txt", nil))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "generated app.wasm") {
		t.Fatalf("embedded resource status = %d, body = %q", res.Code, res.Body.String())
	}

	res = httptest.NewRecorder()
	srv.Handler().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/web/app.wasm", nil))
	if res.Code != http.StatusOK || res.Header().Get("Content-Type") != "application/wasm" || res.Body.Len() == 0 {
		t.Fatalf("wasm status = %d, content-type = %q, length = %d", res.Code, res.Header().Get("Content-Type"), res.Body.Len())
	}
}

func TestJSONResponseContract(t *testing.T) {
	service := &fakeService{receiveStream: castor.Stream{URL: "https://cdn.example/video.mp4", BitRate: 55}}
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/receive", bytes.NewBufferString(`{"url":"https://example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	testServer(service).Handler().ServeHTTP(res, req)
	var decoded apiResponse
	if err := json.Unmarshal(res.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if decoded.Status != "ready" || decoded.StreamURL == "" || decoded.BitRate != 55 {
		t.Fatalf("unexpected response: %#v", decoded)
	}
}
