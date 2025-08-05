package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Mock log entry structure
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Target    string
	Message   string
}

// Test data for logs
var sampleLogEntries = []LogEntry{
	{time.Now().Add(-10 * time.Minute), "INFO", "target1", "Build started"},
	{time.Now().Add(-9 * time.Minute), "DEBUG", "target1", "Compiling main.go"},
	{time.Now().Add(-8 * time.Minute), "INFO", "target1", "Build completed successfully"},
	{time.Now().Add(-7 * time.Minute), "INFO", "target2", "Build started"},
	{time.Now().Add(-6 * time.Minute), "ERROR", "target2", "Compilation failed: syntax error"},
	{time.Now().Add(-5 * time.Minute), "INFO", "target1", "Build started"},
	{time.Now().Add(-4 * time.Minute), "WARN", "target1", "Warning: unused variable"},
	{time.Now().Add(-3 * time.Minute), "INFO", "target1", "Build completed successfully"},
	{time.Now().Add(-2 * time.Minute), "INFO", "target3", "Build started"},
	{time.Now().Add(-1 * time.Minute), "INFO", "target3", "Build completed successfully"},
}

func TestRunLogs_AllTargets(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create log directory and files
	setupTestLogs(t, tempDir, sampleLogEntries)

	// Test basic log retrieval (all targets)
	err := runLogs("", false, 50)
	if err != nil {
		t.Errorf("runLogs failed: %v", err)
	}
}

func TestRunLogs_SpecificTarget(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create log directory and files
	setupTestLogs(t, tempDir, sampleLogEntries)

	// Test log retrieval for specific target
	err := runLogs("target1", false, 50)
	if err != nil {
		t.Errorf("runLogs failed for specific target: %v", err)
	}
}

func TestRunLogs_LimitLines(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }() 

	// Create log directory with more entries
	manyEntries := make([]LogEntry, 100)
	for i := 0; i < 100; i++ {
		manyEntries[i] = LogEntry{
			Timestamp: time.Now().Add(time.Duration(-i) * time.Minute),
			Level:     "INFO",
			Target:    "target1",
			Message:   fmt.Sprintf("Log entry %d", i),
		}
	}
	setupTestLogs(t, tempDir, manyEntries)

	// Test with line limit
	err := runLogs("target1", false, 10)
	if err != nil {
		t.Errorf("runLogs failed with line limit: %v", err)
	}
}

func TestRunLogs_NonexistentTarget(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create log directory with sample logs
	setupTestLogs(t, tempDir, sampleLogEntries)

	// Test with nonexistent target - should not error but show no logs
	err := runLogs("nonexistent-target", false, 50)
	if err != nil {
		t.Errorf("runLogs should not error for nonexistent target: %v", err)
	}
}

func TestRunLogs_NoLogDirectory(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Don't create log directory - should handle gracefully
	err := runLogs("", false, 50)
	if err == nil {
		t.Error("Expected error when log directory doesn't exist")
	}
}

func TestRunLogs_EmptyLogDirectory(t *testing.T) {
	tempDir := t.TempDir()
	originalProjectRoot := projectRoot
	projectRoot = tempDir
	defer func() { projectRoot = originalProjectRoot }()

	// Create empty log directory
	logDir := filepath.Join(tempDir, ".poltergeist", "logs")
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	// Should handle empty directory gracefully
	err = runLogs("", false, 50)
	if err != nil {
		t.Errorf("runLogs should handle empty log directory: %v", err)
	}
}

func TestParseLogEntry_ValidFormats(t *testing.T) {
	testCases := []struct {
		line     string
		expected *LogEntry
	}{
		{
			line: "2023-12-01T10:30:45Z INFO target1 Build started",
			expected: &LogEntry{
				Level:   "INFO",
				Target:  "target1",
				Message: "Build started",
			},
		},
		{
			line: "2023-12-01T10:30:45Z ERROR target2 Compilation failed: syntax error in main.go:42",
			expected: &LogEntry{
				Level:   "ERROR",
				Target:  "target2",
				Message: "Compilation failed: syntax error in main.go:42",
			},
		},
		{
			line: "2023-12-01T10:30:45Z DEBUG target1 Processing file: /path/to/file.go",
			expected: &LogEntry{
				Level:   "DEBUG",
				Target:  "target1",
				Message: "Processing file: /path/to/file.go",
			},
		},
	}

	for _, tc := range testCases {
		entry := parseLogEntry(tc.line)
		if entry == nil {
			t.Errorf("Failed to parse valid log line: %s", tc.line)
			continue
		}

		if entry.Level != tc.expected.Level {
			t.Errorf("Level mismatch for line '%s': expected %s, got %s", tc.line, tc.expected.Level, entry.Level)
		}

		if entry.Target != tc.expected.Target {
			t.Errorf("Target mismatch for line '%s': expected %s, got %s", tc.line, tc.expected.Target, entry.Target)
		}

		if entry.Message != tc.expected.Message {
			t.Errorf("Message mismatch for line '%s': expected %s, got %s", tc.line, tc.expected.Message, entry.Message)
		}
	}
}

func TestParseLogEntry_InvalidFormats(t *testing.T) {
	invalidLines := []string{
		"Invalid log line",
		"2023-12-01T10:30:45Z", // Too few fields
		"Not a timestamp INFO target1 message", // Invalid timestamp
		"", // Empty line
		"   ", // Whitespace only
	}

	for _, line := range invalidLines {
		entry := parseLogEntry(line)
		if entry != nil {
			t.Errorf("Should not parse invalid log line: %s", line)
		}
	}
}

func TestFilterLogsByTarget(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Target: "target1", Message: "Message 1"},
		{Level: "ERROR", Target: "target2", Message: "Message 2"},
		{Level: "INFO", Target: "target1", Message: "Message 3"},
		{Level: "WARN", Target: "target3", Message: "Message 4"},
		{Level: "DEBUG", Target: "target1", Message: "Message 5"},
	}

	// Filter by target1
	filtered := filterLogsByTarget(entries, "target1")
	if len(filtered) != 3 {
		t.Errorf("Expected 3 entries for target1, got %d", len(filtered))
	}

	for _, entry := range filtered {
		if entry.Target != "target1" {
			t.Errorf("Filtered entries should only contain target1, got %s", entry.Target)
		}
	}

	// Filter by target2
	filtered = filterLogsByTarget(entries, "target2")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 entry for target2, got %d", len(filtered))
	}

	// Filter by nonexistent target
	filtered = filterLogsByTarget(entries, "nonexistent")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 entries for nonexistent target, got %d", len(filtered))
	}
}

func TestFilterLogsByLevel(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Target: "target1", Message: "Info message"},
		{Level: "ERROR", Target: "target1", Message: "Error message"},
		{Level: "WARN", Target: "target1", Message: "Warning message"},
		{Level: "DEBUG", Target: "target1", Message: "Debug message"},
		{Level: "INFO", Target: "target1", Message: "Another info message"},
	}

	// Filter by ERROR level
	filtered := filterLogsByLevel(entries, "ERROR")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 ERROR entry, got %d", len(filtered))
	}

	// Filter by INFO level
	filtered = filterLogsByLevel(entries, "INFO")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 INFO entries, got %d", len(filtered))
	}

	// Filter by nonexistent level
	filtered = filterLogsByLevel(entries, "CRITICAL")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 entries for CRITICAL level, got %d", len(filtered))
	}
}

func TestSortLogsByTimestamp(t *testing.T) {
	now := time.Now()
	entries := []LogEntry{
		{Timestamp: now.Add(-3 * time.Minute), Message: "Third"},
		{Timestamp: now.Add(-1 * time.Minute), Message: "First"},
		{Timestamp: now.Add(-2 * time.Minute), Message: "Second"},
	}

	sortLogsByTimestamp(entries)

	// Should be sorted by timestamp (newest first)
	expectedOrder := []string{"First", "Second", "Third"}
	for i, entry := range entries {
		if entry.Message != expectedOrder[i] {
			t.Errorf("Sort order incorrect at position %d: expected %s, got %s", i, expectedOrder[i], entry.Message)
		}
	}
}

func TestLimitLogEntries(t *testing.T) {
	entries := make([]LogEntry, 100)
	for i := 0; i < 100; i++ {
		entries[i] = LogEntry{
			Message: fmt.Sprintf("Entry %d", i),
		}
	}

	// Limit to 10 entries
	limited := limitLogEntries(entries, 10)
	if len(limited) != 10 {
		t.Errorf("Expected 10 entries, got %d", len(limited))
	}

	// Limit to more than available
	limited = limitLogEntries(entries[:5], 10)
	if len(limited) != 5 {
		t.Errorf("Expected 5 entries when limit > available, got %d", len(limited))
	}

	// Limit to 0
	limited = limitLogEntries(entries, 0)
	if len(limited) != 0 {
		t.Errorf("Expected 0 entries with limit 0, got %d", len(limited))
	}
}

func TestFormatLogEntry(t *testing.T) {
	entry := LogEntry{
		Timestamp: time.Date(2023, 12, 1, 10, 30, 45, 0, time.UTC),
		Level:     "INFO",
		Target:    "target1",
		Message:   "Build completed successfully",
	}

	formatted := formatLogEntry(entry)
	
	// Should contain all components
	if !strings.Contains(formatted, "INFO") {
		t.Error("Formatted entry should contain level")
	}
	if !strings.Contains(formatted, "target1") {
		t.Error("Formatted entry should contain target")
	}
	if !strings.Contains(formatted, "Build completed successfully") {
		t.Error("Formatted entry should contain message")
	}
	if !strings.Contains(formatted, "10:30:45") {
		t.Error("Formatted entry should contain time")
	}
}

func TestReadLogFile_ValidFile(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test log file
	logFile := filepath.Join(tempDir, "test.log")
	content := `2023-12-01T10:30:45Z INFO target1 Build started
2023-12-01T10:31:00Z ERROR target1 Compilation failed
2023-12-01T10:31:15Z INFO target1 Build retried`

	err := os.WriteFile(logFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	entries, err := readLogFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 log entries, got %d", len(entries))
	}

	// Check first entry
	if entries[0].Level != "INFO" {
		t.Errorf("Expected first entry level INFO, got %s", entries[0].Level)
	}
	if entries[0].Target != "target1" {
		t.Errorf("Expected first entry target target1, got %s", entries[0].Target)
	}
	if entries[0].Message != "Build started" {
		t.Errorf("Expected first entry message 'Build started', got %s", entries[0].Message)
	}
}

func TestReadLogFile_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create empty log file
	logFile := filepath.Join(tempDir, "empty.log")
	err := os.WriteFile(logFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty log file: %v", err)
	}

	entries, err := readLogFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read empty log file: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries from empty file, got %d", len(entries))
	}
}

func TestReadLogFile_NonexistentFile(t *testing.T) {
	entries, err := readLogFile("/nonexistent/file.log")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	if entries != nil {
		t.Error("Expected nil entries for nonexistent file")
	}
}

func TestGetLogFiles_ValidDirectory(t *testing.T) {
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	// Create test log files
	logFiles := []string{"target1.log", "target2.log", "system.log"}
	for _, filename := range logFiles {
		logFile := filepath.Join(logDir, filename)
		err := os.WriteFile(logFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create log file %s: %v", filename, err)
		}
	}

	// Create non-log file (should be ignored)
	nonLogFile := filepath.Join(logDir, "config.txt")
	err = os.WriteFile(nonLogFile, []byte("not a log"), 0644)
	if err != nil {
		t.Fatalf("Failed to create non-log file: %v", err)
	}

	files, err := getLogFiles(logDir)
	if err != nil {
		t.Fatalf("Failed to get log files: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("Expected 3 log files, got %d", len(files))
	}

	// Check that all log files are present
	foundFiles := make(map[string]bool)
	for _, file := range files {
		foundFiles[filepath.Base(file)] = true
	}

	for _, expectedFile := range logFiles {
		if !foundFiles[expectedFile] {
			t.Errorf("Expected log file %s not found", expectedFile)
		}
	}

	// Check that non-log file is not included
	if foundFiles["config.txt"] {
		t.Error("Non-log file should not be included")
	}
}

func TestGetLogFiles_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	files, err := getLogFiles(logDir)
	if err != nil {
		t.Fatalf("Failed to get log files from empty directory: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected 0 files from empty directory, got %d", len(files))
	}
}

func TestGetLogFiles_NonexistentDirectory(t *testing.T) {
	files, err := getLogFiles("/nonexistent/directory")
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
	if files != nil {
		t.Error("Expected nil files for nonexistent directory")
	}
}

// Helper functions for testing

func setupTestLogs(t *testing.T, tempDir string, entries []LogEntry) {
	logDir := filepath.Join(tempDir, ".poltergeist", "logs")
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	// Group entries by target
	targetLogs := make(map[string][]LogEntry)
	for _, entry := range entries {
		targetLogs[entry.Target] = append(targetLogs[entry.Target], entry)
	}

	// Create log files for each target
	for target, logs := range targetLogs {
		logFile := filepath.Join(logDir, target+".log")
		content := ""
		for _, entry := range logs {
			line := fmt.Sprintf("%s %s %s %s\n",
				entry.Timestamp.Format(time.RFC3339),
				entry.Level,
				entry.Target,
				entry.Message)
			content += line
		}

		err := os.WriteFile(logFile, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create log file for %s: %v", target, err)
		}
	}
}

// Placeholder implementations for functions that would be in the actual logs command

func parseLogEntry(line string) *LogEntry {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return nil
	}

	timestamp, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return nil
	}

	return &LogEntry{
		Timestamp: timestamp,
		Level:     parts[1],
		Target:    parts[2],
		Message:   strings.Join(parts[3:], " "),
	}
}

func filterLogsByTarget(entries []LogEntry, target string) []LogEntry {
	var filtered []LogEntry
	for _, entry := range entries {
		if entry.Target == target {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func filterLogsByLevel(entries []LogEntry, level string) []LogEntry {
	var filtered []LogEntry
	for _, entry := range entries {
		if entry.Level == level {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func sortLogsByTimestamp(entries []LogEntry) {
	// Simple bubble sort for testing (newest first)
	for i := 0; i < len(entries)-1; i++ {
		for j := 0; j < len(entries)-i-1; j++ {
			if entries[j].Timestamp.Before(entries[j+1].Timestamp) {
				entries[j], entries[j+1] = entries[j+1], entries[j]
			}
		}
	}
}

func limitLogEntries(entries []LogEntry, limit int) []LogEntry {
	if limit <= 0 {
		return []LogEntry{}
	}
	if limit >= len(entries) {
		return entries
	}
	return entries[:limit]
}

func formatLogEntry(entry LogEntry) string {
	return fmt.Sprintf("%s [%s] %s: %s",
		entry.Timestamp.Format("15:04:05"),
		entry.Level,
		entry.Target,
		entry.Message)
}

func readLogFile(filename string) ([]LogEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry := parseLogEntry(line)
		if entry != nil {
			entries = append(entries, *entry)
		}
	}

	return entries, scanner.Err()
}

func getLogFiles(logDir string) ([]string, error) {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	var logFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".log") {
			logFiles = append(logFiles, filepath.Join(logDir, file.Name()))
		}
	}

	return logFiles, nil
}