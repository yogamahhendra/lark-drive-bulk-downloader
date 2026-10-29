package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseArgsPositional(t *testing.T) {
	var buf bytes.Buffer
	args := []string{"https://example.larksuite.com/drive/folder/123", "--workers", "8"}
	cfg, err := ParseArgs(args, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.URL != "https://example.larksuite.com/drive/folder/123" {
		t.Errorf("expected URL, got %s", cfg.URL)
	}
	if cfg.Workers != 8 {
		t.Errorf("expected workers 8, got %d", cfg.Workers)
	}
	if cfg.Retries != 3 {
		t.Errorf("expected default retries 3, got %d", cfg.Retries)
	}
	if cfg.OutputDir != "./output" {
		t.Errorf("expected default output ./output, got %s", cfg.OutputDir)
	}
}

func TestParseArgsFlags(t *testing.T) {
	var buf bytes.Buffer
	args := []string{
		"--url", "https://example.larksuite.com/file/456",
		"-o", "./my_downloads",
		"-w", "2",
		"-r", "5",
		"-t", "10m",
		"-q",
	}
	cfg, err := ParseArgs(args, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.URL != "https://example.larksuite.com/file/456" {
		t.Errorf("expected URL, got %s", cfg.URL)
	}
	if cfg.OutputDir != "./my_downloads" {
		t.Errorf("expected output ./my_downloads, got %s", cfg.OutputDir)
	}
	if cfg.Workers != 2 {
		t.Errorf("expected workers 2, got %d", cfg.Workers)
	}
	if cfg.Retries != 5 {
		t.Errorf("expected retries 5, got %d", cfg.Retries)
	}
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("expected timeout 10m, got %v", cfg.Timeout)
	}
	if !cfg.Quiet {
		t.Errorf("expected quiet mode true, got false")
	}
}

func TestParseArgsValidation(t *testing.T) {
	var buf bytes.Buffer

	// Missing URL
	_, err := ParseArgs([]string{"--workers", "4"}, &buf)
	if err == nil {
		t.Errorf("expected error for missing URL, got nil")
	}

	// Workers = 0
	_, err = ParseArgs([]string{"https://example.larksuite.com/drive/folder/1", "--workers", "0"}, &buf)
	if err == nil {
		t.Errorf("expected error for workers 0, got nil")
	}

	// Workers > 64
	_, err = ParseArgs([]string{"https://example.larksuite.com/drive/folder/1", "--workers", "100"}, &buf)
	if err == nil {
		t.Errorf("expected error for workers 100, got nil")
	}

	// Negative retries
	_, err = ParseArgs([]string{"https://example.larksuite.com/drive/folder/1", "--retries", "-1"}, &buf)
	if err == nil {
		t.Errorf("expected error for negative retries, got nil")
	}
}

func TestParseArgsHelp(t *testing.T) {
	var buf bytes.Buffer
	cfg, err := ParseArgs([]string{"--help"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error on help: %v", err)
	}
	if !cfg.Help {
		t.Errorf("expected Help true, got false")
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Errorf("expected usage output in buffer, got: %s", buf.String())
	}
}
