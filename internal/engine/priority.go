// Priority engine implementation (merged from pkg/priority)
package engine

import (
	"math"
	"sync"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// PriorityEngine calculates build priorities
type PriorityEngine struct {
	config        *types.BuildSchedulingConfig
	logger        logger.Logger
	targetMetrics map[string]*targetMetrics
	fileChanges   map[string][]fileChangeRecord
	mu            sync.RWMutex
}

type targetMetrics struct {
	lastBuildTime    time.Duration
	totalBuilds      int
	successfulBuilds int
	lastDirectChange time.Time
	changeFrequency  float64
	recentChanges    []types.ChangeEvent
}

type fileChangeRecord struct {
	timestamp time.Time
	targets   []string
}

// NewPriorityEngine creates a new priority engine
func NewPriorityEngine(config *types.BuildSchedulingConfig, log logger.Logger) *PriorityEngine {
	return &PriorityEngine{
		config:        config,
		logger:        log,
		targetMetrics: make(map[string]*targetMetrics),
		fileChanges:   make(map[string][]fileChangeRecord),
	}
}

// CalculatePriority calculates priority for a build request
func (e *PriorityEngine) CalculatePriority(target types.Target, triggeringFiles []string) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	basePriority := 50.0

	// Get target metrics
	metrics, exists := e.targetMetrics[target.GetName()]
	if !exists {
		// New target gets moderate priority
		return basePriority
	}

	// Factor 1: Recent direct changes (higher priority)
	if time.Since(metrics.lastDirectChange) < time.Duration(e.config.Prioritization.FocusDetectionWindow)*time.Millisecond {
		basePriority += 30.0
	}

	// Factor 2: Change frequency (more frequent = higher priority)
	basePriority += metrics.changeFrequency * 10.0

	// Factor 3: Success rate (lower success = lower priority)
	if metrics.totalBuilds > 0 {
		successRate := float64(metrics.successfulBuilds) / float64(metrics.totalBuilds)
		// Only apply success rate if we have build history
		basePriority *= (0.5 + successRate*0.5) // Scale between 0.5x and 1x
	}

	// Factor 4: Build time (faster builds get slight priority)
	if metrics.lastBuildTime < 5*time.Second {
		basePriority += 10.0
	} else if metrics.lastBuildTime > 30*time.Second {
		basePriority -= 10.0
	}

	// Factor 5: Time decay (older requests lose priority)
	for _, change := range metrics.recentChanges {
		age := time.Since(change.Timestamp)
		decayTime := time.Duration(e.config.Prioritization.PriorityDecayTime) * time.Millisecond
		if age < decayTime {
			decayFactor := 1.0 - (float64(age) / float64(decayTime))
			basePriority += decayFactor * 5.0
		}
	}

	// Clamp to reasonable range
	if basePriority < 0 {
		basePriority = 0
	} else if basePriority > 100 {
		basePriority = 100
	}

	return basePriority
}

// UpdateTargetMetrics updates metrics after a build
func (e *PriorityEngine) UpdateTargetMetrics(target string, buildTime time.Duration, success bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	metrics, exists := e.targetMetrics[target]
	if !exists {
		metrics = &targetMetrics{
			recentChanges: make([]types.ChangeEvent, 0),
		}
		e.targetMetrics[target] = metrics
	}

	metrics.lastBuildTime = buildTime
	metrics.totalBuilds++
	if success {
		metrics.successfulBuilds++
	}

	// Update change frequency
	e.updateChangeFrequency(metrics)
}

// GetTargetPriority returns priority information for a target
func (e *PriorityEngine) GetTargetPriority(target string) *types.TargetPriority {
	e.mu.RLock()
	defer e.mu.RUnlock()

	metrics, exists := e.targetMetrics[target]
	if !exists {
		return nil
	}

	successRate := float64(metrics.successfulBuilds) / float64(max(metrics.totalBuilds, 1))

	// Calculate priority score based on metrics
	score := 50.0 // Base priority

	// Factor 1: Recent direct changes (higher priority)
	if e.config != nil && e.config.Prioritization.Enabled {
		if time.Since(metrics.lastDirectChange) < time.Duration(e.config.Prioritization.FocusDetectionWindow)*time.Millisecond {
			score += 30.0
		}

		// Factor 2: Change frequency (more frequent = higher priority)
		score += metrics.changeFrequency * 10.0

		// Factor 3: Build time (longer builds = slightly higher priority)
		buildTimeSeconds := metrics.lastBuildTime.Seconds()
		score += math.Min(buildTimeSeconds/10.0, 10.0)

		// Factor 4: Success rate (lower success = higher priority for fixing)
		score += (1.0 - successRate) * 20.0
	}

	return &types.TargetPriority{
		Target:                target,
		Score:                 score,
		LastDirectChange:      metrics.lastDirectChange,
		DirectChangeFrequency: metrics.changeFrequency,
		FocusMultiplier:       1.0,
		AvgBuildTime:          metrics.lastBuildTime,
		SuccessRate:           successRate,
		RecentChanges:         metrics.recentChanges,
	}
}

// RecordFileChange records a file change event
func (e *PriorityEngine) RecordFileChange(file string, targets []string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Record file change
	record := fileChangeRecord{
		timestamp: time.Now(),
		targets:   targets,
	}

	e.fileChanges[file] = append(e.fileChanges[file], record)

	// Update target metrics
	for _, target := range targets {
		metrics, exists := e.targetMetrics[target]
		if !exists {
			metrics = &targetMetrics{
				recentChanges: make([]types.ChangeEvent, 0),
			}
			e.targetMetrics[target] = metrics
		}

		metrics.lastDirectChange = time.Now()

		// Add to recent changes
		event := types.ChangeEvent{
			File:            file,
			Timestamp:       time.Now(),
			AffectedTargets: targets,
			ChangeType:      types.ChangeTypeDirect,
			ImpactWeight:    1.0,
		}

		metrics.recentChanges = append(metrics.recentChanges, event)

		// Keep only recent changes
		if len(metrics.recentChanges) > 100 {
			metrics.recentChanges = metrics.recentChanges[1:]
		}

		// Update change frequency
		e.updateChangeFrequency(metrics)
	}
}

// Private methods

func (e *PriorityEngine) updateChangeFrequency(metrics *targetMetrics) {
	// Calculate change frequency based on recent changes
	if len(metrics.recentChanges) < 2 {
		metrics.changeFrequency = 0
		return
	}

	// Calculate average time between changes
	totalDuration := time.Duration(0)
	for i := 1; i < len(metrics.recentChanges); i++ {
		duration := metrics.recentChanges[i].Timestamp.Sub(metrics.recentChanges[i-1].Timestamp)
		totalDuration += duration
	}

	avgDuration := totalDuration / time.Duration(len(metrics.recentChanges)-1)

	// Convert to frequency (changes per minute)
	if avgDuration > 0 {
		metrics.changeFrequency = float64(time.Minute) / float64(avgDuration)
	} else {
		metrics.changeFrequency = 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
