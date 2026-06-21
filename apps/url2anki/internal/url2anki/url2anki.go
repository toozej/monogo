package url2anki

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

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
func Run(cmd *cobra.Command, args []string) {
	inputURL, _ := cmd.Flags().GetString("url")
	pageURL, _ := url.ParseRequestURI(inputURL)
	url := pageURL.String()
	questionSelector, _ := cmd.Flags().GetString("question-selector")
	answerSelector, _ := cmd.Flags().GetString("answer-selector")
	outputFile, _ := cmd.Flags().GetString("output-file")
	preview, _ := cmd.Flags().GetBool("preview")

	// Scrape the flashcards from the provided URL using the specified selectors
	flashcards, err := scrapeFlashcards(url, questionSelector, answerSelector)
	if err != nil {
		fmt.Println("Error scraping flashcards: ", err)
		return
	}

	// If preview is enabled, display flashcards as a table and ask for confirmation
	if preview {
		fmt.Println("Preview of flashcards:")
		printFlashcards(flashcards)
		fmt.Print("Do they look ok? (y/n): ")
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil {
			fmt.Println("Error getting response from user: ", err)
			return
		}
		if strings.ToLower(response) != "y" {
			fmt.Println("Aborting.")
			return
		}
	}

	// Export to JSON file
	if outputFile != "" && strings.HasSuffix(outputFile, ".json") {
		if err := exportFlashcardsToJSONFile(flashcards, outputFile); err != nil {
			fmt.Println("Error exporting flashcards to JSON file: ", err)
			return
		}
		fmt.Printf("Flashcards exported to %s\n", outputFile)
	}

	// Export to CSV file
	if outputFile != "" && strings.HasSuffix(outputFile, ".csv") {
		if err := exportFlashcardsToCSVFile(flashcards, outputFile); err != nil {
			fmt.Println("Error exporting flashcards to CSV file: ", err)
			return
		}
		fmt.Printf("Flashcards exported to %s\n", outputFile)
	}
}

// scrapeFlashcards scrapes the flashcards from the provided URL using the provided HTML selectors
func scrapeFlashcards(url, questionSelector, answerSelector string) ([]Flashcard, error) {
	// Request the webpage
	res, err := http.Get(url) //#nosec G107
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, errors.New("failed to fetch the URL")
	}

	// Parse the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}

	// Find the questions and answers using the specified selectors
	questions := doc.Find(questionSelector)
	answers := doc.Find(answerSelector)

	if questions.Length() != answers.Length() {
		return nil, errors.New("the number of questions and answers do not match")
	}

	// Create flashcards by pairing questions and answers
	var flashcards []Flashcard
	questions.Each(func(i int, s *goquery.Selection) {
		// Clean up the question by removing newlines and trimming whitespace
		question := strings.ReplaceAll(s.Text(), "\n", "")
		question = strings.ReplaceAll(question, "\r\n", "")
		question = strings.TrimSpace(question)

		// Clean up the answer by removing newlines and trimming whitespace
		answer := answers.Eq(i).Text()
		answer = strings.ReplaceAll(answer, "\n", "")
		answer = strings.ReplaceAll(answer, "\r\n", "")
		answer = strings.TrimSpace(answer)

		flashcards = append(flashcards, Flashcard{
			Question: question,
			Answer:   answer,
		})
	})

	return flashcards, nil
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
	return os.WriteFile(filename, data, 0600) // #nosec G304 G703 -- filename from user CLI arg, expected
}

// exportFlashcardsToCSVFile exports the flashcards to a JSON file
func exportFlashcardsToCSVFile(flashcards []Flashcard, filename string) error {
	file, err := os.Create(filename) //#nosec G304
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	err = writer.Write([]string{"Question", "Answer"})
	if err != nil {
		return err
	}

	// Write flashcard data
	for _, flashcard := range flashcards {
		err := writer.Write([]string{flashcard.Question, flashcard.Answer})
		if err != nil {
			return err
		}
	}

	return nil
}
