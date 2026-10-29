package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"lark-downloader/internal/lark"
)

func TestEngineDownloadSuccess(t *testing.T) {
	content := "hello world binary payload test 12345"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "lark_dl_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(EngineConfig{
		Workers:   2,
		Retries:   2,
		Timeout:   5 * time.Second,
		OutputDir: tempDir,
		Quiet:     true,
	}, &lark.SessionInfo{Cookies: "test=1"})

	files := []lark.RemoteFile{
		{
			ID:          "token1",
			Name:        "file1.bin",
			Size:        int64(len(content)),
			DownloadURL: server.URL + "/file1",
		},
		{
			ID:          "token2",
			Name:        "file2.bin",
			Size:        int64(len(content)),
			DownloadURL: server.URL + "/file2",
		},
	}

	results := engine.DownloadAll(context.Background(), files)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, res := range results {
		if res.Status != "success" {
			t.Errorf("expected success, got %s: %s", res.Status, res.LastError)
		}
		savedData, err := os.ReadFile(filepath.Join(tempDir, res.Path))
		if err != nil {
			t.Errorf("failed to read saved file %s: %v", res.Path, err)
		} else if string(savedData) != content {
			t.Errorf("content mismatch: got %q, want %q", string(savedData), content)
		}
	}
}

func TestEngineRetryOnTemporaryError(t *testing.T) {
	content := "recovered after retry"
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "lark_retry_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(EngineConfig{
		Workers:   1,
		Retries:   3,
		Timeout:   5 * time.Second,
		OutputDir: tempDir,
		Quiet:     true,
	}, nil)

	files := []lark.RemoteFile{
		{
			ID:          "token_retry",
			Name:        "retry.bin",
			DownloadURL: server.URL,
		},
	}

	results := engine.DownloadAll(context.Background(), files)
	if results[0].Status != "success" {
		t.Fatalf("expected success on 3rd attempt, got %s (err: %s)", results[0].Status, results[0].LastError)
	}
	if results[0].Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", results[0].Attempts)
	}
}

func TestEnginePermanentErrorNoRetry(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "lark_perm_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(EngineConfig{
		Workers:   1,
		Retries:   3,
		Timeout:   5 * time.Second,
		OutputDir: tempDir,
		Quiet:     true,
	}, nil)

	files := []lark.RemoteFile{
		{
			ID:          "token_forbidden",
			Name:        "forbidden.bin",
			DownloadURL: server.URL,
		},
	}

	results := engine.DownloadAll(context.Background(), files)
	if results[0].Status != "failed" {
		t.Fatalf("expected failed status, got %s", results[0].Status)
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected exactly 1 attempt for HTTP 403, got %d", attempts)
	}
}

func TestEngineRejectHTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>Error login required</body></html>"))
	}))
	defer server.Close()

	tempDir, err := os.MkdirTemp("", "lark_html_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	engine := NewEngine(EngineConfig{
		Workers:   1,
		Retries:   1,
		Timeout:   5 * time.Second,
		OutputDir: tempDir,
		Quiet:     true,
	}, nil)

	files := []lark.RemoteFile{
		{
			ID:          "token_html",
			Name:        "should_fail.jpg",
			DownloadURL: server.URL,
		},
	}

	results := engine.DownloadAll(context.Background(), files)
	if results[0].Status != "failed" {
		t.Errorf("expected failure when server returns HTML, got status %s", results[0].Status)
	}
}
