package url2anki

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// TestScrapeFlashcards tests the scrapeFlashcards function
func TestScrapeFlashcards(t *testing.T) {
	// Mock HTML content
	htmlContent := `
		<div class="term-name">Question 1</div>
		<div class="term-definition">Answer 1</div>
		<div class="term-name">Question 2</div>
		<div class="term-definition">Answer 2</div>
	`

	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(htmlContent))
	}))
	defer server.Close()

	// Call the scrapeFlashcards function
	flashcards, err := scrapeFlashcards(context.Background(), server.URL, "div.term-name", "div.term-definition", server.Client(), 1<<20)
	if err != nil {
		t.Fatalf("scrapeFlashcards returned an error: %v", err)
	}

	// Assert the flashcards content
	expectedFlashcards := []Flashcard{
		{Question: "Question 1", Answer: "Answer 1"},
		{Question: "Question 2", Answer: "Answer 2"},
	}

	if len(flashcards) != len(expectedFlashcards) {
		t.Fatalf("Expected %d flashcards, got %d", len(expectedFlashcards), len(flashcards))
	}

	for i, card := range flashcards {
		if card.Question != expectedFlashcards[i].Question || card.Answer != expectedFlashcards[i].Answer {
			t.Errorf("Expected flashcard %+v, got %+v", expectedFlashcards[i], card)
		}
	}
}

func TestRunPropagatesErrorsAndNormalizesExtension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="q">Question</div><div class="a">Answer</div>`))
	}))
	defer server.Close()

	newCommand := func(output string) *cobra.Command {
		cmd := &cobra.Command{}
		cmd.Flags().String("url", server.URL, "")
		cmd.Flags().String("question-selector", ".q", "")
		cmd.Flags().String("answer-selector", ".a", "")
		cmd.Flags().String("output-file", output, "")
		cmd.Flags().Bool("preview", false, "")
		cmd.Flags().Duration("http-timeout", time.Second, "")
		cmd.Flags().Int64("max-response-bytes", 1<<20, "")
		return cmd
	}

	output := filepath.Join(t.TempDir(), "cards.CSV")
	if err := Run(newCommand(output), nil); err != nil {
		t.Fatalf("uppercase CSV extension was rejected: %v", err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}

	if err := Run(newCommand(filepath.Join(t.TempDir(), "cards.txt")), nil); err == nil {
		t.Fatal("expected unsupported extension error")
	}
}

func TestScrapeFlashcardsRejectsInvalidAndEmptyResults(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		html   string
		status int
	}{
		{name: "malformed URL", url: "://bad"},
		{name: "unsupported scheme", url: "file:///tmp/cards.html"},
		{name: "zero matches", html: "<html><body>no cards</body></html>"},
		{name: "count mismatch", html: `<div class="q">only question</div>`},
		{name: "non-2xx status", html: `<div class="q">q</div><div class="a">a</div>`, status: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawURL := tt.url
			var server *httptest.Server
			client := &http.Client{Timeout: time.Second}
			if rawURL == "" {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tt.status != 0 {
						w.WriteHeader(tt.status)
					}
					_, _ = w.Write([]byte(tt.html))
				}))
				defer server.Close()
				rawURL = server.URL
				client = server.Client()
			}
			if _, err := scrapeFlashcards(context.Background(), rawURL, ".q", ".a", client, 1<<20); err == nil {
				t.Fatal("expected scrape to fail")
			}
		})
	}
}

func TestScrapeFlashcardsBoundsResponseAndTimeout(t *testing.T) {
	t.Run("oversized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", 1025)))
		}))
		defer server.Close()
		if _, err := scrapeFlashcards(context.Background(), server.URL, ".q", ".a", server.Client(), 1024); err == nil {
			t.Fatal("expected oversized response error")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}))
		defer server.Close()
		client := &http.Client{Timeout: 10 * time.Millisecond}
		if _, err := scrapeFlashcards(context.Background(), server.URL, ".q", ".a", client, 1024); err == nil {
			t.Fatal("expected timeout error")
		}
	})
}

func TestScrapeFlashcardsNormalizesCRLF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<div class=\"q\">first\r\nsecond</div><div class=\"a\">one\rtwo</div>"))
	}))
	defer server.Close()
	cards, err := scrapeFlashcards(context.Background(), server.URL, ".q", ".a", server.Client(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if cards[0].Question != "first second" || cards[0].Answer != "one two" {
		t.Fatalf("unexpected normalized card: %+v", cards[0])
	}
}

// TestExportFlashcardsToFile tests the exportFlashcardsToFile function
func TestExportFlashcardsToFile(t *testing.T) {
	flashcards := []Flashcard{
		{Question: "Question 1", Answer: "Answer 1"},
		{Question: "Question 2", Answer: "Answer 2"},
	}

	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "flashcards*.json")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	// Call the exportFlashcardsToJSONFile function
	if err := exportFlashcardsToJSONFile(flashcards, tmpfile.Name()); err != nil {
		t.Fatalf("exportFlashcardsToFile returned an error: %v", err)
	}

	// Read the file back and verify its content
	data, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read the file: %v", err)
	}

	var exportedFlashcards []Flashcard
	if err := json.Unmarshal(data, &exportedFlashcards); err != nil {
		t.Fatalf("Failed to unmarshal file contents: %v", err)
	}

	if len(exportedFlashcards) != len(flashcards) {
		t.Fatalf("Expected %d flashcards, got %d", len(flashcards), len(exportedFlashcards))
	}

	for i, card := range exportedFlashcards {
		if card.Question != flashcards[i].Question || card.Answer != flashcards[i].Answer {
			t.Errorf("Expected flashcard %+v, got %+v", flashcards[i], card)
		}
	}
}

// TestExportFlashcardsToCSVFile tests the exportFlashcardsToCSV function
func TestExportFlashcardsToCSVFile(t *testing.T) {
	flashcards := []Flashcard{
		{Question: "Question 1", Answer: "Answer 1"},
		{Question: "Question 2", Answer: "Answer 2"},
	}

	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "flashcards*.csv")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	// Call the exportFlashcardsToCSVFile function
	if err := exportFlashcardsToCSVFile(flashcards, tmpfile.Name()); err != nil {
		t.Fatalf("exportFlashcardsToCSV returned an error: %v", err)
	}

	// Read the file back and verify its content
	file, err := os.Open(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to open the file: %v", err)
	}
	defer func() { _ = file.Close() }()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read the CSV file: %v", err)
	}

	expectedRecords := [][]string{
		{"Question", "Answer"},
		{"Question 1", "Answer 1"},
		{"Question 2", "Answer 2"},
	}

	for i, record := range records {
		if strings.Join(record, ",") != strings.Join(expectedRecords[i], ",") {
			t.Errorf("Expected record %v, got %v", expectedRecords[i], record)
		}
	}
}
