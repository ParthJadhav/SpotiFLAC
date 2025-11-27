package backend

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-flac/flacpicture"
	"github.com/go-flac/flacvorbis"
	"github.com/go-flac/go-flac"
)

// DownloadRequest represents the request structure for downloading tracks
type DownloadRequest struct {
	ISRC                string `json:"isrc"`
	Service             string `json:"service"`
	Query               string `json:"query,omitempty"`
	TrackName           string `json:"track_name,omitempty"`
	ArtistName          string `json:"artist_name,omitempty"`
	AlbumName           string `json:"album_name,omitempty"`
	ApiURL              string `json:"api_url,omitempty"`
	OutputDir           string `json:"output_dir,omitempty"`
	AudioFormat         string `json:"audio_format,omitempty"`
	FilenameFormat      string `json:"filename_format,omitempty"`
	TrackNumber         bool   `json:"track_number,omitempty"`
	Position            int    `json:"position,omitempty"`               // Position in playlist/album (1-based)
	UseAlbumTrackNumber bool   `json:"use_album_track_number,omitempty"` // Use album track number instead of playlist position
	SpotifyID           string `json:"spotify_id,omitempty"`             // Spotify track ID
	ServiceURL          string `json:"service_url,omitempty"`            // Direct service URL (Tidal/Deezer/Amazon) to skip song.link API call
	Duration            int    `json:"duration,omitempty"`               // Track duration in seconds for better matching
}

type Metadata struct {
	Title       string
	Artist      string
	Album       string
	Date        string
	TrackNumber int
	DiscNumber  int
	ISRC        string
}

func EmbedMetadata(filepath string, metadata Metadata, coverPath string) error {
	f, err := flac.ParseFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to parse FLAC file: %w", err)
	}

	var cmtIdx = -1
	for idx, block := range f.Meta {
		if block.Type == flac.VorbisComment {
			cmtIdx = idx
			break
		}
	}

	cmt := flacvorbis.New()

	if metadata.Title != "" {
		_ = cmt.Add(flacvorbis.FIELD_TITLE, metadata.Title)
	}
	if metadata.Artist != "" {
		_ = cmt.Add(flacvorbis.FIELD_ARTIST, metadata.Artist)
	}
	if metadata.Album != "" {
		_ = cmt.Add(flacvorbis.FIELD_ALBUM, metadata.Album)
	}
	if metadata.Date != "" {
		_ = cmt.Add(flacvorbis.FIELD_DATE, metadata.Date)
	}
	if metadata.TrackNumber > 0 {
		_ = cmt.Add(flacvorbis.FIELD_TRACKNUMBER, strconv.Itoa(metadata.TrackNumber))
	}
	if metadata.DiscNumber > 0 {
		_ = cmt.Add("DISCNUMBER", strconv.Itoa(metadata.DiscNumber))
	}
	if metadata.ISRC != "" {
		_ = cmt.Add(flacvorbis.FIELD_ISRC, metadata.ISRC)
	}

	cmtBlock := cmt.Marshal()
	if cmtIdx < 0 {
		f.Meta = append(f.Meta, &cmtBlock)
	} else {
		f.Meta[cmtIdx] = &cmtBlock
	}

	if coverPath != "" && fileExists(coverPath) {
		if err := embedCoverArt(f, coverPath); err != nil {
			fmt.Printf("Warning: Failed to embed cover art: %v\n", err)
		}
	}

	if err := f.Save(filepath); err != nil {
		return fmt.Errorf("failed to save FLAC file: %w", err)
	}

	return nil
}

func embedCoverArt(f *flac.File, coverPath string) error {
	imgData, err := os.ReadFile(coverPath)
	if err != nil {
		return fmt.Errorf("failed to read cover image: %w", err)
	}

	picture, err := flacpicture.NewFromImageData(
		flacpicture.PictureTypeFrontCover,
		"Cover",
		imgData,
		"image/jpeg",
	)
	if err != nil {
		return fmt.Errorf("failed to create picture block: %w", err)
	}

	pictureBlock := picture.Marshal()

	for i := len(f.Meta) - 1; i >= 0; i-- {
		if f.Meta[i].Type == flac.Picture {
			f.Meta = append(f.Meta[:i], f.Meta[i+1:]...)
		}
	}

	f.Meta = append(f.Meta, &pictureBlock)

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadISRCFromFile reads ISRC metadata from a FLAC file
func ReadISRCFromFile(filepath string) (string, error) {
	if !fileExists(filepath) {
		return "", fmt.Errorf("file does not exist")
	}

	f, err := flac.ParseFile(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to parse FLAC file: %w", err)
	}

	// Find VorbisComment block
	for _, block := range f.Meta {
		if block.Type == flac.VorbisComment {
			cmt, err := flacvorbis.ParseFromMetaDataBlock(*block)
			if err != nil {
				continue
			}

			// Get ISRC field
			isrcValues, err := cmt.Get(flacvorbis.FIELD_ISRC)
			if err == nil && len(isrcValues) > 0 {
				return isrcValues[0], nil
			}
		}
	}

	return "", nil // No ISRC found
}

// CheckFilenameExists checks if a file with the given filename already exists in the directory
func CheckFilenameExists(outputDir string, expectedFilename string) (string, bool) {
	if expectedFilename == "" {
		return "", false
	}

	// Construct the full path to the expected file
	expectedPath := fmt.Sprintf("%s/%s", outputDir, expectedFilename)
	
	// Check if file exists and has content (size > 0)
	if info, err := os.Stat(expectedPath); err == nil && info.Size() > 0 {
		return expectedPath, true
	}

	return "", false
}

// CheckISRCExists checks if a file with the given ISRC already exists in the directory
// DEPRECATED: Use CheckFilenameExists instead for better performance
func CheckISRCExists(outputDir string, targetISRC string) (string, bool) {
	if targetISRC == "" {
		return "", false
	}

	// Read all .flac files in directory
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return "", false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check only .flac files
		filename := entry.Name()
		if len(filename) < 5 || filename[len(filename)-5:] != ".flac" {
			continue
		}

		filepath := fmt.Sprintf("%s/%s", outputDir, filename)

		// Read ISRC from file
		isrc, err := ReadISRCFromFile(filepath)
		if err != nil {
			continue
		}

		// Compare ISRC (case-insensitive)
		if isrc != "" && strings.EqualFold(isrc, targetISRC) {
			return filepath, true
		}
	}

	return "", false
}

// ExistingTrackInfo represents a track that already exists
type ExistingTrackInfo struct {
	ISRC       string  `json:"isrc"`
	Filename   string  `json:"filename"`
	Path       string  `json:"path"`
	SizeGB     float64 `json:"size_gb"`
	Completed  bool    `json:"completed"`
}

// TrackCheckRequest represents a track to check for existence
type TrackCheckRequest struct {
	ISRC       string `json:"isrc"`
	TrackName  string `json:"track_name"`
	ArtistName string `json:"artist_name"`
	AlbumName  string `json:"album_name"`
	Position   int    `json:"position"`
}

// CheckFilesExistBatch checks multiple files for existence primarily by filename, with ISRC as fallback
// Returns list of existing files and their paths for fast pre-download checks
func CheckFilesExistBatch(outputDir string, tracks []TrackCheckRequest, filenameFormat string, includeTrackNumber, useAlbumTrackNumber bool) []ExistingTrackInfo {
	if len(tracks) == 0 {
		return []ExistingTrackInfo{}
	}

	// Read all .flac files in directory once
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return []ExistingTrackInfo{}
	}

	// Index files by filename for fast lookup
	filenameIndex := make(map[string]ExistingTrackInfo)
	isrcIndex := make(map[string]ExistingTrackInfo)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if len(filename) < 5 || filename[len(filename)-5:] != ".flac" {
			continue
		}

		filepath := fmt.Sprintf("%s/%s", outputDir, filename)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Always index by filename (primary method)
		trackInfo := ExistingTrackInfo{
			Filename: filename,
			Path:     filepath,
			SizeGB:   float64(info.Size()) / (1024 * 1024 * 1024),
			Completed: true, // File exists, so it's completed
		}

		// Try to read ISRC from file for additional indexing
		if isrc, err := ReadISRCFromFile(filepath); err == nil && isrc != "" {
			trackInfo.ISRC = isrc
			// Index by ISRC (case-insensitive) as secondary method
			isrcKey := strings.ToLower(isrc)
			isrcIndex[isrcKey] = trackInfo
		}

		// Primary index by filename
		filenameIndex[filename] = trackInfo
	}

	// Check each track
	var existing []ExistingTrackInfo

	for _, track := range tracks {
		// PRIMARY: Try filename matching first (faster and more reliable)
		if track.TrackName != "" && track.ArtistName != "" {
			expectedFilename := BuildExpectedFilename(track.TrackName, track.ArtistName, filenameFormat, includeTrackNumber, track.Position, useAlbumTrackNumber)
			if found, exists := filenameIndex[expectedFilename]; exists {
				existing = append(existing, found)
				continue
			}
		}

		// FALLBACK: Try ISRC matching only if filename matching failed
		if track.ISRC != "" {
			isrcKey := strings.ToLower(track.ISRC)
			if found, exists := isrcIndex[isrcKey]; exists {
				existing = append(existing, found)
				continue
			}
		}
	}

	return existing
}

// MarkTrackCompleted marks a track as completed by creating a completion marker
func MarkTrackCompleted(outputDir string, trackName, artistName, filenameFormat string, includeTrackNumber bool, position int, useAlbumTrackNumber bool) error {
	if trackName == "" || artistName == "" {
		return fmt.Errorf("track name and artist name are required")
	}

	expectedFilename := BuildExpectedFilename(trackName, artistName, filenameFormat, includeTrackNumber, position, useAlbumTrackNumber)
	completionMarkerPath := fmt.Sprintf("%s/.%s.completed", outputDir, strings.TrimSuffix(expectedFilename, ".flac"))

	// Create the completion marker file
	file, err := os.Create(completionMarkerPath)
	if err != nil {
		return fmt.Errorf("failed to create completion marker: %w", err)
	}
	defer file.Close()

	// Write timestamp to the marker file
	_, err = file.WriteString(fmt.Sprintf("completed_at=%d\n", time.Now().Unix()))
	if err != nil {
		return fmt.Errorf("failed to write completion marker: %w", err)
	}

	return nil
}

// IsTrackCompleted checks if a track has been marked as completed
func IsTrackCompleted(outputDir string, trackName, artistName, filenameFormat string, includeTrackNumber bool, position int, useAlbumTrackNumber bool) bool {
	if trackName == "" || artistName == "" {
		return false
	}

	expectedFilename := BuildExpectedFilename(trackName, artistName, filenameFormat, includeTrackNumber, position, useAlbumTrackNumber)
	completionMarkerPath := fmt.Sprintf("%s/.%s.completed", outputDir, strings.TrimSuffix(expectedFilename, ".flac"))

	// Check if completion marker exists
	_, err := os.Stat(completionMarkerPath)
	return err == nil
}

// ClearTrackCompletion removes the completion marker for a track
func ClearTrackCompletion(outputDir string, trackName, artistName, filenameFormat string, includeTrackNumber bool, position int, useAlbumTrackNumber bool) error {
	if trackName == "" || artistName == "" {
		return fmt.Errorf("track name and artist name are required")
	}

	expectedFilename := BuildExpectedFilename(trackName, artistName, filenameFormat, includeTrackNumber, position, useAlbumTrackNumber)
	completionMarkerPath := fmt.Sprintf("%s/.%s.completed", outputDir, strings.TrimSuffix(expectedFilename, ".flac"))

	// Remove the completion marker file
	err := os.Remove(completionMarkerPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove completion marker: %w", err)
	}

	return nil
}
