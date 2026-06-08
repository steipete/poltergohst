package utils_test

import (
	"testing"

	"github.com/poltergeist/poltergeist/pkg/utils"
)

func TestPatternMatcher_Match(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		path     string
		want     bool
	}{
		{
			name:     "simple wildcard",
			patterns: []string{"*.go"},
			path:     "main.go",
			want:     true,
		},
		{
			name:     "simple wildcard no match",
			patterns: []string{"*.go"},
			path:     "main.js",
			want:     false,
		},
		{
			name:     "double wildcard",
			patterns: []string{"**/*.go"},
			path:     "src/pkg/main.go",
			want:     true,
		},
		{
			name:     "double wildcard root",
			patterns: []string{"**/*.go"},
			path:     "main.go",
			want:     true,
		},
		{
			name:     "question mark",
			patterns: []string{"test?.go"},
			path:     "test1.go",
			want:     true,
		},
		{
			name:     "question mark no match",
			patterns: []string{"test?.go"},
			path:     "test12.go",
			want:     false,
		},
		{
			name:     "character class",
			patterns: []string{"test[0-9].go"},
			path:     "test5.go",
			want:     true,
		},
		{
			name:     "negated character class",
			patterns: []string{"test[!a-z].go"},
			path:     "test1.go",
			want:     true,
		},
		{
			name:     "multiple patterns",
			patterns: []string{"*.go", "*.js"},
			path:     "main.js",
			want:     true,
		},
		{
			name:     "exact match",
			patterns: []string{"main.go"},
			path:     "main.go",
			want:     true,
		},
		{
			name:     "directory pattern",
			patterns: []string{"src/**/*"},
			path:     "src/pkg/utils/file.go",
			want:     true,
		},
		{
			name:     "complex pattern",
			patterns: []string{"src/**/test_*.go"},
			path:     "src/pkg/test_utils.go",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher, err := utils.NewPatternMatcher(tt.patterns)
			if err != nil {
				t.Fatalf("failed to create matcher: %v", err)
			}

			if got := matcher.Match(tt.path); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPatternMatcher_MatchAny(t *testing.T) {
	matcher, _ := utils.NewPatternMatcher([]string{"*.go", "*.js"})

	paths := []string{"main.go", "test.py", "app.js", "style.css"}

	if !matcher.MatchAny(paths) {
		t.Error("expected MatchAny to return true")
	}

	paths = []string{"test.py", "style.css"}

	if matcher.MatchAny(paths) {
		t.Error("expected MatchAny to return false")
	}
}

func TestPatternMatcher_GetMatchingPaths(t *testing.T) {
	matcher, _ := utils.NewPatternMatcher([]string{"*.go", "test/*"})

	paths := []string{
		"main.go",
		"utils.go",
		"main.js",
		"test/unit.js",
		"test/integration.py",
		"src/app.go",
	}

	matching := matcher.GetMatchingPaths(paths)

	expected := []string{
		"main.go",
		"utils.go",
		"test/unit.js",
		"test/integration.py",
		"src/app.go",
	}

	if len(matching) != len(expected) {
		t.Errorf("expected %d matching paths, got %d", len(expected), len(matching))
	}
}

func TestIsGlobPattern(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		{"*.go", true},
		{"test?.js", true},
		{"src/[abc].txt", true},
		{"main.go", false},
		{"src/pkg/file.go", false},
		{"**/*.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if got := utils.IsGlobPattern(tt.pattern); got != tt.want {
				t.Errorf("IsGlobPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestNormalizePattern(t *testing.T) {
	tests := []struct {
		pattern string
		want    string
	}{
		{"./src/*.go", "src/*.go"},
		{"src/", "src"},
		{"\\path\\to\\file", "/path/to/file"},
		{"src/../test", "src/../test"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			if got := utils.NormalizePattern(tt.pattern); got != tt.want {
				t.Errorf("NormalizePattern(%q) = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestExpandPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		wantLen  int
		contains []string
	}{
		{
			pattern:  "*.go",
			wantLen:  2,
			contains: []string{"*.go", "**/*.go"},
		},
		{
			pattern:  "src",
			wantLen:  2,
			contains: []string{"src", "src/**/*"},
		},
		{
			pattern:  "**/*.go",
			wantLen:  1,
			contains: []string{"**/*.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			expanded := utils.ExpandPattern(tt.pattern)

			if len(expanded) != tt.wantLen {
				t.Errorf("ExpandPattern(%q) returned %d patterns, want %d", tt.pattern, len(expanded), tt.wantLen)
			}

			for _, want := range tt.contains {
				found := false
				for _, got := range expanded {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ExpandPattern(%q) missing pattern %q", tt.pattern, want)
				}
			}
		})
	}
}

func TestExclusionMatcher(t *testing.T) {
	patterns := []string{
		"node_modules",
		".git",
		"*.log",
		"tmp",
	}

	matcher, err := utils.NewExclusionMatcher(patterns)
	if err != nil {
		t.Fatalf("failed to create exclusion matcher: %v", err)
	}

	tests := []struct {
		path     string
		excluded bool
	}{
		{"node_modules/package/index.js", true},
		{"src/node_modules/test.js", true},
		{".git/config", true},
		{"error.log", true},
		{"tmp/cache.txt", true},
		{"src/main.go", false},
		{"test/unit.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := matcher.IsExcluded(tt.path); got != tt.excluded {
				t.Errorf("IsExcluded(%q) = %v, want %v", tt.path, got, tt.excluded)
			}
		})
	}
}

func TestExclusionMatcher_FilterPaths(t *testing.T) {
	matcher, _ := utils.NewExclusionMatcher([]string{
		"*.log",
		"tmp",
		"node_modules",
	})

	paths := []string{
		"src/main.go",
		"error.log",
		"tmp/cache.txt",
		"node_modules/pkg/index.js",
		"test/unit.js",
		"debug.log",
	}

	filtered := matcher.FilterPaths(paths)

	expected := []string{
		"src/main.go",
		"test/unit.js",
	}

	if len(filtered) != len(expected) {
		t.Errorf("expected %d paths after filtering, got %d", len(expected), len(filtered))
	}

	for i, path := range filtered {
		if path != expected[i] {
			t.Errorf("filtered[%d] = %q, want %q", i, path, expected[i])
		}
	}
}

func TestGetDefaultExclusions(t *testing.T) {
	exclusions := utils.GetDefaultExclusions()

	// Check some common exclusions are present
	expectedPatterns := []string{
		".git",
		"node_modules",
		"vendor",
		"*.pyc",
		".DS_Store",
	}

	for _, pattern := range expectedPatterns {
		found := false
		for _, exclusion := range exclusions {
			if exclusion == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected default exclusion %q not found", pattern)
		}
	}

	// Should have a reasonable number of exclusions
	if len(exclusions) < 10 {
		t.Errorf("expected at least 10 default exclusions, got %d", len(exclusions))
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "main.js", false},
		{"src/*.go", "src/main.go", true},
		{"src/*.go", "src/pkg/main.go", false},
		{"**/*.go", "src/pkg/main.go", true},
		{"test[0-9].txt", "test5.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+" vs "+tt.path, func(t *testing.T) {
			got, err := utils.MatchGlob(tt.pattern, tt.path)
			if err != nil {
				t.Fatalf("MatchGlob error: %v", err)
			}
			if got != tt.want {
				t.Errorf("MatchGlob(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestPatternMatcher_EdgeCases(t *testing.T) {
	// Empty pattern
	matcher, err := utils.NewPatternMatcher([]string{})
	if err != nil {
		t.Fatalf("failed with empty patterns: %v", err)
	}
	if matcher.Match("any.file") {
		t.Error("empty patterns should not match anything")
	}

	// Pattern with spaces
	matcher, _ = utils.NewPatternMatcher([]string{"file with spaces.txt"})
	if !matcher.Match("file with spaces.txt") {
		t.Error("should match file with spaces")
	}

	// Pattern with special characters
	matcher, _ = utils.NewPatternMatcher([]string{"file-name_2.0.txt"})
	if !matcher.Match("file-name_2.0.txt") {
		t.Error("should match file with special characters")
	}
}

func TestPatternMatcher_CaseSensitivity(t *testing.T) {
	matcher, _ := utils.NewPatternMatcher([]string{"*.GO"})

	// Go patterns are case-sensitive by default
	if matcher.Match("main.go") {
		t.Error("pattern matching should be case-sensitive")
	}

	if !matcher.Match("main.GO") {
		t.Error("should match exact case")
	}
}

func BenchmarkPatternMatcher_Match(b *testing.B) {
	patterns := []string{
		"**/*.go",
		"**/*.js",
		"**/*.ts",
		"**/test_*.py",
		"src/**/internal/*.c",
	}

	matcher, _ := utils.NewPatternMatcher(patterns)
	path := "src/pkg/internal/utils/helper.go"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Match(path)
	}
}

func BenchmarkExclusionMatcher_IsExcluded(b *testing.B) {
	exclusions := utils.GetDefaultExclusions()
	matcher, _ := utils.NewExclusionMatcher(exclusions)

	paths := []string{
		"src/main.go",
		"node_modules/pkg/index.js",
		".git/config",
		"build/output.exe",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			matcher.IsExcluded(path)
		}
	}
}
