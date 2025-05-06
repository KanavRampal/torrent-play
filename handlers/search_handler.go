package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"torrent-play/services" // Adjust import path if needed
)

// SearchHandler handles requests for searching media.
// It relies on an ImdbService to perform the actual search operations.
type SearchHandler struct {
	ImdbService services.ImdbSearcher // Expects an ImdbSearcher from the services package
}

// NewSearchHandler creates and returns a new SearchHandler.
// It's a good practice to use constructors for initializing handlers with their dependencies.
func NewSearchHandler(imdbService services.ImdbSearcher) *SearchHandler {
	if imdbService == nil {
		// Depending on the application's needs, you might panic or log a fatal error.
		// For now, we'll assume a valid service is always provided.
		log.Println("Warning: ImdbService is nil during SearchHandler creation")
	}
	return &SearchHandler{
		ImdbService: imdbService,
	}
}

// SearchMoviesHandler handles GET requests to /search?q=<query>.
// It fetches movie/show details based on the query parameter using the ImdbService.
func (h *SearchHandler) SearchMoviesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed. Only GET is supported.", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing 'q' query parameter", http.StatusBadRequest)
		return
	}

	log.Printf("Received search query: %s", query)

	results, err := h.ImdbService.Search(r.Context(), query) // Pass request context
	if err != nil {
		log.Printf("Error searching IMDB via service: %v", err)
		http.Error(w, "Failed to fetch search results from IMDB.", http.StatusInternalServerError)
		return
	}

	// Ensure that a nil slice is encoded as an empty JSON array "[]" rather than "null"
	if results == nil {
		results = []services.SearchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		log.Printf("Error encoding search results to JSON: %v", err)
		// The header might have already been sent, so we can only log this server-side error.
	}
}
