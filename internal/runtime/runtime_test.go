package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseNodeRuntime(t *testing.T) {
	tests := []struct {
		input       string
		wantVersion string
		wantOK      bool
	}{
		{"node12", "12", true},
		{"node16", "16", true},
		{"node20", "20", true},
		{"node22", "22", true},
		{"Node20", "20", true},
		{" node20 ", "20", true},
		{"docker", "", false},
		{"composite", "", false},
		{"node", "", false},
		{"nodeabc", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			version, ok := ParseNodeRuntime(tt.input)
			if version != tt.wantVersion || ok != tt.wantOK {
				t.Errorf("ParseNodeRuntime(%q) = (%q, %v), want (%q, %v)", tt.input, version, ok, tt.wantVersion, tt.wantOK)
			}
		})
	}
}

func TestEOLClient_FetchReleaseEOL(t *testing.T) {
	eolFrom := "2023-09-11"
	notEOLFrom := "2027-10-01"

	tests := []struct {
		name         string
		product      string
		version      string
		responseCode int
		responseBody interface{}
		wantEOL      bool
		wantNil      bool
		wantErr      bool
		wantEOLDate  string
	}{
		{
			name:         "node 16 is EOL",
			product:      "nodejs",
			version:      "16",
			responseCode: 200,
			responseBody: ProductReleaseResponse{
				SchemaVersion: "1.0.0",
				Result: ProductRelease{
					Name:    "16",
					IsEol:   true,
					EolFrom: &eolFrom,
				},
			},
			wantEOL:     true,
			wantEOLDate: "2023-09-11",
		},
		{
			name:         "node 22 is not EOL",
			product:      "nodejs",
			version:      "22",
			responseCode: 200,
			responseBody: ProductReleaseResponse{
				SchemaVersion: "1.0.0",
				Result: ProductRelease{
					Name:    "22",
					IsEol:   false,
					EolFrom: &notEOLFrom,
				},
			},
			wantEOL:     false,
			wantEOLDate: "2027-10-01",
		},
		{
			name:         "unknown version returns nil",
			product:      "nodejs",
			version:      "99",
			responseCode: 404,
			wantNil:      true,
		},
		{
			name:         "server error",
			product:      "nodejs",
			version:      "16",
			responseCode: 500,
			wantErr:      true,
		},
		{
			name:    "request error",
			product: "nodejs",
			version: "16",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			var client *EOLClient

			if tt.name == "request error" {
				client = NewEOLClientWithHTTP("http://127.0.0.1:0", http.DefaultClient)
			} else {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("User-Agent") == "" {
						t.Error("Expected User-Agent header")
					}
					w.WriteHeader(tt.responseCode)
					if tt.responseBody != nil {
						body, _ := json.Marshal(tt.responseBody)
						if _, err := w.Write(body); err != nil {
							t.Errorf("failed to write response body: %v", err)
						}
					}
				}))
				defer server.Close()
				client = NewEOLClientWithHTTP(server.URL, server.Client())
			}

			ctx := context.Background()
			info, err := client.FetchReleaseEOL(ctx, tt.product, tt.version)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.wantNil {
				if info != nil {
					t.Errorf("Expected nil, got %+v", info)
				}
				return
			}

			if info == nil {
				t.Fatal("Expected non-nil result")
			}

			if info.IsEOL != tt.wantEOL {
				t.Errorf("IsEOL = %v, want %v", info.IsEOL, tt.wantEOL)
			}

			if tt.wantEOLDate != "" {
				expectedDate, _ := time.Parse("2006-01-02", tt.wantEOLDate)
				if !info.EOLDate.Equal(expectedDate) {
					t.Errorf("EOLDate = %v, want %v", info.EOLDate, expectedDate)
				}
			}
		})
	}
}

func TestEOLClient_FetchReleaseEOL_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		eolFrom := "2023-09-11"
		resp := ProductReleaseResponse{
			SchemaVersion: "1.0.0",
			Result: ProductRelease{
				Name:    "16",
				IsEol:   true,
				EolFrom: &eolFrom,
			},
		}
		w.WriteHeader(200)
		body, _ := json.Marshal(resp)
		if _, err := w.Write(body); err != nil {
			t.Errorf("failed to write response body: %v", err)
		}
	}))
	defer server.Close()

	client := NewEOLClientWithHTTP(server.URL, server.Client())
	ctx := context.Background()

	info1, err := client.FetchReleaseEOL(ctx, "nodejs", "16")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !info1.IsEOL {
		t.Error("Expected IsEOL=true")
	}

	info2, err := client.FetchReleaseEOL(ctx, "nodejs", "16")
	if err != nil {
		t.Fatalf("Unexpected error on cached call: %v", err)
	}
	if !info2.IsEOL {
		t.Error("Expected IsEOL=true from cache")
	}

	if callCount != 1 {
		t.Errorf("Expected 1 API call (second should be cached), got %d", callCount)
	}
}

func TestEOLClient_CheckRunsUsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/nodejs/releases/20") {
			eol := "2026-04-30"
			resp := ProductReleaseResponse{
				SchemaVersion: "1.0.0",
				Result: ProductRelease{
					Name:    "20",
					IsEol:   true,
					EolFrom: &eol,
				},
			}
			w.WriteHeader(200)
			body, _ := json.Marshal(resp)
			if _, err := w.Write(body); err != nil {
				t.Errorf("failed to write response body: %v", err)
			}
			return
		}
		if strings.Contains(r.URL.Path, "/nodejs/releases/22") {
			notEOL := "2027-10-01"
			resp := ProductReleaseResponse{
				SchemaVersion: "1.0.0",
				Result: ProductRelease{
					Name:    "22",
					IsEol:   false,
					EolFrom: &notEOL,
				},
			}
			w.WriteHeader(200)
			body, _ := json.Marshal(resp)
			if _, err := w.Write(body); err != nil {
				t.Errorf("failed to write response body: %v", err)
			}
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := NewEOLClientWithHTTP(server.URL, server.Client())
	ctx := context.Background()

	info, err := client.CheckRunsUsing(ctx, "node20")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("Expected non-nil result for node20")
	}
	if !info.IsEOL {
		t.Error("Expected node20 to be EOL")
	}
	if info.Version != "20" {
		t.Errorf("Expected version 20, got %s", info.Version)
	}

	info, err = client.CheckRunsUsing(ctx, "node22")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("Expected non-nil result for node22")
	}
	if info.IsEOL {
		t.Error("Expected node22 to not be EOL")
	}

	info, err = client.CheckRunsUsing(ctx, "docker")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if info != nil {
		t.Error("Expected nil for docker runtime")
	}
}

func TestRuntimeEOLInfoString(t *testing.T) {
	info := &RuntimeEOLInfo{
		Runtime: "nodejs",
		Version: "20",
		EOLDate: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		IsEOL:   true,
	}
	expected := "nodejs20 (EOL since 2026-04-30)"
	if info.String() != expected {
		t.Errorf("String() = %q, want %q", info.String(), expected)
	}
}

func TestRuntimeEOLInfoString_ZeroEOLDate(t *testing.T) {
	info := &RuntimeEOLInfo{
		Runtime: "nodejs",
		Version: "20",
		EOLDate: time.Time{},
		IsEOL:   true,
	}
	expected := "nodejs20 (EOL since unknown)"
	if info.String() != expected {
		t.Errorf("String() = %q, want %q", info.String(), expected)
	}
}

func TestRuntimeEOLInfoString_Nil(t *testing.T) {
	var info *RuntimeEOLInfo
	if info.String() != "" {
		t.Errorf("String() = %q, want empty string for nil", info.String())
	}
}
