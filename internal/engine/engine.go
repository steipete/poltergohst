// Package engine provides the core build orchestration engine for Poltergeist.
// This package consolidates the following functionality:
// - Core orchestration (formerly pkg/poltergeist)
// - Build queue management (formerly pkg/queue)
// - Process lifecycle (formerly pkg/process)
// - Context utilities (formerly pkg/context)
// - Priority engine (formerly pkg/priority)
package engine

// This file serves as the package documentation.
// The actual implementation is split across multiple files for clarity:
// - poltergeist.go: Core orchestration engine
// - factory.go: Dependency injection factory
// - safegroup.go: Panic-safe concurrency utilities
// - queue.go: Build queue implementation
// - process.go: Process lifecycle management
// - context.go: Context enrichment utilities
