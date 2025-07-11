package ai

import (
	"strings"
)

// shouldSkipPath determines if a path should be skipped during file operations
func shouldSkipPath(path string) bool {
	// Common directories to skip
	skipDirs := []string{
		"node_modules", ".git", ".vscode", ".idea", "dist", "build", "target",
		"vendor", ".cache", ".tmp", "coverage", ".nyc_output", "logs",
		"bin", "obj", ".vs", "__pycache__", ".pytest_cache", ".mypy_cache",
	}

	pathLower := strings.ToLower(path)
	for _, skipDir := range skipDirs {
		if strings.Contains(pathLower, skipDir) {
			return true
		}
	}

	// Skip files with certain extensions
	skipExts := []string{".log", ".tmp", ".temp", ".cache", ".lock"}
	for _, ext := range skipExts {
		if strings.HasSuffix(pathLower, ext) {
			return true
		}
	}

	return false
}

// escapeRegexQuery escapes special regex characters in the query
func escapeRegexQuery(query string) string {
	// For simple text searches, we want to escape regex special characters
	// but we need to be careful not to over-escape

	// First, handle backslashes (must be first)
	escapedQuery := strings.ReplaceAll(query, "\\", "\\\\")

	// Escape regex special characters
	specialChars := []string{".", "*", "+", "?", "^", "$", "|", "(", ")", "[", "]", "{", "}", " "}
	for _, char := range specialChars {
		escapedQuery = strings.ReplaceAll(escapedQuery, char, "\\"+char)
	}

	return escapedQuery
}
