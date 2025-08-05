package watchman

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// ConfigManager manages watchman configuration
type ConfigManager struct {
	projectRoot string
	logger      logger.Logger
}

// NewConfigManager creates a new watchman config manager
func NewConfigManager(projectRoot string, log logger.Logger) *ConfigManager {
	return &ConfigManager{
		projectRoot: projectRoot,
		logger:      log,
	}
}

// EnsureConfigUpToDate ensures watchman config is current
func (m *ConfigManager) EnsureConfigUpToDate(config *types.PoltergeistConfig) error {
	// TODO: Implement .watchmanconfig generation
	return nil
}

// SuggestOptimizations suggests performance optimizations
func (m *ConfigManager) SuggestOptimizations() ([]string, error) {
	suggestions := []string{}

	// TODO: Analyze project and suggest optimizations

	return suggestions, nil
}

// CreateExclusionExpressions creates watchman exclusion expressions
func (m *ConfigManager) CreateExclusionExpressions(config *types.PoltergeistConfig) []interfaces.ExclusionExpression {
	exclusions := []interfaces.ExclusionExpression{}

	// Add custom exclusions
	if config.Watchman != nil && config.Watchman.ExcludeDirs != nil {
		for _, dir := range config.Watchman.ExcludeDirs {
			exclusions = append(exclusions, interfaces.ExclusionExpression{
				Type:     "dirname",
				Patterns: []string{dir},
			})
		}
	}

	// Add default exclusions if enabled
	if config.Watchman == nil || config.Watchman.UseDefaultExclusions {
		defaultExclusions := []string{
			"node_modules", ".git", "vendor", "build", "dist", "target",
			".next", ".nuxt", ".cache", "coverage", ".vscode",
			".idea", "*.log", "tmp", "temp",
		}

		for _, pattern := range defaultExclusions {
			exclusions = append(exclusions, interfaces.ExclusionExpression{
				Type:     "dirname",
				Patterns: []string{pattern},
			})
		}
	}

	return exclusions
}

// NormalizeWatchPattern normalizes a watch pattern
func (m *ConfigManager) NormalizeWatchPattern(pattern string) string {
	// Clean up the pattern
	pattern = strings.TrimSpace(pattern)

	// Convert to absolute path if relative
	if !filepath.IsAbs(pattern) && !strings.Contains(pattern, "*") {
		pattern = filepath.Join(m.projectRoot, pattern)
	}

	// Normalize glob patterns
	if strings.HasPrefix(pattern, "**") {
		// Already a proper glob
		return pattern
	}

	// Add wildcards for directories
	if !strings.Contains(pattern, "*") {
		pattern = filepath.Join(pattern, "**", "*")
	}

	return pattern
}

// ValidateWatchPattern validates a watch pattern
func (m *ConfigManager) ValidateWatchPattern(pattern string) error {
	// Basic validation
	if pattern == "" {
		return fmt.Errorf("empty watch pattern")
	}

	// TODO: More sophisticated pattern validation

	return nil
}
