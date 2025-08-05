//go:build tools

// Package tools imports development dependencies to ensure they're tracked in go.mod.
// This follows Go best practices for managing tool dependencies.
// Install tools with: go install -tags tools ./...
package tools

import (
	// Linting and formatting
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "golang.org/x/tools/cmd/goimports"
	
	// Code generation
	_ "github.com/golang/mock/mockgen"
	
	// Testing tools
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "gotest.tools/gotestsum"
	
	// Security scanning
	_ "github.com/securego/gosec/v2/cmd/gosec"
	
	// Performance profiling
	_ "github.com/google/pprof"
	
	// API documentation
	_ "github.com/swaggo/swag/cmd/swag"
)