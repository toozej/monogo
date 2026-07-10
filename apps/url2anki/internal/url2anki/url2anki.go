package url2anki

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
)

// Flashcard represents a single Anki flashcard
type Flashcard struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// AnkiSyncRequest represents the request structure to the Anki Sync API
type AnkiSyncRequest struct {
	DeckName   string      `json:"deckName"`
	Flashcards []Flashcard `json:"flashcards"`
}

// run is the main function that orchestrates the workflow of url2anki
func Run(cmd *cobra.Command, args []string) error {
	inputURL, err := cmd.Flags().GetString("url")
	if err != nil {
		return err
	}
	questionSelector, err := cmd.Flags().GetString("question-selector")
	if err != nil {
		return err
	}
	answerSelector, err := cmd.Flags().GetString("answer-selector")
	if err != nil {
		return err
	}
	outputFile, err := cmd.Flags().GetString("output-file")
	if err != nil {
		return err
	}
	preview, err := cmd.Flags().GetBool("preview")
	if err != nil {
		return err
	}
	timeout, err := cmd.Flags().GetDuration("http-timeout")
	if err != nil {
		return err
	}
	maxResponseBytes, err := cmd.Flags().GetInt64("max-response-bytes")
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Scrape the flashcards from the provided URL using the specified selectors
	flashcards, err := scrapeFlashcards(ctx, inputURL, questionSelector, answerSelector, &http.Client{Timeout: timeout}, maxResponseBytes)
	if err != nil {
		return fmt.Errorf("scrape flashcards: %w", err)
	}

	// If preview is enabled, display flashcards as a table and ask for confirmation
	if preview {
		fmt.Println("Preview of flashcards:")
		printFlashcards(flashcards)
		fmt.Print("Do they look ok? (y/n): ")
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil {
			return fmt.Errorf("read preview response: %w", err)
		}
		if strings.ToLower(response) != "y" {
			fmt.Println("Aborting.")
			return nil
		}
	}

	switch strings.ToLower(filepath.Ext(outputFile)) {
	case ".json":
		if err := exportFlashcardsToJSONFile(flashcards, outputFile); err != nil {
			return fmt.Errorf("export flashcards to JSON: %w", err)
		}
		fmt.Printf("Flashcards exported to %s\n", outputFile)
	case ".csv":
		if err := exportFlashcardsToCSVFile(flashcards, outputFile); err != nil {
			return fmt.Errorf("export flashcards to CSV: %w", err)
		}
		fmt.Printf("Flashcards exported to %s\n", outputFile)
	default:
		return fmt.Errorf("unsupported output format %q: use .json or .csv", filepath.Ext(outputFile))
	}
	return nil
}

// scrapeFlashcards scrapes the flashcards from the provided URL using the provided HTML selectors
func scrapeFlashcards(ctx context.Context, rawURL, questionSelector, answerSelector string, client *http.Client, maxResponseBytes int64) ([]Flashcard, error) {
	pageURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %q: %w", rawURL, err)
	}
	if (pageURL.Scheme != "http" && pageURL.Scheme != "https") || pageURL.Host == "" {
		return nil, fmt.Errorf("URL must use http or https and include a host")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if maxResponseBytes <= 0 || maxResponseBytes == 1<<63-1 {
		return nil, fmt.Errorf("maximum response size must be greater than zero")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	// Request the webpage
	res, err := client.Do(req) // #nosec G704 -- URL is the explicit CLI input and validated above
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch URL: unexpected HTTP status %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, fmt.Errorf("response exceeds %d-byte limit", maxResponseBytes)
	}

	// Parse the HTML document
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// Find the questions and answers using the specified selectors
	questions := doc.Find(questionSelector)
	answers := doc.Find(answerSelector)

	if questions.Length() != answers.Length() {
		return nil, fmt.Errorf("the number of questions and answers do not match")
	}
	if questions.Length() == 0 {
		return nil, fmt.Errorf("selectors matched no flashcards")
	}

	// Create flashcards by pairing questions and answers
	var flashcards []Flashcard
	questions.Each(func(i int, s *goquery.Selection) {
		// Clean up the question by removing newlines and trimming whitespace
		question := normalizeText(s.Text())

		// Clean up the answer by removing newlines and trimming whitespace
		answer := normalizeText(answers.Eq(i).Text())

		flashcards = append(flashcards, Flashcard{
			Question: question,
			Answer:   answer,
		})
	})

	return flashcards, nil
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// printFlashcards displays the flashcards as a table on the CLI
func printFlashcards(flashcards []Flashcard) {
	fmt.Println("+-----------------------------+-----------------------------+")
	fmt.Println("|           Question           |           Answer            |")
	fmt.Println("+-----------------------------+-----------------------------+")
	for _, flashcard := range flashcards {
		fmt.Printf("| %-27s | %-27s |\n", flashcard.Question, flashcard.Answer)
	}
	fmt.Println("+-----------------------------+-----------------------------+")
}

// exportFlashcardsToJSONFile exports the flashcards to a JSON file
func exportFlashcardsToJSONFile(flashcards []Flashcard, filename string) error {
	data, err := json.MarshalIndent(flashcards, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomically(filename, func(file *os.File) error {
		_, err := file.Write(data)
		return err
	})
}

// exportFlashcardsToCSVFile exports the flashcards to a JSON file
func exportFlashcardsToCSVFile(flashcards []Flashcard, filename string) error {
	return writeAtomically(filename, func(file *os.File) error {
		writer := csv.NewWriter(file)

		// Write header
		if err := writer.Write([]string{"Question", "Answer"}); err != nil {
			return err
		}

		// Write flashcard data
		for _, flashcard := range flashcards {
			if err := writer.Write([]string{flashcard.Question, flashcard.Answer}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	})
}

func writeAtomically(filename string, write func(*os.File) error) error {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(absPath), "."+filepath.Base(absPath)+".tmp-*") // #nosec G304 -- directory is user-selected output location
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := write(temp); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, absPath); err != nil {
		return err
	}
	return nil
}
