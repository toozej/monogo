package search

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeRoundTripper serves the age-verification GET with a canned body and makes
// the subsequent POST fail, reproducing the CI network conditions.
type fakeRoundTripper struct {
	getBody string
	postErr error
}

func (f fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodGet {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(f.getBody)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}
	return nil, f.postErr
}

// TestAgeVerificationPostFailureReturnsError guards a regression: when the first
// GET succeeded but the age-verification POST failed, a deferred Close over the
// reused resp variable dereferenced a nil response and panicked. It must return
// the error instead.
func TestAgeVerificationPostFailureReturnsError(t *testing.T) {
	s := &Searcher{
		client: &http.Client{
			Transport: fakeRoundTripper{
				getBody: "<html><body><form></form></body></html>",
				postErr: errors.New("simulated network failure"),
			},
		},
		userAgent: "test-agent",
	}

	err := s.AgeVerification()
	if err == nil {
		t.Fatal("expected an error when the age-verification POST fails, got nil")
	}
}

func TestRandomCommonItem(t *testing.T) {
	item := RandomCommonItem(nil)
	if item == "" {
		t.Error("RandomCommonItem(nil) returned empty string")
	}

	found := false
	for _, ci := range DefaultCommonItems {
		if ci == item {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("RandomCommonItem(nil) returned '%s' which is not in DefaultCommonItems list", item)
	}
}

func TestRandomCommonItemWithCustomList(t *testing.T) {
	customItems := []string{"ITEM_A", "ITEM_B", "ITEM_C"}
	item := RandomCommonItem(customItems)

	found := false
	for _, ci := range customItems {
		if ci == item {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("RandomCommonItem() returned '%s' which is not in custom list", item)
	}
}

func TestDefaultCommonItemsNotEmpty(t *testing.T) {
	if len(DefaultCommonItems) == 0 {
		t.Error("DefaultCommonItems list should not be empty")
	}
}

func TestRandomCommonItemDistribution(t *testing.T) {
	results := make(map[string]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		item := RandomCommonItem(nil)
		results[item]++
	}

	if len(results) < 2 {
		t.Errorf("Expected randomness across items, but only %d unique item(s) selected in %d iterations", len(results), iterations)
	}
}
