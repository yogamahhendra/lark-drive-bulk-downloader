package downloader

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lark-downloader/internal/filesystem"
	"lark-downloader/internal/lark"
	"lark-downloader/internal/report"
)

// EngineConfig holds settings for the download engine.
type EngineConfig struct {
	Workers   int
	Retries   int
	Timeout   time.Duration
	OutputDir string
	Quiet     bool
}

// Engine coordinates the concurrent bulk downloading of remote files.
type Engine struct {
	cfg        EngineConfig
	session    *lark.SessionInfo
	httpClient *http.Client
	printMu    sync.Mutex

	totalFiles       int64
	completedFiles   int64
	failedFiles      int64
	downloadedBytes  int64
	startTime        time.Time
}

// NewEngine creates a new download Engine.
func NewEngine(cfg EngineConfig, session *lark.SessionInfo) *Engine {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.Retries < 0 {
		cfg.Retries = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Minute
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: cfg.Workers * 2,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Engine{
		cfg:     cfg,
		session: session,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		},
	}
}

// DownloadAll runs the worker pool to download all files into outputDir.
func (e *Engine) DownloadAll(ctx context.Context, files []lark.RemoteFile) []report.FileItem {
	e.totalFiles = int64(len(files))
	e.completedFiles = 0
	e.failedFiles = 0
	e.downloadedBytes = 0
	e.startTime = time.Now()

	results := make([]report.FileItem, len(files))
	jobs := make(chan int, len(files))

	var wg sync.WaitGroup
	workers := e.cfg.Workers
	if workers > len(files) {
		workers = len(files)
	}
	if workers < 1 {
		workers = 1
	}

	// Start workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if ctx.Err() != nil {
					results[idx] = report.FileItem{
						Name:      files[idx].Name,
						Path:      files[idx].Path,
						Size:      files[idx].Size,
						Status:    "failed",
						LastError: "context cancelled",
					}
					atomic.AddInt64(&e.failedFiles, 1)
					continue
				}

				res := e.downloadWithRetry(ctx, files[idx])
				results[idx] = res

				if res.Status == "success" {
					atomic.AddInt64(&e.completedFiles, 1)
					atomic.AddInt64(&e.downloadedBytes, res.BytesDownloaded)
				} else {
					atomic.AddInt64(&e.failedFiles, 1)
				}

				e.logProgress()
			}
		}()
	}

	for i := range files {
		jobs <- i
	}
	close(jobs)

	wg.Wait()
	return results
}

func (e *Engine) downloadWithRetry(ctx context.Context, file lark.RemoteFile) report.FileItem {
	item := report.FileItem{
		Name: file.Name,
		Path: file.Name,
		Size: file.Size,
	}

	maxAttempts := e.cfg.Retries + 1
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		item.Attempts = attempt

		if attempt > 1 {
			e.logRetry(file.Name, attempt, maxAttempts, lastErr)
		}

		relPath := file.Path
		if relPath == "" {
			relPath = file.Name
		}
		targetPath := filesystem.ResolveUniquePath(e.cfg.OutputDir, relPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			item.Status = "failed"
			item.LastError = err.Error()
			e.logFailure(file.Name, item.LastError)
			return item
		}
		if rel, err := filepath.Rel(e.cfg.OutputDir, targetPath); err == nil {
			item.Path = rel
		} else {
			item.Path = filepath.Base(targetPath)
		}

		bytesWritten, isPermanent, err := e.downloadSingle(ctx, file, targetPath)
		if err == nil {
			item.Status = "success"
			item.BytesDownloaded = bytesWritten
			item.LastError = ""
			e.logSuccess(file.Name, bytesWritten)
			return item
		}

		lastErr = err
		item.LastError = err.Error()

		// Stop retrying if it's a permanent error (HTTP 401, 403, 404, etc.)
		if isPermanent || ctx.Err() != nil {
			break
		}

		if attempt < maxAttempts {
			backoff := e.calculateBackoff(attempt)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				item.Status = "failed"
				item.LastError = "context cancelled"
				e.logFailure(file.Name, item.LastError)
				return item
			}
		}
	}

	item.Status = "failed"
	e.logFailure(file.Name, lastErr.Error())
	return item
}

// downloadSingle performs a single download attempt, with resume support via Range if available.
func (e *Engine) downloadSingle(ctx context.Context, file lark.RemoteFile, targetPath string) (int64, bool, error) {
	// Check for existing partial file for resume
	var existingSize int64
	if stat, err := os.Stat(targetPath); err == nil {
		existingSize = stat.Size()
		// If already complete according to known size
		if file.Size > 0 && existingSize == file.Size {
			return existingSize, false, nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.DownloadURL, nil)
	if err != nil {
		return 0, true, fmt.Errorf("failed to create request: %w", err)
	}

	// Attach authenticated visitor headers
	if e.session != nil {
		if e.session.Cookies != "" {
			req.Header.Set("Cookie", e.session.Cookies)
		}
		if e.session.UserAgent != "" {
			req.Header.Set("User-Agent", e.session.UserAgent)
		}
		if e.session.ShareURL != "" {
			req.Header.Set("Referer", e.session.ShareURL)
		}
	}

	// Try Range request if partial file exists
	isResuming := false
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		isResuming = true
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, false, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	// Check status codes
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		return 0, true, fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	// Range not satisfiable -> restart from beginning
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		_ = os.Remove(targetPath)
		return e.downloadSingle(ctx, file, targetPath)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		// Temporary error (e.g. 429, 500, 502, 503)
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "" {
			return 0, false, fmt.Errorf("HTTP %d (Retry-After: %s)", resp.StatusCode, retryAfter)
		}
		return 0, false, fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	// Integrity check: verify response is not an HTML error page
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/html") {
		return 0, true, fmt.Errorf("server returned HTML instead of file content (likely session expired)")
	}

	var outFile *os.File
	if resp.StatusCode == http.StatusPartialContent && isResuming {
		// Resume append
		outFile, err = os.OpenFile(targetPath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return 0, false, fmt.Errorf("failed to open file for append: %w", err)
		}
	} else {
		// Fresh file
		outFile, err = os.Create(targetPath)
		if err != nil {
			return 0, false, fmt.Errorf("failed to create destination file: %w", err)
		}
		existingSize = 0
	}
	defer outFile.Close()

	// Stream directly to disk
	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return existingSize + written, false, fmt.Errorf("failed during streaming to disk: %w", err)
	}

	totalSize := existingSize + written

	// Validate against Content-Length if full download
	if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 && written != resp.ContentLength {
		return totalSize, false, fmt.Errorf("incomplete download: expected %d bytes, got %d", resp.ContentLength, written)
	}

	// Validate against known file size if provided
	if file.Size > 0 && totalSize != file.Size {
		// Note: only treat as error if size was strictly known and mismatch
		if totalSize < file.Size {
			return totalSize, false, fmt.Errorf("truncated file: expected %d bytes, got %d", file.Size, totalSize)
		}
	}

	return totalSize, false, nil
}

func (e *Engine) calculateBackoff(attempt int) time.Duration {
	base := 500 * time.Millisecond
	mult := 1 << (attempt - 1)
	jitter := time.Duration(rand.Intn(300)) * time.Millisecond
	backoff := base*time.Duration(mult) + jitter
	if backoff > 10*time.Second {
		backoff = 10 * time.Second
	}
	return backoff
}

func (e *Engine) logSuccess(name string, size int64) {
	if e.cfg.Quiet {
		return
	}
	e.printMu.Lock()
	defer e.printMu.Unlock()
	fmt.Printf("[OK]   %-35s %s\n", truncateString(name, 35), formatBytes(size))
}

func (e *Engine) logFailure(name string, reason string) {
	if e.cfg.Quiet {
		return
	}
	e.printMu.Lock()
	defer e.printMu.Unlock()
	fmt.Printf("[FAIL] %-35s %s\n", truncateString(name, 35), reason)
}

func (e *Engine) logRetry(name string, attempt, maxAttempts int, err error) {
	if e.cfg.Quiet {
		return
	}
	e.printMu.Lock()
	defer e.printMu.Unlock()
	fmt.Printf("[RETRY] %-30s attempt %d/%d (%v)\n", truncateString(name, 30), attempt, maxAttempts, err)
}

func (e *Engine) logProgress() {
	if e.cfg.Quiet {
		return
	}
	e.printMu.Lock()
	defer e.printMu.Unlock()

	completed := atomic.LoadInt64(&e.completedFiles)
	failed := atomic.LoadInt64(&e.failedFiles)
	bytes := atomic.LoadInt64(&e.downloadedBytes)
	total := e.totalFiles

	fmt.Printf("Progress: %d/%d files | Downloaded: %s | Failed: %d\n", completed, total, formatBytes(bytes), failed)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
