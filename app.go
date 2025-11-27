package main

import (
	"context"
	"encoding/json"
	"fmt"
	"spotiflac/backend"
	"strings"
	"sync"
	"time"
)

// App struct
type App struct {
	ctx                    context.Context
	parallelDownloadLock   sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SpotifyMetadataRequest represents the request structure for fetching Spotify metadata
type SpotifyMetadataRequest struct {
	URL     string  `json:"url"`
	Batch   bool    `json:"batch"`
	Delay   float64 `json:"delay"`
	Timeout float64 `json:"timeout"`
}

// DownloadResponse represents the response structure for download operations
type DownloadResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	File          string `json:"file,omitempty"`
	Error         string `json:"error,omitempty"`
	AlreadyExists bool   `json:"already_exists,omitempty"`
}

// GetStreamingURLs fetches all streaming URLs from song.link API
func (a *App) GetStreamingURLs(spotifyTrackID string) (string, error) {
	if spotifyTrackID == "" {
		return "", fmt.Errorf("spotify track ID is required")
	}

	fmt.Printf("[GetStreamingURLs] Called for track ID: %s\n", spotifyTrackID)
	client := backend.NewSongLinkClient()
	urls, err := client.GetAllURLsFromSpotify(spotifyTrackID)
	if err != nil {
		return "", err
	}

	jsonData, err := json.Marshal(urls)
	if err != nil {
		return "", fmt.Errorf("failed to encode response: %v", err)
	}

	return string(jsonData), nil
}

// GetSpotifyMetadata fetches metadata from Spotify
func (a *App) GetSpotifyMetadata(req SpotifyMetadataRequest) (string, error) {
	if req.URL == "" {
		return "", fmt.Errorf("URL parameter is required")
	}

	if req.Delay == 0 {
		req.Delay = 1.0
	}
	if req.Timeout == 0 {
		req.Timeout = 300.0
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Timeout*float64(time.Second)))
	defer cancel()

	data, err := backend.GetFilteredSpotifyData(ctx, req.URL, req.Batch, time.Duration(req.Delay*float64(time.Second)))
	if err != nil {
		return "", fmt.Errorf("failed to fetch metadata: %v", err)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode response: %v", err)
	}

	return string(jsonData), nil
}

// DownloadTrack downloads a track by ISRC
func (a *App) DownloadTrack(req backend.DownloadRequest) (DownloadResponse, error) {
	if req.ISRC == "" {
		return DownloadResponse{
			Success: false,
			Error:   "ISRC is required",
		}, fmt.Errorf("ISRC is required")
	}

	if req.Service == "" {
		req.Service = "deezer"
	}

	if req.OutputDir == "" {
		req.OutputDir = "."
	}

	if req.AudioFormat == "" {
		req.AudioFormat = "LOSSLESS"
	}

	var err error
	var filename string

	// Set default filename format if not provided
	if req.FilenameFormat == "" {
		req.FilenameFormat = "title-artist"
	}

	// Primary check: Check if file with expected filename already exists
	if req.TrackName != "" && req.ArtistName != "" {
		expectedFilename := backend.BuildExpectedFilename(req.TrackName, req.ArtistName, req.FilenameFormat, req.TrackNumber, req.Position, req.UseAlbumTrackNumber)
		if existingFile, exists := backend.CheckFilenameExists(req.OutputDir, expectedFilename); exists {
			fmt.Printf("File with filename %s already exists: %s\n", expectedFilename, existingFile)
			return DownloadResponse{
				Success:       true,
				Message:       "File already exists",
				File:          existingFile,
				AlreadyExists: true,
			}, nil
		}
	}

	// Fallback check: Check if file with same ISRC already exists (for backward compatibility)
	if existingFile, exists := backend.CheckISRCExists(req.OutputDir, req.ISRC); exists {
		fmt.Printf("File with ISRC %s already exists: %s\n", req.ISRC, existingFile)
		return DownloadResponse{
			Success:       true,
			Message:       "File with same ISRC already exists (found by ISRC)",
			File:          existingFile,
			AlreadyExists: true,
		}, nil
	}

	// Set downloading state
	backend.SetDownloading(true)
	defer backend.SetDownloading(false)

	// Verbose logging for debugging
	fmt.Printf("\n=== DOWNLOAD START ===\n")
	fmt.Printf("ISRC: %s\n", req.ISRC)
	fmt.Printf("Service: %s\n", req.Service)
	fmt.Printf("Track: %s - %s\n", req.TrackName, req.ArtistName)
	fmt.Printf("Album: %s\n", req.AlbumName)
	fmt.Printf("Spotify ID: %s\n", req.SpotifyID)
	fmt.Printf("Service URL: %s\n", req.ServiceURL)
	fmt.Printf("Output Dir: %s\n", req.OutputDir)
	fmt.Printf("Duration: %d ms\n", req.Duration)
	fmt.Printf("======================\n")

	switch req.Service {
	case "amazon":
		downloader := backend.NewAmazonDownloader()
		if req.ServiceURL != "" {
			// Use provided URL directly
			filename, err = downloader.DownloadByURL(req.ServiceURL, req.OutputDir, req.FilenameFormat, req.TrackNumber, req.Position, req.TrackName, req.ArtistName, req.AlbumName, req.UseAlbumTrackNumber)
		} else {
			if req.SpotifyID == "" {
				return DownloadResponse{
					Success: false,
					Error:   "Spotify ID is required for Amazon Music",
				}, fmt.Errorf("spotify ID is required for Amazon Music")
			}
			filename, err = downloader.DownloadBySpotifyID(req.SpotifyID, req.OutputDir, req.FilenameFormat, req.TrackNumber, req.Position, req.TrackName, req.ArtistName, req.AlbumName, req.UseAlbumTrackNumber)
		}

	case "tidal":
		if req.ApiURL == "" || req.ApiURL == "auto" {
			downloader := backend.NewTidalDownloader("")
			if req.ServiceURL != "" {
				// Use provided URL directly with fallback to multiple APIs
				filename, err = downloader.DownloadByURLWithFallback(req.ServiceURL, req.OutputDir, req.AudioFormat, req.FilenameFormat, req.TrackNumber, req.Position, req.TrackName, req.ArtistName, req.AlbumName, req.UseAlbumTrackNumber)
			} else {
				if req.SpotifyID == "" {
					return DownloadResponse{
						Success: false,
						Error:   "Spotify ID is required for Tidal",
					}, fmt.Errorf("spotify ID is required for Tidal")
				}
				// Use ISRC matching for search fallback
				filename, err = downloader.DownloadWithFallbackAndISRC(req.SpotifyID, req.ISRC, req.OutputDir, req.AudioFormat, req.FilenameFormat, req.TrackNumber, req.Position, req.TrackName, req.ArtistName, req.AlbumName, req.UseAlbumTrackNumber, req.Duration)
			}
		} else {
			downloader := backend.NewTidalDownloader(req.ApiURL)
			if req.ServiceURL != "" {
				// Use provided URL directly with specific API
				filename, err = downloader.DownloadByURL(req.ServiceURL, req.OutputDir, req.AudioFormat, req.FilenameFormat, req.TrackNumber, req.Position, req.TrackName, req.ArtistName, req.AlbumName, req.UseAlbumTrackNumber)
			} else {
				if req.SpotifyID == "" {
					return DownloadResponse{
						Success: false,
						Error:   "Spotify ID is required for Tidal",
					}, fmt.Errorf("spotify ID is required for Tidal")
				}
				// Use ISRC matching for search fallback
				filename, err = downloader.DownloadWithISRC(req.SpotifyID, req.ISRC, req.OutputDir, req.AudioFormat, req.FilenameFormat, req.TrackNumber, req.Position, req.TrackName, req.ArtistName, req.AlbumName, req.UseAlbumTrackNumber, req.Duration)
			}
		}

	case "qobuz":
		downloader := backend.NewQobuzDownloader()
		filename, err = downloader.DownloadByISRC(req.ISRC, req.OutputDir, req.AudioFormat, req.FilenameFormat, req.TrackNumber, req.Position, req.TrackName, req.ArtistName, req.AlbumName, req.UseAlbumTrackNumber)

	default: // deezer
		downloader := backend.NewDeezerDownloader()
		if req.ServiceURL != "" {
			// Use provided URL directly
			filename, err = downloader.DownloadByURL(req.ServiceURL, req.OutputDir, req.FilenameFormat, req.TrackNumber, req.Position, req.TrackName, req.ArtistName, req.AlbumName, req.UseAlbumTrackNumber)
		} else {
			if req.SpotifyID == "" {
				return DownloadResponse{
					Success: false,
					Error:   "Spotify ID is required for Deezer",
				}, fmt.Errorf("spotify ID is required for Deezer")
			}
			filename, err = downloader.DownloadBySpotifyID(req.SpotifyID, req.OutputDir, req.FilenameFormat, req.TrackNumber, req.Position, req.TrackName, req.ArtistName, req.AlbumName, req.UseAlbumTrackNumber)
		}
	}

	if err != nil {
		fmt.Printf("\n=== DOWNLOAD FAILED ===\n")
		fmt.Printf("Error: %v\n", err)
		fmt.Printf("Service: %s\n", req.Service)
		fmt.Printf("Track: %s - %s\n", req.TrackName, req.ArtistName)
		fmt.Printf("ISRC: %s\n", req.ISRC)
		fmt.Printf("=======================\n")
		return DownloadResponse{
			Success: false,
			Error:   fmt.Sprintf("Download failed: %v", err),
		}, err
	}

	// Check if file already existed
	alreadyExists := false
	if strings.HasPrefix(filename, "EXISTS:") {
		alreadyExists = true
		filename = strings.TrimPrefix(filename, "EXISTS:")
		fmt.Printf("✓ File already existed: %s\n", filename)
	} else {
		fmt.Printf("✓ Download completed: %s\n", filename)
	}

	// Mark track as completed after successful download or if it already existed
	if req.TrackName != "" && req.ArtistName != "" {
		if err := backend.MarkTrackCompleted(req.OutputDir, req.TrackName, req.ArtistName, req.FilenameFormat, req.TrackNumber, req.Position, req.UseAlbumTrackNumber); err != nil {
			fmt.Printf("Warning: Failed to mark track as completed: %v\n", err)
		} else {
			fmt.Printf("✓ Track marked as completed\n")
		}
	}

	message := "Download completed successfully"
	if alreadyExists {
		message = "File already exists"
	}

	fmt.Printf("\n=== DOWNLOAD SUCCESS ===\n")
	fmt.Printf("Final file: %s\n", filename)
	fmt.Printf("Already existed: %v\n", alreadyExists)
	fmt.Printf("========================\n")

	return DownloadResponse{
		Success:       true,
		Message:       message,
		File:          filename,
		AlreadyExists: alreadyExists,
	}, nil
}

// OpenFolder opens a folder in the file explorer
func (a *App) OpenFolder(path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	err := backend.OpenFolderInExplorer(path)
	if err != nil {
		return fmt.Errorf("failed to open folder: %v", err)
	}

	return nil
}

// SelectFolder opens a folder selection dialog and returns the selected path
func (a *App) SelectFolder(defaultPath string) (string, error) {
	return backend.SelectFolderDialog(a.ctx, defaultPath)
}

// SelectFile opens a file selection dialog and returns the selected file path
func (a *App) SelectFile() (string, error) {
	return backend.SelectFileDialog(a.ctx)
}

// GetDefaults returns the default configuration
func (a *App) GetDefaults() map[string]string {
	return map[string]string{
		"downloadPath": backend.GetDefaultMusicPath(),
	}
}

// GetDownloadProgress returns current download progress
func (a *App) GetDownloadProgress() backend.ProgressInfo {
	return backend.GetDownloadProgress()
}

// Quit closes the application
func (a *App) Quit() {
	// You can add cleanup logic here if needed
	panic("quit") // This will trigger Wails to close the app
}

// AnalyzeTrack analyzes audio quality of a FLAC file
func (a *App) AnalyzeTrack(filePath string) (string, error) {
	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}

	result, err := backend.AnalyzeTrack(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to analyze track: %v", err)
	}

	jsonData, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to encode response: %v", err)
	}

	return string(jsonData), nil
}

// AnalyzeMultipleTracks analyzes multiple FLAC files
func (a *App) AnalyzeMultipleTracks(filePaths []string) (string, error) {
	if len(filePaths) == 0 {
		return "", fmt.Errorf("at least one file path is required")
	}

	results := make([]*backend.AnalysisResult, 0, len(filePaths))

	for _, filePath := range filePaths {
		result, err := backend.AnalyzeTrack(filePath)
		if err != nil {
			// Skip failed analyses
			continue
		}
		results = append(results, result)
	}

	jsonData, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to encode response: %v", err)
	}

	return string(jsonData), nil
}

// LyricsDownloadRequest represents the request structure for downloading lyrics
type LyricsDownloadRequest struct {
	SpotifyID           string `json:"spotify_id"`
	TrackName           string `json:"track_name"`
	ArtistName          string `json:"artist_name"`
	OutputDir           string `json:"output_dir"`
	FilenameFormat      string `json:"filename_format"`
	TrackNumber         bool   `json:"track_number"`
	Position            int    `json:"position"`
	UseAlbumTrackNumber bool   `json:"use_album_track_number"`
}

// DownloadLyrics downloads lyrics for a single track
func (a *App) DownloadLyrics(req LyricsDownloadRequest) (backend.LyricsDownloadResponse, error) {
	if req.SpotifyID == "" {
		return backend.LyricsDownloadResponse{
			Success: false,
			Error:   "Spotify ID is required",
		}, fmt.Errorf("spotify ID is required")
	}

	client := backend.NewLyricsClient()
	backendReq := backend.LyricsDownloadRequest{
		SpotifyID:           req.SpotifyID,
		TrackName:           req.TrackName,
		ArtistName:          req.ArtistName,
		OutputDir:           req.OutputDir,
		FilenameFormat:      req.FilenameFormat,
		TrackNumber:         req.TrackNumber,
		Position:            req.Position,
		UseAlbumTrackNumber: req.UseAlbumTrackNumber,
	}

	resp, err := client.DownloadLyrics(backendReq)
	if err != nil {
		return backend.LyricsDownloadResponse{
			Success: false,
			Error:   err.Error(),
		}, err
	}

	return *resp, nil
}

// CheckTrackAvailability checks the availability of a track on different streaming platforms
func (a *App) CheckTrackAvailability(spotifyTrackID string, isrc string) (string, error) {
	if spotifyTrackID == "" {
		return "", fmt.Errorf("spotify track ID is required")
	}

	client := backend.NewSongLinkClient()
	availability, err := client.CheckTrackAvailability(spotifyTrackID, isrc)
	if err != nil {
		return "", err
	}

	jsonData, err := json.Marshal(availability)
	if err != nil {
		return "", fmt.Errorf("failed to encode response: %v", err)
	}

	return string(jsonData), nil
}

// PreCheckDownloadRequest represents the request for pre-checking files
type PreCheckDownloadRequest struct {
	Tracks              []backend.TrackCheckRequest `json:"tracks"`
	OutputDir           string                      `json:"output_dir"`
	FilenameFormat      string                      `json:"filename_format"`
	TrackNumber         bool                        `json:"track_number"`
	UseAlbumTrackNumber bool                        `json:"use_album_track_number"`
}

// PreCheckDownloadFiles checks if multiple files already exist before downloading
// Returns list of existing files that can be skipped
func (a *App) PreCheckDownloadFiles(req PreCheckDownloadRequest) (string, error) {
	if len(req.Tracks) == 0 {
		return "[]", nil
	}

	if req.OutputDir == "" {
		req.OutputDir = "."
	}

	if req.FilenameFormat == "" {
		req.FilenameFormat = "title-artist"
	}

	existing := backend.CheckFilesExistBatch(req.OutputDir, req.Tracks, req.FilenameFormat, req.TrackNumber, req.UseAlbumTrackNumber)

	jsonData, err := json.Marshal(existing)
	if err != nil {
		return "", fmt.Errorf("failed to encode response: %v", err)
	}

	return string(jsonData), nil
}

