package search

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// fakeRoundTripper serves the age-verification GET with a canned body and makes
// the subsequent POST fail, reproducing the CI network conditions.
type fakeRoundTripper struct {
	getBody string
	postErr error
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func response(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status, Status: http.StatusText(status),
		Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: req,
	}
}

func TestSearchItemHonorsContextOnEveryRequest(t *testing.T) {
	requestCount := 0
	searcher := &Searcher{
		client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			if requestCount < 3 {
				return response(req, http.StatusOK, "<html></html>"), nil
			}
			<-req.Context().Done()
			return nil, req.Context().Err()
		})},
		userAgent: "test-agent",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := searcher.SearchItem(ctx, "item", "97201", 10); err == nil {
		t.Fatal("SearchItem() error = nil, want context deadline error")
	}
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
}

func TestAgeVerificationRejectsBadStatusAndOversizedBody(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "bad status", status: http.StatusInternalServerError, body: "error"},
		{name: "oversized", status: http.StatusOK, body: strings.Repeat("x", maxHTMLBytes+1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searcher := &Searcher{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return response(req, tt.status, tt.body), nil
			})}, userAgent: "test-agent"}
			if err := searcher.AgeVerificationContext(context.Background()); err == nil {
				t.Fatal("AgeVerificationContext() error = nil")
			}
		})
	}
}

func TestExtractResultsRequiresPositiveIntegerQuantity(t *testing.T) {
	html := `<table>
	<tr class="row"><td><span class="link">1</span></td><td>Portland</td><td></td><td></td><td></td><td></td><td class="qty"></td></tr>
	<tr class="row"><td><span class="link">2</span></td><td>Salem</td><td></td><td></td><td></td><td></td><td class="qty">unknown</td></tr>
	<tr class="row"><td><span class="link">3</span></td><td>Eugene</td><td></td><td></td><td></td><td></td><td class="qty">0</td></tr>
	<tr class="row"><td><span class="link">4</span></td><td>Bend</td><td></td><td></td><td></td><td></td><td class="qty">2</td></tr>
	</table>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	got := extractResults(doc, ProductInfo{Name: "Whiskey", ItemCode: "123", BottlePrice: "$10"})
	if len(got) != 1 || got[0].Store != "4 - Bend" {
		t.Fatalf("extractResults() = %+v, want only positive integer quantity", got)
	}
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
