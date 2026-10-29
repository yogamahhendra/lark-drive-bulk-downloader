package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var invalidWindowsChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// CreateSessionDir creates a unique directory with format YYYYMMDD_HHMMSS under baseDir.
// If the directory already exists, an incremental suffix is appended (e.g. YYYYMMDD_HHMMSS_1).
func CreateSessionDir(baseDir string, t time.Time) (string, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	timeFormat := t.Format("20060102_150405")
	targetDir := filepath.Join(baseDir, timeFormat)

	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return "", err
		}
		return targetDir, nil
	}

	// Suffix handling for collisions
	for i := 1; ; i++ {
		candidate := filepath.Join(baseDir, fmt.Sprintf("%s_%d", timeFormat, i))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			if err := os.MkdirAll(candidate, 0755); err != nil {
				return "", err
			}
			return candidate, nil
		}
	}
}

// SanitizeFilename cleans up a raw filename to prevent path traversal and ensure safety on Windows and Linux.
func SanitizeFilename(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "unnamed_file"
	}

	// Remove path separators to prevent path traversal
	cleaned = filepath.Base(cleaned)

	// Replace invalid characters with underscore
	cleaned = invalidWindowsChars.ReplaceAllString(cleaned, "_")

	// Windows doesn't allow filenames ending with dot or space
	cleaned = strings.TrimRight(cleaned, ". ")

	// Reserved Windows filenames (CON, PRN, AUX, NUL, COM1-9, LPT1-9)
	upper := strings.ToUpper(cleaned)
	baseUpper := upper
	if extIdx := strings.Index(upper, "."); extIdx != -1 {
		baseUpper = upper[:extIdx]
	}
	reserved := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
		"COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
		"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
	if reserved[baseUpper] {
		cleaned = "_" + cleaned
	}

	if cleaned == "" || strings.Trim(cleaned, "_") == "" {
		return "unnamed_file"
	}

	return cleaned
}

// ResolveUniqueFilename returns a unique filename path in the directory, appending (1), (2), etc. if needed.
func ResolveUniqueFilename(dir, filename string) string {
	sanitized := SanitizeFilename(filename)
	ext := filepath.Ext(sanitized)
	base := strings.TrimSuffix(sanitized, ext)

	candidate := filepath.Join(dir, sanitized)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}

	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// SanitizeRelativePath cleans each path component while preserving safe relative subdirectories.
func SanitizeRelativePath(rawPath string) string {
	rawPath = strings.ReplaceAll(rawPath, "\\", "/")
	parts := strings.Split(rawPath, "/")
	var cleanedParts []string
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" || p == "." || p == ".." {
			continue
		}
		cleaned := SanitizeFilename(p)
		if cleaned != "" && cleaned != "." && cleaned != ".." {
			cleanedParts = append(cleanedParts, cleaned)
		}
	}
	if len(cleanedParts) == 0 {
		return "unnamed_file"
	}
	return filepath.Join(cleanedParts...)
}

// ResolveUniquePath ensures target path in baseDir is unique, creating parent directories as needed.
func ResolveUniquePath(baseDir, relPath string) string {
	cleanRel := SanitizeRelativePath(relPath)
	dir := filepath.Join(baseDir, filepath.Dir(cleanRel))
	filename := filepath.Base(cleanRel)
	return ResolveUniqueFilename(dir, filename)
}
