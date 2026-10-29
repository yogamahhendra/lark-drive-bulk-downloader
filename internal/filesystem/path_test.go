package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateSessionDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lark_test_session_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	now := time.Date(2026, 10, 29, 20, 4, 30, 0, time.UTC)
	sessionDir, err := CreateSessionDir(tempDir, now)
	if err != nil {
		t.Fatalf("unexpected error creating session dir: %v", err)
	}

	expectedName := "20261029_200430"
	if filepath.Base(sessionDir) != expectedName {
		t.Errorf("expected session dir name %s, got %s", expectedName, filepath.Base(sessionDir))
	}

	// Collision test: creating again with the same timestamp should append suffix _1
	sessionDir2, err := CreateSessionDir(tempDir, now)
	if err != nil {
		t.Fatalf("unexpected error on collision creation: %v", err)
	}
	expectedName2 := "20261029_200430_1"
	if filepath.Base(sessionDir2) != expectedName2 {
		t.Errorf("expected session dir name %s, got %s", expectedName2, filepath.Base(sessionDir2))
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal_photo.jpg", "normal_photo.jpg"},
		{"../../../etc/passwd", "passwd"},
		{"C:\\Windows\\System32\\calc.exe", "calc.exe"},
		{"bad:file*name?.png", "bad_file_name_.png"},
		{"trailing_dots....", "trailing_dots"},
		{"trailing_spaces   ", "trailing_spaces"},
		{"CON.txt", "_CON.txt"},
		{"NUL", "_NUL"},
		{"", "unnamed_file"},
		{"???", "unnamed_file"},
	}

	for _, tc := range tests {
		got := SanitizeFilename(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeFilename(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestResolveUniqueFilename(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lark_test_unique_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// First file
	path1 := ResolveUniqueFilename(tempDir, "image.jpg")
	if err := os.WriteFile(path1, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path1, "image.jpg") {
		t.Errorf("expected original name, got %s", path1)
	}

	// Second duplicate
	path2 := ResolveUniqueFilename(tempDir, "image.jpg")
	if err := os.WriteFile(path2, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path2, "image (1).jpg") {
		t.Errorf("expected duplicate name with (1), got %s", path2)
	}

	// Third duplicate
	path3 := ResolveUniqueFilename(tempDir, "image.jpg")
	if !strings.HasSuffix(path3, "image (2).jpg") {
		t.Errorf("expected duplicate name with (2), got %s", path3)
	}
}

func TestSanitizeRelativePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"folder/photo.jpg", filepath.Join("folder", "photo.jpg")},
		{"sub/nested/deep/file.pdf", filepath.Join("sub", "nested", "deep", "file.pdf")},
		{"../evil/path.jpg", filepath.Join("evil", "path.jpg")},
		{"folder/bad:name*.jpg", filepath.Join("folder", "bad_name_.jpg")},
		{"", "unnamed_file"},
	}

	for _, tc := range tests {
		got := SanitizeRelativePath(tc.input)
		if got != tc.expected {
			t.Errorf("SanitizeRelativePath(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestResolveUniquePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lark_test_uniq_path_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	subDir := filepath.Join(tempDir, "album")
	_ = os.MkdirAll(subDir, 0755)

	path1 := ResolveUniquePath(tempDir, "album/pic.jpg")
	if err := os.WriteFile(path1, []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}

	path2 := ResolveUniquePath(tempDir, "album/pic.jpg")
	if !strings.HasSuffix(path2, "pic (1).jpg") {
		t.Errorf("expected unique path with (1), got %s", path2)
	}
}
