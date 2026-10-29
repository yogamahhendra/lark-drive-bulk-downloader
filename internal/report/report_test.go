package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lark_test_report_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	start := time.Now().Add(-10 * time.Second)
	end := time.Now()

	rep := &DownloadReport{
		StartTime:            start,
		EndTime:              end,
		SourceURL:            "https://test.larksuite.com/drive/folder/123",
		TotalFiles:           2,
		SuccessCount:         1,
		FailedCount:          1,
		TotalBytesDownloaded: 1024,
		Files: []FileItem{
			{
				Name:            "test1.jpg",
				Path:            "test1.jpg",
				Size:            1024,
				Status:          "success",
				Attempts:        1,
				BytesDownloaded: 1024,
			},
			{
				Name:            "test2.png",
				Path:            "test2.png",
				Size:            2048,
				Status:          "failed",
				Attempts:        3,
				LastError:       "HTTP 404",
				BytesDownloaded: 0,
			},
		},
	}

	reportPath, err := WriteReport(tempDir, rep)
	if err != nil {
		t.Fatalf("WriteReport error: %v", err)
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	var parsed DownloadReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse report json: %v", err)
	}

	if parsed.TotalFiles != 2 || parsed.SuccessCount != 1 || parsed.FailedCount != 1 {
		t.Errorf("unexpected counts in parsed report: %+v", parsed)
	}
	if filepath.Base(reportPath) != "download-report.json" {
		t.Errorf("expected report filename download-report.json, got %s", filepath.Base(reportPath))
	}
}
