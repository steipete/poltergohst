package logger_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/poltergeist/poltergeist/pkg/logger"
)

func TestCreateLogger(t *testing.T) {
	log := logger.CreateLogger("", "info")
	if log == nil {
		t.Fatal("expected logger to be created")
	}
}

func TestCreateLogger_Levels(t *testing.T) {
	tests := []struct {
		level   string
		message string
	}{
		{"debug", "debug message"},
		{"info", "info message"},
		{"warn", "warning message"},
		{"error", "error message"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			var buf bytes.Buffer
			log := logger.CreateLoggerWithOutput("", tt.level, &buf)

			// Log at different levels - just verify no panic
			log.Debug(tt.message)
			log.Info(tt.message)
			log.Warn(tt.message)
			log.Error(tt.message)

			output := buf.String()
			// At minimum, we should have some output for appropriate levels
			if tt.level != "error" && len(output) > 0 {
				// Output generated, that's good
				t.Logf("Level %s generated output: %d bytes", tt.level, len(output))
			}
		})
	}
}

func TestLogger_WithTarget(t *testing.T) {
	var buf bytes.Buffer
	log := logger.CreateLoggerWithOutput("", "info", &buf)

	targetLog := log.WithTarget("backend")
	targetLog.Info("building target")

	output := buf.String()
	if !strings.Contains(output, "backend") {
		t.Error("expected target name in log output")
	}
}

func TestLogger_Success(t *testing.T) {
	var buf bytes.Buffer
	log := logger.CreateLoggerWithOutput("", "info", &buf)

	log.Success("build completed")

	output := buf.String()
	if !strings.Contains(output, "build completed") {
		t.Error("expected success message in log output")
	}
}

func TestLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	log := logger.CreateLoggerWithOutput("", "info", &buf)

	log.Info("test message",
		logger.WithField("key1", "value1"),
		logger.WithField("key2", 42),
	)

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("expected message in log output")
	}
}

func TestLogger_MultipleTargets(t *testing.T) {
	var buf bytes.Buffer
	baseLog := logger.CreateLoggerWithOutput("", "info", &buf)

	backend := baseLog.WithTarget("backend")
	frontend := baseLog.WithTarget("frontend")

	backend.Info("backend message")
	frontend.Info("frontend message")

	output := buf.String()
	if !strings.Contains(output, "backend") {
		t.Error("expected backend target in output")
	}
	if !strings.Contains(output, "frontend") {
		t.Error("expected frontend target in output")
	}
}

func TestLogger_EmptyTarget(t *testing.T) {
	var buf bytes.Buffer
	log := logger.CreateLoggerWithOutput("", "info", &buf)

	log.Info("no target message")

	output := buf.String()
	if !strings.Contains(output, "no target message") {
		t.Error("expected message in log output")
	}
}

func TestLogger_ErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	log := logger.CreateLoggerWithOutput("", "error", &buf)

	log.Debug("should not appear")
	log.Info("should not appear")
	log.Warn("should not appear")
	log.Error("should appear")

	output := buf.String()
	if strings.Contains(output, "should not appear") {
		t.Error("lower level logs should not appear with error level")
	}
	if !strings.Contains(output, "should appear") {
		t.Error("error level log should appear")
	}
}
