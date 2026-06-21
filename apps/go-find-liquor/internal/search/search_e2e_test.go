//go:build e2e

package search

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestE2ESearchItem(t *testing.T) {
	searcher := NewSearcher("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	results, err := searcher.SearchItem(ctx, "99900014675", "97202", 15)
	if err != nil {
		t.Fatalf("SearchItem failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("SearchItem returned 0 results for item 99900014675 (JACK DANIELS #7 BL LABEL), expected at least 1")
	}

	first := results[0]

	if first.Name == "" {
		t.Error("LiquorItem.Name is empty")
	}
	if !strings.Contains(first.Name, "JACK DANIELS") {
		t.Errorf("LiquorItem.Name = %q, expected to contain 'JACK DANIELS'", first.Name)
	}

	if first.Code == "" {
		t.Error("LiquorItem.Code is empty")
	}

	if first.Store == "" {
		t.Error("LiquorItem.Store is empty")
	}

	if first.Price == "" {
		t.Error("LiquorItem.Price is empty")
	}
	if !strings.HasPrefix(first.Price, "$") {
		t.Errorf("LiquorItem.Price = %q, expected to start with '$'", first.Price)
	}

	if first.Date.IsZero() {
		t.Error("LiquorItem.Date is zero")
	}
}

func TestE2EAgeVerification(t *testing.T) {
	searcher := NewSearcher("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	if err := searcher.AgeVerification(); err != nil {
		t.Fatalf("AgeVerification failed: %v", err)
	}
}

func TestE2EExtractProductInfo(t *testing.T) {
	searcher := NewSearcher("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	results, err := searcher.SearchItem(ctx, "99900014675", "97202", 15)
	if err != nil {
		t.Fatalf("SearchItem failed: %v", err)
	}

	if len(results) == 0 {
		t.Skip("No results returned, cannot verify product info extraction")
	}

	first := results[0]

	if first.Code != "0146B" {
		t.Errorf("LiquorItem.Code = %q, expected '0146B' (short code from parentheses)", first.Code)
	}

	if first.Price != "$22.95" {
		t.Errorf("LiquorItem.Price = %q, expected '$22.95'", first.Price)
	}
}

func TestE2ESearchItemNonExistent(t *testing.T) {
	searcher := NewSearcher("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	results, err := searcher.SearchItem(ctx, "NONEXISTENT_ITEM_12345", "97202", 15)
	if err != nil {
		t.Fatalf("SearchItem for non-existent item returned error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("SearchItem for non-existent item returned %d results, expected 0", len(results))
	}
}
