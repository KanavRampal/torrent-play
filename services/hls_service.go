package services

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
)

type StreamState string

const (
	StateInitializing StreamState = "initializing"
	StateGettingInfo  StreamState = "getting_info"
	StateDownloading  StreamState = "downloading"
	StateTranscoding  StreamState = "transcoding"
	StateReady        StreamState = "ready"
	StateError        StreamState = "error"
)

type StreamInfo struct {
	ID        string
	MagnetURI string
	State     StreamState
	HlsDir    string
	Error     error
	Torrent   *torrent.Torrent
	File      *torrent.File
}

type HlsService struct {
	client      *torrent.Client
	streams     map[string]*StreamInfo
	mu          sync.RWMutex
	baseTempDir string
	listenAddr  string
}

func NewHlsService(client *torrent.Client, listenAddr string) (*HlsService, error) {
	tempDir, err := os.MkdirTemp("", "torrent-hls-service")
	if err != nil {
		return nil, fmt.Errorf("failed to create base temp dir: %w", err)
	}
	log.Printf("Created base temporary directory: %s", tempDir)

	return &HlsService{
		client:      client,
		streams:     make(map[string]*StreamInfo),
		baseTempDir: tempDir,
		listenAddr:  listenAddr,
	}, nil
}

func (s *HlsService) Cleanup() {
	os.RemoveAll(s.baseTempDir)
	log.Printf("Removed base temporary directory: %s", s.baseTempDir)
}

// PrepareStream adds a torrent and starts the process to make it streamable via HLS.
func (s *HlsService) PrepareStream(ctx context.Context, magnetURI string) (*StreamInfo, error) {
	s.mu.Lock()
	// Simple ID generation for example purposes. Use something more robust in production.
	streamID := fmt.Sprintf("%d", time.Now().UnixNano())
	info := &StreamInfo{
		ID:        streamID,
		MagnetURI: magnetURI,
		State:     StateInitializing,
	}
	s.streams[streamID] = info
	s.mu.Unlock()

	log.Printf("[%s] Adding magnet: %s", streamID, magnetURI)
	t, err := s.client.AddMagnet(magnetURI)
	if err != nil {
		s.updateStreamState(streamID, StateError, fmt.Errorf("error adding magnet: %w", err))
		return info, info.Error
	}
	info.Torrent = t
	s.updateStreamState(streamID, StateGettingInfo, nil)

	go s.manageStream(ctx, streamID, t)

	return info, nil
}

func (s *HlsService) GetStreamInfo(streamID string) (*StreamInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.streams[streamID]
	return info, ok
}

func (s *HlsService) updateStreamState(streamID string, state StreamState, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if info, ok := s.streams[streamID]; ok {
		info.State = state
		info.Error = err
		if err != nil {
			log.Printf("[%s] Error state: %v", streamID, err)
		} else {
			log.Printf("[%s] State changed to: %s", streamID, state)
		}
	}
}

func (s *HlsService) manageStream(ctx context.Context, streamID string, t *torrent.Torrent) {
	<-t.GotInfo() // Wait for the torrent to get info
	if t.Info() == nil {
		s.updateStreamState(streamID, StateError, fmt.Errorf("torrent info not available"))
		return
	}

	// Select the largest file
	var largestFile *torrent.File
	for _, file := range t.Files() {
		if largestFile == nil || file.Length() > largestFile.Length() {
			largestFile = file
		}
	}
	if largestFile == nil {
		s.updateStreamState(streamID, StateError, fmt.Errorf("no files found in torrent"))
		return
	}
	log.Printf("[%s] Selected largest file: %s (%d bytes)", streamID, largestFile.Path(), largestFile.Length())

	s.mu.Lock()
	s.streams[streamID].File = largestFile
	s.mu.Unlock()

	s.updateStreamState(streamID, StateDownloading, nil)
	// In a real scenario, you might wait for a certain percentage or amount here.
	// For simplicity, we'll proceed directly to transcoding, relying on the reader to block.
	// largestFile.Download() // Prioritize this file

	// Create HLS directory
	hlsDir, err := os.MkdirTemp(s.baseTempDir, fmt.Sprintf("hls-%s-", streamID))
	if err != nil {
		s.updateStreamState(streamID, StateError, fmt.Errorf("failed to create HLS temp dir: %w", err))
		return
	}
	log.Printf("[%s] Created HLS directory: %s", streamID, hlsDir)

	s.mu.Lock()
	s.streams[streamID].HlsDir = hlsDir
	s.mu.Unlock()

	s.updateStreamState(streamID, StateTranscoding, nil)

	// Start transcoding (simplified error handling)
	err = s.transcodeToHLS(ctx, streamID, largestFile, hlsDir)
	if err != nil {
		s.updateStreamState(streamID, StateError, fmt.Errorf("transcoding failed: %w", err))
		os.RemoveAll(hlsDir) // Clean up failed transcoding attempt
		return
	}

	s.updateStreamState(streamID, StateReady, nil)

	// Schedule cleanup (optional)
	// go func() {
	// 	time.Sleep(30 * time.Minute)
	// 	s.mu.Lock()
	// 	delete(s.streams, streamID)
	// 	s.mu.Unlock()
	// 	os.RemoveAll(hlsDir)
	// 	log.Printf("[%s] Cleaned up HLS directory: %s", streamID, hlsDir)
	// 	// Consider dropping the torrent if no longer needed: t.Drop()
	// }()
}

// ServeHTTP makes HlsService serve the HLS files.
func (s *HlsService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Expecting paths like /hls/{streamID}/playlist.m3u8 or /hls/{streamID}/segmentXX.ts
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 3)
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	streamID := parts[1]
	fileName := parts[2]

	s.mu.RLock()
	stream, ok := s.streams[streamID]
	s.mu.RUnlock()

	if !ok { // Allow serving while transcoding
		log.Printf("Stream not found or not ready: %s", streamID)
		http.NotFound(w, r)
		return
	}

	// Security: Ensure fileName doesn't contain path traversal elements.
	if strings.Contains(fileName, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(stream.HlsDir, fileName)
	// log.Printf("[%s] Serving file: %s", streamID, filePath) // Can be noisy

	// Set CORS headers to allow playback in browsers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	http.ServeFile(w, r, filePath)
}

func (s *HlsService) transcodeToHLS(ctx context.Context, streamID string, file *torrent.File, hlsDir string) error {
	fileReader := file.NewReader()
	defer fileReader.Close() // Ensure reader is closed eventually

	playlistPath := filepath.Join(hlsDir, "playlist.m3u8")
	segmentPattern := filepath.Join(hlsDir, "segment%03d.ts")

	// Ensure ffmpeg is in PATH or provide the full path
	cmd := exec.Command("ffmpeg",
		"-i", "pipe:0", // Read from stdin
		"-c:v", "libx264", // Example codec, adjust as needed
		"-c:a", "aac", // Example codec, adjust as needed
		"-f", "hls",
		"-hls_time", "10", // 10-second segments
		"-hls_list_size", "0", // Keep all segments in the playlist
		"-hls_segment_filename", segmentPattern,
		playlistPath,
	)

	cmd.Stdin = fileReader // Pipe the torrent file reader to ffmpeg's stdin

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe for ffmpeg: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting ffmpeg: %w", err)
	}

	// Log ffmpeg output
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("ffmpeg [%s]: %s", streamID, line)
			// Basic error detection
			if strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "failed") {
				log.Printf("Error detected in ffmpeg output for stream %s", streamID)
				// Consider killing the process if a fatal error is detected
				// cmd.Process.Kill()
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading ffmpeg stderr for stream %s: %v", streamID, err)
		}
	}()

	log.Printf("[%s] Waiting for ffmpeg to finish...", streamID)
	err = cmd.Wait()
	if err != nil {
		// Check if the error is due to context cancellation
		if ctx.Err() != nil {
			return fmt.Errorf("ffmpeg stopped due to context cancellation: %w", ctx.Err())
		}
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}

	log.Printf("[%s] ffmpeg finished successfully.", streamID)
	return nil
}
