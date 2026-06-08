// Package logger provides enhanced logging with target-specific support
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

// Logger interface for abstracted logging
type Logger interface {
	Info(message string, fields ...Field)
	Error(message string, fields ...Field)
	Warn(message string, fields ...Field)
	Debug(message string, fields ...Field)
	Success(message string, fields ...Field)
	WithTarget(target string) Logger
}

// Field represents a structured logging field
type Field struct {
	Key   string
	Value interface{}
}

// WithField creates a new field
func WithField(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// TargetLogger implements Logger with target awareness
type TargetLogger struct {
	logger     *logrus.Logger
	targetName string
	mu         sync.RWMutex
}

// CustomFormatter formats logs with colors and emojis
type CustomFormatter struct {
	TimestampFormat string
	DisableColors   bool
}

// Format implements logrus.Formatter
func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	ghost := "👻"
	timestamp := entry.Time.Format(f.TimestampFormat)

	// Color the level
	var levelColor *color.Color
	var levelText string

	switch entry.Level {
	case logrus.ErrorLevel:
		levelColor = color.New(color.FgRed, color.Bold)
		levelText = "ERROR"
	case logrus.WarnLevel:
		levelColor = color.New(color.FgYellow, color.Bold)
		levelText = "WARN"
	case logrus.InfoLevel:
		levelColor = color.New(color.FgCyan)
		levelText = "INFO"
	case logrus.DebugLevel:
		levelColor = color.New(color.FgWhite, color.Faint)
		levelText = "DEBUG"
	default:
		levelColor = color.New(color.FgGreen)
		levelText = "SUCCESS"
	}

	// Build target prefix
	targetPrefix := ""
	if target, ok := entry.Data["target"]; ok {
		targetPrefix = fmt.Sprintf("[%s] ", color.New(color.FgBlue).Sprint(target))
		delete(entry.Data, "target") // Remove from data to avoid duplication
	}

	// Format the message
	var output string
	if f.DisableColors {
		output = fmt.Sprintf("%s [%s] %s: %s%s", ghost, timestamp, levelText, targetPrefix, entry.Message)
	} else {
		output = fmt.Sprintf("%s [%s] %s: %s%s",
			ghost,
			timestamp,
			levelColor.Sprint(levelText),
			targetPrefix,
			entry.Message,
		)
	}

	// Add remaining fields
	if len(entry.Data) > 0 {
		fields := " {"
		first := true
		for k, v := range entry.Data {
			if !first {
				fields += ", "
			}
			fields += fmt.Sprintf("%s=%v", k, v)
			first = false
		}
		fields += "}"
		output += color.New(color.FgWhite, color.Faint).Sprint(fields)
	}

	return []byte(output + "\n"), nil
}

// CreateLogger creates a new logger instance
func CreateLogger(logFile string, logLevel string) Logger {
	log := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Set custom formatter for console
	log.SetFormatter(&CustomFormatter{
		TimestampFormat: "15:04:05",
		DisableColors:   false,
	})

	// Add file output if specified
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			multiWriter := io.MultiWriter(os.Stdout, file)
			log.SetOutput(multiWriter)
		}
	}

	return &TargetLogger{
		logger: log,
	}
}

// CreateTargetLogger creates a logger for a specific target
func CreateTargetLogger(baseLogger Logger, targetName string) Logger {
	if tl, ok := baseLogger.(*TargetLogger); ok {
		return &TargetLogger{
			logger:     tl.logger,
			targetName: targetName,
		}
	}
	return baseLogger
}

// CreateLoggerWithOutput creates a logger with custom output (for testing)
func CreateLoggerWithOutput(logFile string, logLevel string, output io.Writer) Logger {
	log := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Set custom formatter for console
	log.SetFormatter(&CustomFormatter{
		TimestampFormat: "15:04:05",
		DisableColors:   true, // Disable colors for test output
	})

	// Set custom output
	log.SetOutput(output)

	return &TargetLogger{
		logger: log,
	}
}

// WithTarget creates a new logger with target context
func (l *TargetLogger) WithTarget(target string) Logger {
	return &TargetLogger{
		logger:     l.logger,
		targetName: target,
	}
}

// convertFields converts Field slice to logrus.Fields
func (l *TargetLogger) convertFields(fields []Field) logrus.Fields {
	result := make(logrus.Fields)
	if l.targetName != "" {
		result["target"] = l.targetName
	}
	for _, f := range fields {
		result[f.Key] = f.Value
	}
	return result
}

// Info logs an info message
func (l *TargetLogger) Info(message string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.WithFields(l.convertFields(fields)).Info(message)
}

// Error logs an error message
func (l *TargetLogger) Error(message string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.WithFields(l.convertFields(fields)).Error(message)
}

// Warn logs a warning message
func (l *TargetLogger) Warn(message string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.WithFields(l.convertFields(fields)).Warn(message)
}

// Debug logs a debug message
func (l *TargetLogger) Debug(message string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.WithFields(l.convertFields(fields)).Debug(message)
}

// Success logs a success message (info level with special formatting)
func (l *TargetLogger) Success(message string, fields ...Field) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.logger.WithFields(l.convertFields(fields)).Info("✅ " + message)
}

// SimpleLogger provides a lightweight logger without dependencies
type SimpleLogger struct {
	targetName string
	logLevel   logrus.Level
	mu         sync.RWMutex
}

// NewSimpleLogger creates a simple console logger
func NewSimpleLogger(targetName string, logLevel string) Logger {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}

	return &SimpleLogger{
		targetName: targetName,
		logLevel:   level,
	}
}

// shouldLog checks if message should be logged at given level
func (l *SimpleLogger) shouldLog(level logrus.Level) bool {
	return level <= l.logLevel
}

// formatMessage formats a log message
func (l *SimpleLogger) formatMessage(level, message string) string {
	ghost := "👻"
	time := time.Now().Format("15:04:05")
	target := ""
	if l.targetName != "" {
		target = fmt.Sprintf(" [%s]", l.targetName)
	}
	return fmt.Sprintf("%s [%s] %s:%s %s", ghost, time, level, target, message)
}

// WithTarget creates a new logger with target context
func (l *SimpleLogger) WithTarget(target string) Logger {
	return &SimpleLogger{
		targetName: target,
		logLevel:   l.logLevel,
	}
}

// Info logs an info message
func (l *SimpleLogger) Info(message string, fields ...Field) {
	if l.shouldLog(logrus.InfoLevel) {
		l.mu.RLock()
		defer l.mu.RUnlock()
		fmt.Println(l.formatMessage("INFO", message))
		if len(fields) > 0 {
			l.printFields(fields)
		}
	}
}

// Error logs an error message
func (l *SimpleLogger) Error(message string, fields ...Field) {
	if l.shouldLog(logrus.ErrorLevel) {
		l.mu.RLock()
		defer l.mu.RUnlock()
		fmt.Fprintln(os.Stderr, color.RedString(l.formatMessage("ERROR", message)))
		if len(fields) > 0 {
			l.printFields(fields)
		}
	}
}

// Warn logs a warning message
func (l *SimpleLogger) Warn(message string, fields ...Field) {
	if l.shouldLog(logrus.WarnLevel) {
		l.mu.RLock()
		defer l.mu.RUnlock()
		fmt.Println(color.YellowString(l.formatMessage("WARN", message)))
		if len(fields) > 0 {
			l.printFields(fields)
		}
	}
}

// Debug logs a debug message
func (l *SimpleLogger) Debug(message string, fields ...Field) {
	if l.shouldLog(logrus.DebugLevel) {
		l.mu.RLock()
		defer l.mu.RUnlock()
		fmt.Println(color.New(color.Faint).Sprint(l.formatMessage("DEBUG", message)))
		if len(fields) > 0 {
			l.printFields(fields)
		}
	}
}

// Success logs a success message
func (l *SimpleLogger) Success(message string, fields ...Field) {
	if l.shouldLog(logrus.InfoLevel) {
		l.mu.RLock()
		defer l.mu.RUnlock()
		fmt.Println(color.GreenString(l.formatMessage("INFO", "✅ "+message)))
		if len(fields) > 0 {
			l.printFields(fields)
		}
	}
}

// printFields prints structured fields
func (l *SimpleLogger) printFields(fields []Field) {
	for _, f := range fields {
		fmt.Printf("  %s: %v\n", f.Key, f.Value)
	}
}

// ConsoleLogger provides simple console output for CLI
type ConsoleLogger struct{}

// NewConsoleLogger creates a console logger for CLI output
func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{}
}

// Info prints info message
func (c *ConsoleLogger) Info(message string) {
	ghost := "👻"
	fmt.Printf("%s %s %s\n", ghost, color.CyanString("[Poltergeist]"), message)
}

// Error prints error message
func (c *ConsoleLogger) Error(message string) {
	ghost := "👻"
	fmt.Fprintf(os.Stderr, "%s %s %s\n", ghost, color.RedString("[Poltergeist]"), message)
}

// Warn prints warning message
func (c *ConsoleLogger) Warn(message string) {
	ghost := "👻"
	fmt.Printf("%s %s %s\n", ghost, color.YellowString("[Poltergeist]"), message)
}

// Success prints success message
func (c *ConsoleLogger) Success(message string) {
	ghost := "👻"
	fmt.Printf("%s %s ✅ %s\n", ghost, color.GreenString("[Poltergeist]"), message)
}
