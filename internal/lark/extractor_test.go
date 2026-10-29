package lark

import (
	"context"
	"testing"
)

func TestBrowserExtractorFindBrowser(t *testing.T) {
	e := NewBrowserExtractor()
	path, err := e.FindBrowserPath()
	if err != nil {
		t.Logf("No browser found on test machine: %v", err)
		return
	}
	if path == "" {
		t.Errorf("FindBrowserPath returned empty path with nil error")
	}
}

func TestBrowserExtractorLiveList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live browser test in short mode")
	}

	e := NewBrowserExtractor()
	if _, err := e.FindBrowserPath(); err != nil {
		t.Skip("no browser available")
	}

	url := "https://kjpflil5k85o.jp.larksuite.com/drive/folder/XP0cfwQQclSLZsds27Cjuknxppg"
	var progressReports []int
	e.OnProgress = func(found int) {
		progressReports = append(progressReports, found)
	}

	files, session, err := e.ListFiles(context.Background(), url)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(files) < 150 {
		t.Errorf("Expected at least 150 files, got %d", len(files))
	}
	if session == nil || session.Cookies == "" {
		t.Errorf("Expected session cookies to be populated")
	}
	t.Logf("Successfully extracted %d files, progress updates: %v", len(files), progressReports)
}
