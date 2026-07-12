package url2anki

import (
	"bytes"
	"context"
	"crypto/rand"
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
	if timeout <= 0 {
		return fmt.Errorf("http timeout must be greater than zero")
	}
	if maxResponseBytes <= 0 {
		return fmt.Errorf("maximum response size must be greater than zero")
	}
	outputFormat := strings.ToLower(filepath.Ext(outputFile))
	if outputFormat != ".json" && outputFormat != ".csv" {
		return fmt.Errorf("unsupported output format %q: use .json or .csv", filepath.Ext(outputFile))
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

	switch outputFormat {
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
	}
	return nil
}

// scrapeFlashcards scrapes the flashcards from the provided URL using the provided HTML selectors
func scrapeFlashcards(ctx context.Context, rawURL, questionSelector, answerSelector string, client *http.Client, maxResponseBytes int64) ([]Flashcard, error) {
	rawURL = strings.TrimSpace(rawURL)
	questionSelector = strings.TrimSpace(questionSelector)
	answerSelector = strings.TrimSpace(answerSelector)
	if questionSelector == "" {
		return nil, fmt.Errorf("question selector is required")
	}
	if answerSelector == "" {
		return nil, fmt.Errorf("answer selector is required")
	}
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
	if maxResponseBytes <= 0 {
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
	body, err := readResponse(res.Body, maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
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

func readResponse(body io.Reader, maxResponseBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponseBytes))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) < maxResponseBytes {
		return data, nil
	}

	var extra [1]byte
	n, err := io.ReadFull(body, extra[:])
	if n > 0 {
		return nil, fmt.Errorf("response exceeds %d-byte limit", maxResponseBytes)
	}
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data, nil
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
	return writeAtomically(filename, 0o600, func(file *os.File) error {
		_, err := file.Write(data)
		return err
	})
}

// exportFlashcardsToCSVFile exports the flashcards to a JSON file
func exportFlashcardsToCSVFile(flashcards []Flashcard, filename string) error {
	return writeAtomically(filename, 0o666, func(file *os.File) error {
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

func writeAtomically(filename string, mode os.FileMode, write func(*os.File) error) error {
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	preserveMode := false
	if info, statErr := os.Stat(absPath); statErr == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
		preserveMode = true
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	temp, err := createTemporaryFile(filepath.Dir(absPath), filepath.Base(absPath), mode)
	if err != nil {
		return err
	}
	if preserveMode {
		if err := temp.Chmod(mode); err != nil {
			_ = temp.Close()
			_ = os.Remove(temp.Name())
			return err
		}
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
	if err := replaceFile(tempPath, absPath); err != nil {
		return err
	}
	return nil
}

func createTemporaryFile(dir, base string, mode os.FileMode) (*os.File, error) {
	for {
		path := filepath.Join(dir, "."+base+".tmp-"+rand.Text())
		// #nosec G304 G302 -- dir and base come from the explicit output path,
		// the random suffix and O_EXCL prevent replacement, and mode preserves
		// the legacy exporter permissions while still applying the user's umask.
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, mode)
		if os.IsExist(err) {
			continue
		}
		return file, err
	}
}
