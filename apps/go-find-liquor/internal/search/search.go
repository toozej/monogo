package search

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/sirupsen/logrus"
)

const (
	baseURL       = "https://www.oregonliquorsearch.com/"
	searchURL     = "https://www.oregonliquorsearch.com/servlet/FrontController"
	ageBtnFormURL = "https://www.oregonliquorsearch.com/servlet/WelcomeController"
)

// DefaultCommonItems are items that are typically always in stock at OLCC stores,
// used as fallback for health check searches when none are configured.
var DefaultCommonItems = []string{
	"99900046075",
	"99900014675",
	"99900088075",
	"99900054075",
	"99900202175",
	"99900069075",
}

// RandomCommonItem returns a random item from the provided list.
// If the list is empty, it falls back to DefaultCommonItems.
func RandomCommonItem(items []string) string {
	if len(items) == 0 {
		items = DefaultCommonItems
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(items))))
	if err != nil {
		return items[0]
	}
	return items[n.Int64()]
}

// User agent strings to cycle through
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/119.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:102.0) Gecko/20100101 Firefox/102.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/119.0",
}

// LiquorItem represents a found liquor item
// with only the information we care about
type LiquorItem struct {
	Name  string
	Code  string
	Store string
	Date  time.Time
	Price string
}

// ProductInfo represents all the possible information about a liquor item
// including the information we don't really care about
type ProductInfo struct {
	ItemCode    string
	Name        string
	BottlePrice string
	CasePrice   string
	Size        string
	Proof       string
	Category    string
}

// Searcher provides functionality to search for liquor items
type Searcher struct {
	client     *http.Client
	userAgent  string
	cycleAgent bool
}

// NewSearcher creates a new searcher with cookie support
func NewSearcher(userAgent string) *Searcher {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	bigLenUserAgents := new(big.Int)
	bigLenUserAgents.SetInt64(int64(len(userAgents))) // Convert int to int64 first
	randUserAgent, _ := rand.Int(rand.Reader, bigLenUserAgents)
	cycleAgent := userAgent == ""
	if cycleAgent {
		userAgent = userAgents[randUserAgent.Int64()]
	}

	return &Searcher{
		client:     client,
		userAgent:  userAgent,
		cycleAgent: cycleAgent,
	}
}

// updateUserAgent sets a new random user agent if cycling is enabled
func (s *Searcher) updateUserAgent() {
	if s.cycleAgent {
		bigLenUserAgents := new(big.Int)
		bigLenUserAgents.SetInt64(int64(len(userAgents))) // Convert int to int64 first
		randUserAgent, _ := rand.Int(rand.Reader, bigLenUserAgents)
		s.userAgent = userAgents[randUserAgent.Int64()]
		log.Debugf("Using user agent: %s", s.userAgent)
	}
}

// AgeVerification performs the age verification
func (s *Searcher) AgeVerification() error {
	// First get the page to get session cookies
	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)

	resp, err := s.client.Do(req) // #nosec G704 -- URL is from config, not user input
	if err != nil {
		return fmt.Errorf("failed to get page: %w", err)
	}
	defer resp.Body.Close()

	// Parse the form for the age verification
	_, err = goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse page: %w", err)
	}

	// Prepare the form submission for age verification
	formData := url.Values{}
	formData.Set("ageCheck", "true")
	formData.Set("action", "search")

	// Submit the form
	log.Debugf("AgeVerification() POSTing %v\n", formData)
	req, err = http.NewRequest("POST", ageBtnFormURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create form submission request: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", ageBtnFormURL)

	resp, err = s.client.Do(req) // #nosec G704 -- URL is hardcoded
	if err != nil {
		return fmt.Errorf("failed to submit age verification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("age verification failed with status: %s", resp.Status)
	}

	return nil
}

// SearchItem searches for a specific liquor item by name or code
func (s *Searcher) SearchItem(ctx context.Context, item string, zipcode string, distance int) ([]LiquorItem, error) {
	s.updateUserAgent()

	// Perform age verification before search
	if err := s.AgeVerification(); err != nil {
		return nil, fmt.Errorf("age verification failed: %w", err)
	}

	// Prepare search form data
	formData := url.Values{}
	formData.Set("view", "global")
	formData.Set("action", "search")
	formData.Set("radiusSearchParam", fmt.Sprintf("%d", distance))
	formData.Set("productSearchParam", item)
	formData.Set("locationSearchParam", zipcode)
	formData.Set("btnSearch", "Search")

	// Submit search form
	log.Debugf("SearchItem() POSTing formData %v\n", formData)
	req, err := http.NewRequest("POST", searchURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", searchURL)

	// Perform search request
	resp, err := s.client.Do(req) // #nosec G704 -- URL is hardcoded
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status: %s", resp.Status)
	}

	// Generate goquery document from response
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to generate goquery document from search query response: %w", err)
	}

	// Extract product information
	product := extractProductInfo(doc)

	// Extract results from the table and generate list of found LiquorItem
	results := extractResults(doc, product)

	return results, nil
}

// extractResults extracts found products from the table and creates a list of found liquor item results
func extractResults(doc *goquery.Document, product ProductInfo) []LiquorItem {
	var results []LiquorItem

	doc.Find("tr.row, tr.alt-row").Each(func(i int, s *goquery.Selection) {
		// Check if the store has stock
		qtyText := strings.TrimSpace(s.Find("td.qty").Text())
		if qtyText == "0" {
			return // Skip stores with no stock
		}

		tds := s.Find("td")
		// The actual table columns are:
		// [0]Store No, [1]Location(City), [2]Address, [3]Zip, [4]Telephone, [5]Store Hours, [6]Qty, [7]Distance
		// Note: Store No (td[0]) contains <noscript><a>...</noscript><span class="link">NNNN</span><noscript>...</noscript>
		// The store number is in <span class="link">, so we prefer that; fall back to full td text.
		storeNoTd := tds.Eq(0)
		storeNo := strings.TrimSpace(storeNoTd.Find("span.link").Text())
		if storeNo == "" {
			storeNo = strings.TrimSpace(storeNoTd.Text())
		}
		location := strings.TrimSpace(tds.Eq(1).Text())

		// Combine store number and city for a meaningful store identifier
		storeName := location
		if storeNo != "" {
			storeName = storeNo + " - " + location
		}

		if storeName != "" {
			results = append(results, LiquorItem{
				Name:  product.Name,
				Code:  product.ItemCode,
				Store: storeName,
				Date:  time.Now(),
				Price: product.BottlePrice,
			})
		}
	})

	return results
}

// extractProductInfo extracts product details from the product-details table
func extractProductInfo(doc *goquery.Document) ProductInfo {
	product := ProductInfo{}

	// Extract product name and item code from the product description
	// The actual HTML contains: "Item\n\t99900014675(0146B):\n\tJACK DANIELS #7 BL LABEL"
	// We need to normalize whitespace before parsing.
	productDescRaw := doc.Find("#product-desc h2").Text()
	// Normalize whitespace: replace tabs/newlines with spaces, collapse multiple spaces
	productDesc := strings.TrimSpace(strings.Join(strings.Fields(productDescRaw), " "))
	if productDesc != "" {
		// Parse "Item 99900733075(7330B): MICHTER'S STRAIGHT RYE"
		parts := strings.SplitN(productDesc, ":", 2)
		if len(parts) == 2 {
			// Extract the item code from "Item 99900014675(0146B)"
			itemParts := strings.Split(parts[0], " ")
			if len(itemParts) >= 2 {
				fullCode := itemParts[1]
				// Extract the code in parentheses if it exists
				codeInParens := ""
				if i := strings.Index(fullCode, "("); i != -1 {
					if j := strings.Index(fullCode, ")"); j != -1 && j > i {
						codeInParens = fullCode[i+1 : j]
					}
				}

				if codeInParens != "" {
					product.ItemCode = codeInParens
				} else {
					product.ItemCode = fullCode
				}
			}

			// Extract the product name
			product.Name = strings.TrimSpace(parts[1])
		}
	}

	// Extract product details from the table.
	// The actual HTML table has multi-row layout where <th> and <td> are
	// siblings within each <tr>, e.g.:
	//   <tr><th>Category:</th><td>DOMESTIC WHISKEY</td><th>Age:</th><td> </td></tr>
	//   <tr><th>Size:</th><td>750 ML</td><th>Case Price:</th><td>$275.40</td></tr>
	//   <tr><th>Proof:</th><td>80.0</td><th>Bottle Price:</th><td>$22.95</td></tr>
	// The product description <th> with colspan="4" has no following <td>,
	// so we skip it by checking that th.Next() has elements.
	doc.Find("#product-details tr").Each(func(i int, s *goquery.Selection) {
		s.Find("th").Each(func(j int, th *goquery.Selection) {
			next := th.Next()
			if next.Length() == 0 {
				return // Skip <th> elements without a following sibling (e.g., product-desc)
			}
			// Only process if the next sibling is a <td>
			if !next.Is("td") {
				return
			}
			label := strings.TrimSpace(th.Text())
			value := strings.TrimSpace(next.Text())
			switch label {
			case "Bottle Price:":
				product.BottlePrice = value
			case "Case Price:":
				product.CasePrice = value
			case "Size:":
				product.Size = value
			case "Proof:":
				product.Proof = value
			case "Category:":
				product.Category = value
			}
		})
	})

	return product
}
