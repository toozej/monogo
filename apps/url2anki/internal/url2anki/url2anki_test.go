package url2anki

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
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
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
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
	requestCount := requests.Load()

	if err := Run(newCommand(filepath.Join(t.TempDir(), "cards.txt")), nil); err == nil {
		t.Fatal("expected unsupported extension error")
	}
	if got := requests.Load(); got != requestCount {
		t.Fatalf("unsupported output made %d HTTP requests, want none", got-requestCount)
	}

	invalidTimeout := newCommand(filepath.Join(t.TempDir(), "cards.csv"))
	if err := invalidTimeout.Flags().Set("http-timeout", "0s"); err != nil {
		t.Fatal(err)
	}
	if err := Run(invalidTimeout, nil); err == nil {
		t.Fatal("expected zero timeout error")
	}
	invalidLimit := newCommand(filepath.Join(t.TempDir(), "cards.csv"))
	if err := invalidLimit.Flags().Set("max-response-bytes", "0"); err != nil {
		t.Fatal(err)
	}
	if err := Run(invalidLimit, nil); err == nil {
		t.Fatal("expected zero response limit error")
	}
	if got := requests.Load(); got != requestCount {
		t.Fatalf("invalid request bounds made %d HTTP requests, want none", got-requestCount)
	}

	invalidURL := newCommand(filepath.Join(t.TempDir(), "cards.csv"))
	if err := invalidURL.Flags().Set("url", "://bad"); err != nil {
		t.Fatal(err)
	}
	if err := Run(invalidURL, nil); err == nil || !strings.Contains(err.Error(), "scrape flashcards") {
		t.Fatalf("Run() scrape error = %v, want propagated scrape error", err)
	}

	missingParent := filepath.Join(t.TempDir(), "missing", "cards.csv")
	if err := Run(newCommand(missingParent), nil); err == nil || !strings.Contains(err.Error(), "export flashcards to CSV") {
		t.Fatalf("Run() export error = %v, want propagated export error", err)
	}
}

func TestRunPropagatesPreviewInputError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="q">Question</div><div class="a">Answer</div>`))
	}))
	defer server.Close()

	cmd := &cobra.Command{}
	cmd.Flags().String("url", server.URL, "")
	cmd.Flags().String("question-selector", ".q", "")
	cmd.Flags().String("answer-selector", ".a", "")
	output := filepath.Join(t.TempDir(), "cards.csv")
	cmd.Flags().String("output-file", output, "")
	cmd.Flags().Bool("preview", true, "")
	cmd.Flags().Duration("http-timeout", time.Second, "")
	cmd.Flags().Int64("max-response-bytes", 1<<20, "")

	input, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := inputWriter.Close(); err != nil {
		t.Fatal(err)
	}
	originalStdin := os.Stdin
	os.Stdin = input
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = input.Close()
	})

	if err := Run(cmd, nil); err == nil || !strings.Contains(err.Error(), "read preview response") {
		t.Fatalf("Run() preview input error = %v, want propagated input error", err)
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output exists after preview input failure: %v", err)
	}
}

func TestScrapeFlashcardsRejectsInvalidAndEmptyResults(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		html             string
		status           int
		questionSelector string
		answerSelector   string
	}{
		{name: "malformed URL", url: "://bad"},
		{name: "unsupported scheme", url: "file:///tmp/cards.html"},
		{name: "blank question selector", url: "https://example.com", questionSelector: " \t"},
		{name: "blank answer selector", url: "https://example.com", answerSelector: " \t"},
		{name: "zero matches", html: "<html><body>no cards</body></html>"},
		{name: "count mismatch", html: `<div class="q">only question</div>`},
		{name: "non-2xx status", html: `<div class="q">q</div><div class="a">a</div>`, status: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawURL := tt.url
			questionSelector := ".q"
			answerSelector := ".a"
			if tt.questionSelector != "" {
				questionSelector = tt.questionSelector
			}
			if tt.answerSelector != "" {
				answerSelector = tt.answerSelector
			}
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
			if _, err := scrapeFlashcards(context.Background(), rawURL, questionSelector, answerSelector, client, 1<<20); err == nil {
				t.Fatal("expected scrape to fail")
			}
		})
	}
}

func TestScrapeFlashcardsBoundsResponseAndTimeout(t *testing.T) {
	validHTML := []byte(`<div class="q">q</div><div class="a">a</div>`)

	t.Run("exact limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(validHTML)
		}))
		defer server.Close()
		if _, err := scrapeFlashcards(context.Background(), server.URL, ".q", ".a", server.Client(), int64(len(validHTML))); err != nil {
			t.Fatalf("response exactly at limit was rejected: %v", err)
		}
	})

	t.Run("maximum int64 limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(validHTML)
		}))
		defer server.Close()
		if _, err := scrapeFlashcards(context.Background(), server.URL, ".q", ".a", server.Client(), math.MaxInt64); err != nil {
			t.Fatalf("maximum int64 response limit was rejected: %v", err)
		}
	})

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
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
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
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
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

func TestAtomicExportPreservesModesAndCleansTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	flashcards := []Flashcard{{Question: "Question", Answer: "Answer"}}

	jsonPath := filepath.Join(dir, "cards.json")
	if err := exportFlashcardsToJSONFile(flashcards, jsonPath); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(jsonPath)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
			t.Fatalf("new JSON mode = %o, want %o", got, want)
		}
	}

	referencePath := filepath.Join(dir, "reference")
	reference, err := os.Create(referencePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := reference.Close(); err != nil {
		t.Fatal(err)
	}
	csvPath := filepath.Join(dir, "cards.csv")
	if err := exportFlashcardsToCSVFile(flashcards, csvPath); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		csvInfo, err := os.Stat(csvPath)
		if err != nil {
			t.Fatal(err)
		}
		referenceInfo, err := os.Stat(referencePath)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := csvInfo.Mode().Perm(), referenceInfo.Mode().Perm(); got != want {
			t.Fatalf("new CSV mode = %o, want os.Create mode %o", got, want)
		}
	}

	if err := os.Chmod(csvPath, 0o660); err != nil {
		t.Fatal(err)
	}
	if err := exportFlashcardsToCSVFile(flashcards, csvPath); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(csvPath)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o660); got != want {
			t.Fatalf("replacement CSV mode = %o, want preserved %o", got, want)
		}
	}
	assertNoExportTemporaryFiles(t, dir)
}

func TestAtomicExportFailurePreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cards.csv")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	errBoom := errors.New("write failed")
	err := writeAtomically(target, 0o666, func(file *os.File) error {
		if _, err := file.WriteString("replacement"); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("writeAtomically() error = %v, want %v", err, errBoom)
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "original" {
		t.Fatalf("target = %q, want original content", contents)
	}
	assertNoExportTemporaryFiles(t, dir)
}

func TestAtomicExportRenameFailureCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cards.csv")
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}
	err := writeAtomically(target, 0o666, func(file *os.File) error {
		_, err := file.WriteString("replacement")
		return err
	})
	if err == nil {
		t.Fatal("writeAtomically() error = nil, want rename error")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("rename failure replaced target directory")
	}
	assertNoExportTemporaryFiles(t, dir)
}

func assertNoExportTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".*.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}
