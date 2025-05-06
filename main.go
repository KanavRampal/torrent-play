package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"torrent-play/config"   // Adjust import path
	"torrent-play/handlers" // Adjust import path
	"torrent-play/services" // Adjust import path

	"github.com/anacrolix/torrent"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	appConfig := config.LoadConfig()

	// Ensure data directory exists
	if err := os.MkdirAll(appConfig.DataDir, 0750); err != nil {
		log.Fatalf("Error creating data directory %s: %v", appConfig.DataDir, err)
	}

	// Create a single torrent client
	clientConfig := torrent.NewDefaultClientConfig()
	clientConfig.DataDir = appConfig.DataDir
	// clientConfig.Debug = true // Enable for more verbose logging
	client, err := torrent.NewClient(clientConfig)
	if err != nil {
		log.Fatalf("Error creating torrent client: %v", err)
	}
	defer client.Close()
	log.Println("Torrent client started.")

	// Create HLS service
	hlsService, err := services.NewHlsService(client, appConfig.ListenAddr)
	if err != nil {
		log.Fatalf("Error creating HLS service: %v", err)
	}
	defer hlsService.Cleanup()

	// Setup handlers
	torrentHandler := &handlers.TorrentHandler{HlsService: hlsService, ListenAddr: appConfig.ListenAddr}

	mux := http.NewServeMux()
	mux.HandleFunc("/add", torrentHandler.AddTorrentHandler)
	mux.HandleFunc("/hls/", hlsService.ServeHTTP) // HLS service handles requests under /hls/
	mux.HandleFunc("/search", handlers.NewSearchHandler(services.NewConcreteImdbService(appConfig.ImdbAPIKey)).SearchMoviesHandler)

	log.Printf("Starting HTTP server on http://%s", appConfig.ListenAddr)

	// Graceful shutdown
	go func() {
		if err := http.ListenAndServe(appConfig.ListenAddr, mux); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for termination signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("Shutting down server...")
	// Add context with timeout for graceful shutdown if needed
	// server.Shutdown(context.Background())
}
