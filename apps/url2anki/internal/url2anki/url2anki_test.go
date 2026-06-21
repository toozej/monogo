package url2anki

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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
	flashcards, err := scrapeFlashcards(server.URL, "div.term-name", "div.term-definition")
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
	defer os.Remove(tmpfile.Name())

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
	defer os.Remove(tmpfile.Name())

	// Call the exportFlashcardsToCSVFile function
	if err := exportFlashcardsToCSVFile(flashcards, tmpfile.Name()); err != nil {
		t.Fatalf("exportFlashcardsToCSV returned an error: %v", err)
	}

	// Read the file back and verify its content
	file, err := os.Open(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to open the file: %v", err)
	}
	defer file.Close()

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
