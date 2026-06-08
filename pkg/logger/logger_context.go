package logger

import (
	"context"

	pcontext "github.com/poltergeist/poltergeist/pkg/context"
)

// LoggerContext extends the Logger interface with context-aware methods.
// This follows Go best practices for structured logging with request tracing.
type LoggerContext interface {
	Logger
	InfoContext(ctx context.Context, message string, fields ...Field)
	ErrorContext(ctx context.Context, message string, fields ...Field)
	WarnContext(ctx context.Context, message string, fields ...Field)
	DebugContext(ctx context.Context, message string, fields ...Field)
	SuccessContext(ctx context.Context, message string, fields ...Field)
}

// Ensure TargetLogger implements LoggerContext
var _ LoggerContext = (*TargetLogger)(nil)

// InfoContext logs an info message with context tracing
func (l *TargetLogger) InfoContext(ctx context.Context, message string, fields ...Field) {
	contextFields := l.extractContextFields(ctx)
	allFields := append(contextFields, fields...)
	l.Info(message, allFields...)
}

// ErrorContext logs an error message with context tracing
func (l *TargetLogger) ErrorContext(ctx context.Context, message string, fields ...Field) {
	contextFields := l.extractContextFields(ctx)
	allFields := append(contextFields, fields...)
	l.Error(message, allFields...)
}

// WarnContext logs a warning message with context tracing
func (l *TargetLogger) WarnContext(ctx context.Context, message string, fields ...Field) {
	contextFields := l.extractContextFields(ctx)
	allFields := append(contextFields, fields...)
	l.Warn(message, allFields...)
}

// DebugContext logs a debug message with context tracing
func (l *TargetLogger) DebugContext(ctx context.Context, message string, fields ...Field) {
	contextFields := l.extractContextFields(ctx)
	allFields := append(contextFields, fields...)
	l.Debug(message, allFields...)
}

// SuccessContext logs a success message with context tracing
func (l *TargetLogger) SuccessContext(ctx context.Context, message string, fields ...Field) {
	contextFields := l.extractContextFields(ctx)
	allFields := append(contextFields, fields...)
	l.Success(message, allFields...)
}

// extractContextFields extracts tracing fields from context
func (l *TargetLogger) extractContextFields(ctx context.Context) []Field {
	if ctx == nil {
		return nil
	}

	var fields []Field

	// Add request ID if present
	if requestID := pcontext.GetRequestID(ctx); requestID != "unknown-request" {
		fields = append(fields, WithField("request_id", requestID))
	}

	// Add correlation ID if present
	if correlationID := pcontext.GetCorrelationID(ctx); correlationID != "unknown-correlation" {
		fields = append(fields, WithField("correlation_id", correlationID))
	}

	// Add operation if present
	if operation := pcontext.GetOperation(ctx); operation != "unknown-operation" {
		fields = append(fields, WithField("operation", operation))
	}

	// Add duration if start time is present
	if duration := pcontext.GetDuration(ctx); duration > 0 {
		fields = append(fields, WithField("duration_ms", duration.Milliseconds()))
	}

	return fields
}

// WithContext creates a logger that automatically includes context fields
func WithContext(ctx context.Context, logger Logger) Logger {
	if ctx == nil {
		return logger
	}

	// Create a wrapper that automatically adds context fields
	return &contextualLogger{
		ctx:    ctx,
		logger: logger,
	}
}

// contextualLogger wraps a logger with automatic context field extraction
type contextualLogger struct {
	ctx    context.Context
	logger Logger
}

func (cl *contextualLogger) Info(message string, fields ...Field) {
	if lc, ok := cl.logger.(LoggerContext); ok {
		lc.InfoContext(cl.ctx, message, fields...)
	} else {
		cl.logger.Info(message, fields...)
	}
}

func (cl *contextualLogger) Error(message string, fields ...Field) {
	if lc, ok := cl.logger.(LoggerContext); ok {
		lc.ErrorContext(cl.ctx, message, fields...)
	} else {
		cl.logger.Error(message, fields...)
	}
}

func (cl *contextualLogger) Warn(message string, fields ...Field) {
	if lc, ok := cl.logger.(LoggerContext); ok {
		lc.WarnContext(cl.ctx, message, fields...)
	} else {
		cl.logger.Warn(message, fields...)
	}
}

func (cl *contextualLogger) Debug(message string, fields ...Field) {
	if lc, ok := cl.logger.(LoggerContext); ok {
		lc.DebugContext(cl.ctx, message, fields...)
	} else {
		cl.logger.Debug(message, fields...)
	}
}

func (cl *contextualLogger) Success(message string, fields ...Field) {
	if lc, ok := cl.logger.(LoggerContext); ok {
		lc.SuccessContext(cl.ctx, message, fields...)
	} else {
		cl.logger.Success(message, fields...)
	}
}

func (cl *contextualLogger) WithTarget(target string) Logger {
	return &contextualLogger{
		ctx:    cl.ctx,
		logger: cl.logger.WithTarget(target),
	}
}
