package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"torrent-play/services" // Adjust import path if needed
)

type TorrentHandler struct {
	HlsService *services.HlsService
	ListenAddr string
}

func (h *TorrentHandler) AddTorrentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	magnetURI := r.URL.Query().Get("magnet")
	if magnetURI == "" {
		http.Error(w, "Missing 'magnet' query parameter", http.StatusBadRequest)
		return
	}

	log.Printf("Received request to add magnet: %s", magnetURI)

	streamInfo, err := h.HlsService.PrepareStream(r.Context(), magnetURI)
	if err != nil {
		log.Printf("Error preparing stream: %v", err)
		http.Error(w, fmt.Sprintf("Error preparing stream: %v", err), http.StatusInternalServerError)
		return
	}

	hlsURL := fmt.Sprintf("http://%s/hls/%s/playlist.m3u8", h.ListenAddr, streamInfo.ID)
	log.Printf("Stream %s prepared. HLS URL: %s", streamInfo.ID, hlsURL)

	// Respond with the stream info (including the HLS URL)
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"streamId": streamInfo.ID,
		"hlsUrl":   hlsURL,
		"status":   string(streamInfo.State),
	}
	json.NewEncoder(w).Encode(response)
}
