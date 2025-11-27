package backend

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// DownloadJob represents a single download task
type DownloadJob struct {
	ID              string
	Request         DownloadRequest
	TrackIndex      int // Position in playlist
	TrackName       string
	ArtistName      string
	AlbumName       string
	PlaylistName    string
	UseAlbumTrackNo bool
}

// DownloadJobResult represents the result of a download job
type DownloadJobResult struct {
	JobID         string
	Success       bool
	Message       string
	FilePath      string
	Error         string
	AlreadyExists bool
	Duration      float64 // Time taken in seconds
}

// ParallelDownloadManager manages parallel downloads with configurable concurrency
type ParallelDownloadManager struct {
	maxConcurrent  int
	jobQueue       chan *DownloadJob
	resultChan     chan *DownloadJobResult
	wg             sync.WaitGroup
	workersStarted int32
	jobsCompleted  int32
	totalJobs      int32
	shutdown       bool
	shutdownLock   sync.Mutex
	downloadFunc   func(req DownloadRequest) (string, error)
	progressUpdate func(completed int32, total int32)
}

// NewParallelDownloadManager creates a new parallel download manager
func NewParallelDownloadManager(maxConcurrent int, downloadFunc func(req DownloadRequest) (string, error)) *ParallelDownloadManager {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	if maxConcurrent > 8 {
		maxConcurrent = 8 // Cap at 8 to avoid overwhelming the system
	}

	return &ParallelDownloadManager{
		maxConcurrent: maxConcurrent,
		downloadFunc:  downloadFunc,
		jobQueue:      make(chan *DownloadJob, maxConcurrent*2),
		resultChan:    make(chan *DownloadJobResult, maxConcurrent*2),
	}
}

// SetProgressCallback sets the callback for progress updates
func (pm *ParallelDownloadManager) SetProgressCallback(fn func(completed int32, total int32)) {
	pm.progressUpdate = fn
}

// Start starts the worker goroutines
func (pm *ParallelDownloadManager) Start() error {
	pm.shutdownLock.Lock()
	defer pm.shutdownLock.Unlock()

	if pm.shutdown {
		return fmt.Errorf("download manager is already shut down")
	}

	// Start worker goroutines
	for i := 0; i < pm.maxConcurrent; i++ {
		pm.wg.Add(1)
		go pm.worker(i)
		atomic.AddInt32(&pm.workersStarted, 1)
	}

	fmt.Printf("Started %d download workers\n", pm.maxConcurrent)
	return nil
}

// worker processes jobs from the queue
func (pm *ParallelDownloadManager) worker(id int) {
	defer pm.wg.Done()

	for job := range pm.jobQueue {
		result := &DownloadJobResult{
			JobID: job.ID,
		}

		fmt.Printf("[Worker %d] Processing: %s - %s\n", id, job.TrackName, job.ArtistName)

		// Execute download
		filename, err := pm.downloadFunc(job.Request)

		// Check if file already existed
		alreadyExists := false
		if err == nil && len(filename) > 7 && filename[:7] == "EXISTS:" {
			alreadyExists = true
			filename = filename[7:]
		}

		if err != nil {
			result.Success = false
			result.Error = err.Error()
			result.Message = fmt.Sprintf("Download failed: %v", err)
		} else {
			result.Success = true
			result.FilePath = filename
			result.AlreadyExists = alreadyExists
			if alreadyExists {
				result.Message = "File already exists"
			} else {
				result.Message = "Download completed successfully"
			}
		}

		pm.resultChan <- result

		// Update progress
		completed := atomic.AddInt32(&pm.jobsCompleted, 1)
		if pm.progressUpdate != nil {
			pm.progressUpdate(completed, atomic.LoadInt32(&pm.totalJobs))
		}

		fmt.Printf("[Worker %d] Completed: %s (%d/%d)\n", id, job.TrackName, completed, atomic.LoadInt32(&pm.totalJobs))
	}
}

// SubmitJob adds a job to the queue
func (pm *ParallelDownloadManager) SubmitJob(job *DownloadJob) error {
	pm.shutdownLock.Lock()
	if pm.shutdown {
		pm.shutdownLock.Unlock()
		return fmt.Errorf("download manager is shut down")
	}
	pm.shutdownLock.Unlock()

	atomic.AddInt32(&pm.totalJobs, 1)
	pm.jobQueue <- job
	return nil
}

// GetResult retrieves the next completed job result
func (pm *ParallelDownloadManager) GetResult() *DownloadJobResult {
	return <-pm.resultChan
}

// GetResultNonBlocking retrieves a result without blocking
func (pm *ParallelDownloadManager) GetResultNonBlocking() *DownloadJobResult {
	select {
	case result := <-pm.resultChan:
		return result
	default:
		return nil
	}
}

// WaitForCompletion waits for all submitted jobs to complete
// Returns all results in order
func (pm *ParallelDownloadManager) WaitForCompletion() []*DownloadJobResult {
	close(pm.jobQueue)
	pm.wg.Wait()
	close(pm.resultChan)

	var results []*DownloadJobResult
	for result := range pm.resultChan {
		results = append(results, result)
	}

	return results
}

// Shutdown gracefully shuts down the manager
func (pm *ParallelDownloadManager) Shutdown() {
	pm.shutdownLock.Lock()
	defer pm.shutdownLock.Unlock()

	if pm.shutdown {
		return
	}

	pm.shutdown = true
	close(pm.jobQueue)

	// Wait for all workers to finish
	pm.wg.Wait()
	close(pm.resultChan)

	fmt.Println("Download manager shut down")
}

// GetStats returns current statistics
func (pm *ParallelDownloadManager) GetStats() map[string]int32 {
	return map[string]int32{
		"workers":   atomic.LoadInt32(&pm.workersStarted),
		"completed": atomic.LoadInt32(&pm.jobsCompleted),
		"total":     atomic.LoadInt32(&pm.totalJobs),
	}
}
