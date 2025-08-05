package context

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Context keys for request tracing and correlation.
// Using unexported struct pointers prevents key collisions.
var (
	requestIDKey     = &struct{}{}
	correlationIDKey = &struct{}{}
	userIDKey        = &struct{}{}
	operationKey     = &struct{}{}
	startTimeKey     = &struct{}{}
)

// WithRequestID adds a request ID to the context
func WithRequestID(parent context.Context, requestID string) context.Context {
	if requestID == "" {
		requestID = GenerateRequestID()
	}
	return context.WithValue(parent, requestIDKey, requestID)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok && id != "" {
		return id
	}
	return "unknown-request"
}

// WithCorrelationID adds a correlation ID to the context for distributed tracing
func WithCorrelationID(parent context.Context, correlationID string) context.Context {
	if correlationID == "" {
		correlationID = GenerateCorrelationID()
	}
	return context.WithValue(parent, correlationIDKey, correlationID)
}

// GetCorrelationID retrieves the correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok && id != "" {
		return id
	}
	return "unknown-correlation"
}

// WithUserID adds a user ID to the context
func WithUserID(parent context.Context, userID string) context.Context {
	return context.WithValue(parent, userIDKey, userID)
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(userIDKey).(string); ok && id != "" {
		return id
	}
	return "anonymous"
}

// WithOperation adds an operation name to the context
func WithOperation(parent context.Context, operation string) context.Context {
	return context.WithValue(parent, operationKey, operation)
}

// GetOperation retrieves the operation name from context
func GetOperation(ctx context.Context) string {
	if op, ok := ctx.Value(operationKey).(string); ok && op != "" {
		return op
	}
	return "unknown-operation"
}

// WithStartTime adds the operation start time to the context
func WithStartTime(parent context.Context, startTime time.Time) context.Context {
	return context.WithValue(parent, startTimeKey, startTime)
}

// GetStartTime retrieves the operation start time from context
func GetStartTime(ctx context.Context) time.Time {
	if t, ok := ctx.Value(startTimeKey).(time.Time); ok {
		return t
	}
	return time.Now()
}

// GetDuration calculates the duration since the start time in context
func GetDuration(ctx context.Context) time.Duration {
	startTime := GetStartTime(ctx)
	return time.Since(startTime)
}

// GenerateRequestID creates a new unique request ID
func GenerateRequestID() string {
	return "req_" + uuid.New().String()
}

// GenerateCorrelationID creates a new unique correlation ID
func GenerateCorrelationID() string {
	return "cor_" + uuid.New().String()
}

// EnrichContext adds common tracing information to a context
func EnrichContext(parent context.Context) context.Context {
	ctx := parent
	
	// Add request ID if not present
	if GetRequestID(ctx) == "unknown-request" {
		ctx = WithRequestID(ctx, GenerateRequestID())
	}
	
	// Add correlation ID if not present
	if GetCorrelationID(ctx) == "unknown-correlation" {
		ctx = WithCorrelationID(ctx, GenerateCorrelationID())
	}
	
	// Add start time
	ctx = WithStartTime(ctx, time.Now())
	
	return ctx
}

// TracingFields returns common tracing fields for structured logging
func TracingFields(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"request_id":     GetRequestID(ctx),
		"correlation_id": GetCorrelationID(ctx),
		"user_id":        GetUserID(ctx),
		"operation":      GetOperation(ctx),
		"duration_ms":    GetDuration(ctx).Milliseconds(),
	}
}