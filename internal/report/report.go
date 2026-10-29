package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileItem represents download status of an individual file.
type FileItem struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	Size            int64  `json:"size"`
	Status          string `json:"status"` // "success" or "failed"
	Attempts        int    `json:"attempts"`
	LastError       string `json:"last_error,omitempty"`
	BytesDownloaded int64  `json:"bytes_downloaded"`
}

// DownloadReport represents the final download-report.json file.
type DownloadReport struct {
	StartTime            time.Time  `json:"start_time"`
	EndTime              time.Time  `json:"end_time"`
	Duration             string     `json:"duration"`
	SourceURL            string     `json:"source_url"`
	TotalFiles           int        `json:"total_files"`
	SuccessCount         int        `json:"success_count"`
	FailedCount          int        `json:"failed_count"`
	TotalBytesDownloaded int64      `json:"total_bytes_downloaded"`
	Files                []FileItem `json:"files"`
}

// WriteReport writes the download report to download-report.json in targetDir.
func WriteReport(targetDir string, r *DownloadReport) (string, error) {
	if r.Duration == "" && !r.StartTime.IsZero() && !r.EndTime.IsZero() {
		r.Duration = r.EndTime.Sub(r.StartTime).Round(time.Second).String()
	}

	reportPath := filepath.Join(targetDir, "download-report.json")
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal download report: %w", err)
	}

	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write download report file: %w", err)
	}

	return reportPath, nil
}
