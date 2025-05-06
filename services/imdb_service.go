package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// SearchResult defines the structure for a single search result item.
// This structure was implicitly defined by the SearchHandler previously.
type SearchResult struct {
	Title  string `json:"title"`
	Year   string `json:"year"`
	ImdbID string `json:"imdbId"`
	Type   string `json:"type"`   // e.g., "movie", "series", "episode"
	Poster string `json:"poster"` // URL to the poster image
}

// ImdbSearcher defines the interface for an IMDB search service.
type ImdbSearcher interface {
	Search(ctx context.Context, query string) ([]SearchResult, error)
}

const (
	omdbAPIBaseURL = "http://www.omdbapi.com/"
	// IMPORTANT: Replace "YOUR_OMDB_API_KEY" with your actual OMDb API key.
	// In a real application, this should be loaded from configuration (e.g., env variable).
	omdbAPIKey = "YOUR_OMDB_API_KEY"
)

// OMDbAPIResponse defines the structure for the top-level response from OMDb API.
type OMDbAPIResponse struct {
	Search       []OMDbSearchResult `json:"Search"`
	TotalResults string             `json:"totalResults"`
	Response     string             `json:"Response"` // "True" or "False"
	Error        string             `json:"Error"`    // Error message if Response is "False"
}

// OMDbSearchResult defines the structure for an individual item in OMDb's search results.
type OMDbSearchResult struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	ImdbID string `json:"imdbID"`
	Type   string `json:"Type"`
	Poster string `json:"Poster"`
}

// ConcreteImdbService is an implementation of ImdbSearcher using the OMDb API.
type ConcreteImdbService struct {
	Client  *http.Client
	BaseURL string
	APIKey  string
}

// NewConcreteImdbService creates a new instance of ConcreteImdbService.
func NewConcreteImdbService(apiKey string) *ConcreteImdbService {
	if apiKey == "" || apiKey == "YOUR_OMDB_API_KEY" {
		// In a real app, you might log a warning or error, or prevent startup.
		// For now, we'll proceed but it won't work without a valid key.
		fmt.Println("Warning: OMDb API key is not set or is using the default placeholder.")
	}
	return &ConcreteImdbService{
		Client: &http.Client{
			Timeout: 10 * time.Second, // Set a reasonable timeout
		},
		BaseURL: omdbAPIBaseURL,
		APIKey:  apiKey,
	}
}

// Search performs a search query against the OMDb API.
func (s *ConcreteImdbService) Search(ctx context.Context, query string) ([]SearchResult, error) {
	if s.APIKey == "" || s.APIKey == "YOUR_OMDB_API_KEY" {
		return nil, fmt.Errorf("OMDb API key is not configured")
	}

	// Construct the request URL
	reqURL, err := url.Parse(s.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}
	params := url.Values{}
	params.Add("s", query) // 's' is for search by title
	params.Add("apikey", s.APIKey)
	reqURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request to OMDb: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OMDb API request failed with status: %s", resp.Status)
	}

	var apiResp OMDbAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode OMDb API response: %w", err)
	}

	if apiResp.Response == "False" {
		if apiResp.Error == "Movie not found!" || apiResp.Error == "Series not found!" || apiResp.Error == "Incorrect IMDb ID." { // OMDb can return "Incorrect IMDb ID." for empty search.
			return []SearchResult{}, nil // No results found is not an error, return empty slice
		}
		return nil, fmt.Errorf("OMDb API error: %s", apiResp.Error)
	}

	results := make([]SearchResult, 0, len(apiResp.Search))
	for _, item := range apiResp.Search {
		results = append(results, SearchResult{
			Title:  item.Title,
			Year:   item.Year,
			ImdbID: item.ImdbID,
			Type:   item.Type,
			Poster: item.Poster,
		})
	}

	return results, nil
}
