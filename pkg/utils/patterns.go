package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// PatternMatcher handles glob pattern matching
type PatternMatcher struct {
	patterns []string
	regexps  []*regexp.Regexp
}

// NewPatternMatcher creates a new pattern matcher
func NewPatternMatcher(patterns []string) (*PatternMatcher, error) {
	// Expand patterns to include variations
	var expandedPatterns []string
	for _, pattern := range patterns {
		expandedPatterns = append(expandedPatterns, ExpandPattern(pattern)...)
	}
	
	pm := &PatternMatcher{
		patterns: expandedPatterns,
		regexps:  make([]*regexp.Regexp, 0, len(expandedPatterns)),
	}
	
	for _, pattern := range expandedPatterns {
		regex, err := pm.globToRegex(pattern)
		if err != nil {
			return nil, err
		}
		pm.regexps = append(pm.regexps, regex)
	}
	
	return pm, nil
}

// Match checks if a path matches any pattern
func (pm *PatternMatcher) Match(path string) bool {
	// Normalize path separators
	path = filepath.ToSlash(path)
	
	for _, regex := range pm.regexps {
		if regex.MatchString(path) {
			return true
		}
	}
	
	return false
}

// MatchAny checks if any of the paths match any pattern
func (pm *PatternMatcher) MatchAny(paths []string) bool {
	for _, path := range paths {
		if pm.Match(path) {
			return true
		}
	}
	return false
}

// GetMatchingPaths returns all paths that match any pattern
func (pm *PatternMatcher) GetMatchingPaths(paths []string) []string {
	var matches []string
	for _, path := range paths {
		if pm.Match(path) {
			matches = append(matches, path)
		}
	}
	return matches
}

// globToRegex converts a glob pattern to a regular expression
func (pm *PatternMatcher) globToRegex(pattern string) (*regexp.Regexp, error) {
	// Normalize pattern
	pattern = filepath.ToSlash(pattern)
	
	// Escape regex special characters except glob wildcards
	var regex strings.Builder
	regex.WriteString("^")
	
	i := 0
	for i < len(pattern) {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// ** matches any number of directories
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					regex.WriteString(".*")
					i += 3 // Skip **/
				} else {
					regex.WriteString(".*")
					i += 2 // Skip **
				}
			} else {
				// * matches any characters except /
				regex.WriteString("[^/]*")
				i++
			}
		case '?':
			// ? matches any single character except /
			regex.WriteString("[^/]")
			i++
		case '[':
			// Character class
			j := i + 1
			if j < len(pattern) && pattern[j] == '!' {
				regex.WriteString("[^")
				j++
			} else {
				regex.WriteString("[")
			}
			
			for j < len(pattern) && pattern[j] != ']' {
				if pattern[j] == '\\' && j+1 < len(pattern) {
					regex.WriteByte(pattern[j])
					regex.WriteByte(pattern[j+1])
					j += 2
				} else {
					regex.WriteByte(pattern[j])
					j++
				}
			}
			
			if j < len(pattern) {
				regex.WriteByte(']')
				i = j + 1
			} else {
				// Unclosed bracket, treat as literal
				regex.WriteString("\\[")
				i++
			}
		case '\\':
			// Escape character
			if i+1 < len(pattern) {
				regex.WriteByte('\\')
				regex.WriteByte(pattern[i+1])
				i += 2
			} else {
				regex.WriteString("\\\\")
				i++
			}
		case '.', '+', '^', '$', '(', ')', '{', '}', '|':
			// Escape regex special characters
			regex.WriteByte('\\')
			regex.WriteByte(pattern[i])
			i++
		default:
			regex.WriteByte(pattern[i])
			i++
		}
	}
	
	regex.WriteString("$")
	
	return regexp.Compile(regex.String())
}

// IsGlobPattern checks if a string contains glob wildcards
func IsGlobPattern(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

// NormalizePattern normalizes a file pattern
func NormalizePattern(pattern string) string {
	// Convert backslashes to forward slashes (for Windows compatibility)
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	
	// Remove leading ./
	pattern = strings.TrimPrefix(pattern, "./")
	
	// Remove trailing /
	pattern = strings.TrimSuffix(pattern, "/")
	
	return pattern
}

// ExpandPattern expands a pattern to include common variations
func ExpandPattern(pattern string) []string {
	patterns := []string{pattern}
	
	// If pattern is a directory (no wildcards or file extensions), add /**/* for contents
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, ".") {
		patterns = append(patterns, pattern+"/**/*")
	} else if !strings.HasPrefix(pattern, "**") && !strings.HasPrefix(pattern, "/") {
		// For file patterns, add ** prefix for recursive matching
		patterns = append(patterns, "**/"+pattern)
	}
	
	return patterns
}

// ExclusionMatcher handles exclusion patterns
type ExclusionMatcher struct {
	patterns []string
	matcher  *PatternMatcher
}

// NewExclusionMatcher creates a new exclusion matcher
func NewExclusionMatcher(patterns []string) (*ExclusionMatcher, error) {
	// Add common exclusion patterns
	allPatterns := append([]string{}, patterns...)
	
	// Convert directory names to patterns
	for i, pattern := range allPatterns {
		if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "/") {
			allPatterns[i] = "**/" + pattern + "/**"
		}
	}
	
	matcher, err := NewPatternMatcher(allPatterns)
	if err != nil {
		return nil, err
	}
	
	return &ExclusionMatcher{
		patterns: patterns,
		matcher:  matcher,
	}, nil
}

// IsExcluded checks if a path should be excluded
func (em *ExclusionMatcher) IsExcluded(path string) bool {
	return em.matcher.Match(path)
}

// FilterPaths removes excluded paths from a list
func (em *ExclusionMatcher) FilterPaths(paths []string) []string {
	var filtered []string
	for _, path := range paths {
		if !em.IsExcluded(path) {
			filtered = append(filtered, path)
		}
	}
	return filtered
}

// GetDefaultExclusions returns default exclusion patterns
func GetDefaultExclusions() []string {
	return []string{
		".git",
		".svn",
		".hg",
		"node_modules",
		"vendor",
		"target",
		"build",
		"dist",
		"out",
		".next",
		".nuxt",
		".cache",
		"coverage",
		".nyc_output",
		".pytest_cache",
		"__pycache__",
		"*.pyc",
		".mypy_cache",
		".tox",
		"*.egg-info",
		".terraform",
		".idea",
		".vscode",
		".vs",
		"*.swp",
		"*.swo",
		"*~",
		".DS_Store",
		"Thumbs.db",
		"*.log",
		"*.tmp",
		"*.temp",
		"*.bak",
		"*.old",
	}
}

// MatchGlob matches a path against a glob pattern
func MatchGlob(pattern, path string) (bool, error) {
	// Use filepath.Match for simple patterns
	if !strings.Contains(pattern, "**") {
		return filepath.Match(pattern, path)
	}
	
	// For ** patterns, use custom matcher
	matcher, err := NewPatternMatcher([]string{pattern})
	if err != nil {
		return false, err
	}
	
	return matcher.Match(path), nil
}