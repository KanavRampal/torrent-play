package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// TorrentSearchResult represents a single torrent found from a search.
type TorrentSearchResult struct {
	Title     string `json:"title"`
	MagnetURL string `json:"magnetUrl"`
	// Future enhancements could include:
	// Seeders   int    `json:"seeders,omitempty"`
	// Leechers  int    `json:"leechers,omitempty"`
	// Size      string `json:"size,omitempty"`
	// UploadDate string `json:"uploadDate,omitempty"`
}

// TorrentSearcher defines the interface for a torrent search service.
type TorrentSearcher interface {
	SearchTorrents(ctx context.Context, query string, page int, orderBy string) ([]TorrentSearchResult, error)
}

const (
	// DefaultBaseURLForTorrentSearch is the default URL for the torrent search site.
	// This should be configurable in a real application.
	DefaultBaseURLForTorrentSearch = "https://tpirbay.site/s/" // Example, as per curl
	defaultTorrentSearchOrderBy    = "99"                      // Common default for seeders desc
	// defaultTorrentSearchPage       = 0                         // Page is 0-indexed
)

// ConcreteTorrentSearchService implements the TorrentSearcher interface.
type ConcreteTorrentSearchService struct {
	Client  *http.Client
	BaseURL string
}

// NewConcreteTorrentSearchService creates a new instance of ConcreteTorrentSearchService.
// If baseURL is empty, it uses DefaultBaseURLForTorrentSearch.
func NewConcreteTorrentSearchService(baseURL string) *ConcreteTorrentSearchService {
	if baseURL == "" {
		baseURL = DefaultBaseURLForTorrentSearch
	}
	return &ConcreteTorrentSearchService{
		Client: &http.Client{
			Timeout: 20 * time.Second, // Reasonalble timeout for external HTTP calls
		},
		BaseURL: baseURL,
	}
}

// SearchTorrents fetches torrents from the configured torrent site based on the query.
func (s *ConcreteTorrentSearchService) SearchTorrents(ctx context.Context, query string, page int, orderBy string) ([]TorrentSearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}
	if orderBy == "" {
		orderBy = defaultTorrentSearchOrderBy
	}

	reqURL, err := url.Parse(s.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL '%s': %w", s.BaseURL, err)
	}

	params := url.Values{}
	params.Add("q", query)
	params.Add("page", strconv.Itoa(page))
	params.Add("orderby", orderBy)
	reqURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers based on the provided curl command
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Dnt", "1")
	req.Header.Set("Priority", "u=0, i")

	// Derive Referer from BaseURL (e.g., "https://tpirbay.site/")
	parsedBaseURL, err := url.Parse(s.BaseURL)
	if err == nil && parsedBaseURL.Scheme != "" && parsedBaseURL.Host != "" {
		refererURL := &url.URL{Scheme: parsedBaseURL.Scheme, Host: parsedBaseURL.Host}
		req.Header.Set("Referer", refererURL.String()+"/")
	} else {
		// Fallback or log warning if BaseURL is unusual
		// For now, we'll proceed without Referer if BaseURL is malformed for this purpose
	}

	req.Header.Set("Sec-Ch-Ua", `"Not:A-Brand";v="24", "Chromium";v="134"`) // Consider making these configurable
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`) // This is quite specific
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin") // Assumes referer is from the same base domain
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36")

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request to torrent site: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) // Attempt to read body for more context
		return nil, fmt.Errorf("torrent site request failed with status %s: %s", resp.Status, string(bodyBytes))
	}

	return s.parseHTMLResults(resp.Body)
}

// parseHTMLResults parses the HTML from the reader and extracts torrent information.
// This parser is specifically tailored for sites like tpirbay.site (table with id="searchResult").
func (s *ConcreteTorrentSearchService) parseHTMLResults(body io.Reader) ([]TorrentSearchResult, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []TorrentSearchResult
	var findTableAndProcessRows func(*html.Node)

	findTableAndProcessRows = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			isSearchResultTable := false
			for _, attr := range n.Attr {
				if attr.Key == "id" && attr.Val == "searchResult" {
					isSearchResultTable = true
					break
				}
			}
			if isSearchResultTable {
				// Found the table, now process its rows (typically within <tbody>)
				for tbodyNode := n.FirstChild; tbodyNode != nil; tbodyNode = tbodyNode.NextSibling {
					if tbodyNode.Type == html.ElementNode && tbodyNode.Data == "tbody" {
						for trNode := tbodyNode.FirstChild; trNode != nil; trNode = trNode.NextSibling {
							if trNode.Type == html.ElementNode && trNode.Data == "tr" {
								title, magnetURL := s.extractTorrentDataFromRow(trNode)
								if title != "" && magnetURL != "" {
									results = append(results, TorrentSearchResult{Title: title, MagnetURL: magnetURL})
								}
							}
						}
					}
				}
				return // Processed the target table, no need to search further in its children
			}
		}
		// Recurse
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTableAndProcessRows(c)
		}
	}

	findTableAndProcessRows(doc)
	return results, nil
}

// extractTorrentDataFromRow scans a <tr> node for torrent title and magnet link.
func (s *ConcreteTorrentSearchService) extractTorrentDataFromRow(trNode *html.Node) (title string, magnetURL string) {
	var findLinks func(*html.Node)
	findLinks = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			isDetLink := false
			hrefVal := ""
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					hrefVal = attr.Val
				}
				if attr.Key == "class" {
					// Check if "detLink" is one of the classes (e.g. "detLink someOtherClass")
					classes := strings.Fields(attr.Val)
					for _, cls := range classes {
						if cls == "detLink" {
							isDetLink = true
							break
						}
					}
				}
			}

			if isDetLink && title == "" { // Capture the first detLink's text as title
				title = strings.TrimSpace(extractText(n))
			}
			if strings.HasPrefix(hrefVal, "magnet:") && magnetURL == "" { // Capture the first magnet link
				magnetURL = hrefVal
			}
		}

		// If both found, can potentially optimize by not recursing further in this branch,
		// but simple recursion is fine for typical row complexity.
		if title != "" && magnetURL != "" {
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findLinks(c)
			if title != "" && magnetURL != "" { // Propagate early exit
				return
			}
		}
	}

	findLinks(trNode) // Search within the provided table row
	return
}

// extractText recursively extracts all text from an HTML node and its children,
// skipping script and style contents.
func extractText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	if n.Type == html.CommentNode || (n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style")) {
		return ""
	}
	if n.Type != html.ElementNode {
		return ""
	}

	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(extractText(c))
	}
	return sb.String()
}
