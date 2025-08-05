package engine

import (
	"context"
	"fmt"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
	"github.com/poltergeist/poltergeist/pkg/logger"
)

// SafeGroup wraps errgroup.Group with panic recovery to prevent
// service crashes from panicking goroutines. This follows Go best
// practices for production concurrency.
type SafeGroup struct {
	group  *errgroup.Group
	logger logger.Logger
}

// NewSafeGroup creates a new SafeGroup with panic recovery
func NewSafeGroup(ctx context.Context, logger logger.Logger) (*SafeGroup, context.Context) {
	g, ctx := errgroup.WithContext(ctx)
	return &SafeGroup{
		group:  g,
		logger: logger,
	}, ctx
}

// Go runs the given function in a new goroutine with panic recovery.
// Any panic is converted to an error and logged with stack trace.
func (sg *SafeGroup) Go(fn func() error) {
	sg.group.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				
				// Convert panic to error
				panicErr := fmt.Errorf("goroutine panic: %v", r)
				
				sg.logger.Error("Goroutine panic recovered",
					logger.WithField("panic", r),
					logger.WithField("stack_trace", string(stack)))
				
				err = panicErr
			}
		}()
		
		return fn()
	})
}

// SetLimit sets the maximum number of concurrent goroutines.
// This prevents resource exhaustion in production systems.
func (sg *SafeGroup) SetLimit(n int) {
	sg.group.SetLimit(n)
}

// Wait blocks until all goroutines have completed or any returns error.
// Returns the first error encountered.
func (sg *SafeGroup) Wait() (err error) {
	// Handle panics during Wait() itself
	defer func() {
		if r := recover(); r != nil {
			sg.logger.Error("Panic during SafeGroup.Wait()",
				logger.WithField("panic", r),
				logger.WithField("stack_trace", string(debug.Stack())))
			err = fmt.Errorf("wait panic: %v", r)
		}
	}()
	
	return sg.group.Wait()
}