# Poltergeist Makefile

# Variables
BINARY_NAME=poltergeist
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

.PHONY: all build clean test coverage lint fmt vet install run help

## help: Show this help message
help:
	@echo "👻 Poltergeist - Go Build System"
	@echo ""
	@echo "Available targets:"
	@grep -E '^##' Makefile | sed 's/## /  /' | column -t -s ':'

## all: Build and test everything
all: fmt vet lint test build

## build: Build the binary
build:
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	@go build $(LDFLAGS) -o $(BINARY_NAME) cmd/poltergeist/main.go
	@echo "$(GREEN)✅ Build complete: ./$(BINARY_NAME)$(NC)"

## install: Install the binary to GOPATH/bin
install:
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(NC)"
	@go install $(LDFLAGS) cmd/poltergeist/main.go
	@echo "$(GREEN)✅ Installed to $(GOPATH)/bin/$(BINARY_NAME)$(NC)"

## clean: Remove build artifacts
clean:
	@echo "$(YELLOW)Cleaning...$(NC)"
	@go clean
	@rm -f $(BINARY_NAME)
	@rm -rf dist/
	@rm -rf coverage/
	@echo "$(GREEN)✅ Clean complete$(NC)"

## test: Run tests
test:
	@echo "$(GREEN)Running tests...$(NC)"
	@go test -v -race -timeout 30s ./...
	@echo "$(GREEN)✅ Tests passed$(NC)"

## test-short: Run short tests
test-short:
	@echo "$(GREEN)Running short tests...$(NC)"
	@go test -short -v ./...

## coverage: Generate test coverage report
coverage:
	@echo "$(GREEN)Generating coverage report...$(NC)"
	@mkdir -p coverage
	@go test -coverprofile=coverage/coverage.out ./...
	@go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "$(GREEN)✅ Coverage report generated: coverage/coverage.html$(NC)"
	@echo "Coverage: $$(go tool cover -func=coverage/coverage.out | grep total | awk '{print $$3}')"

## benchmark: Run benchmarks
benchmark:
	@echo "$(GREEN)Running benchmarks...$(NC)"
	@go test -bench=. -benchmem ./...

## lint: Run linters
lint:
	@echo "$(GREEN)Running linters...$(NC)"
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not installed, skipping...$(NC)"; \
	fi
	@echo "$(GREEN)✅ Linting complete$(NC)"

## fmt: Format code
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	@gofmt -s -w $(GOFILES)
	@goimports -w $(GOFILES) 2>/dev/null || true
	@echo "$(GREEN)✅ Formatting complete$(NC)"

## vet: Run go vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✅ Vet complete$(NC)"

## mod: Download and tidy modules
mod:
	@echo "$(GREEN)Tidying modules...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)✅ Modules updated$(NC)"

## run: Run the application
run: build
	@echo "$(GREEN)Running $(BINARY_NAME)...$(NC)"
	@./$(BINARY_NAME)

## watch: Watch for changes and rebuild
watch:
	@echo "$(GREEN)Watching for changes...$(NC)"
	@if command -v entr > /dev/null; then \
		find . -name '*.go' | entr -r make build; \
	else \
		echo "$(RED)entr not installed. Install it with: brew install entr$(NC)"; \
	fi

## docker-build: Build Docker image
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	@docker build -t poltergeist:$(VERSION) .
	@echo "$(GREEN)✅ Docker image built: poltergeist:$(VERSION)$(NC)"

## docker-run: Run in Docker container
docker-run: docker-build
	@echo "$(GREEN)Running in Docker...$(NC)"
	@docker run --rm -it -v $(PWD):/workspace poltergeist:$(VERSION)

## release: Create release builds for multiple platforms
release:
	@echo "$(GREEN)Building releases...$(NC)"
	@mkdir -p dist
	@echo "Building for Darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 cmd/poltergeist/main.go
	@echo "Building for Darwin/arm64..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 cmd/poltergeist/main.go
	@echo "Building for Linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 cmd/poltergeist/main.go
	@echo "Building for Linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 cmd/poltergeist/main.go
	@echo "Building for Windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe cmd/poltergeist/main.go
	@echo "$(GREEN)✅ Release builds complete in dist/$(NC)"

## init-project: Initialize a Poltergeist project in current directory
init-project: build
	@./$(BINARY_NAME) init

## dev: Run in development mode with live reload
dev:
	@echo "$(GREEN)Starting development mode...$(NC)"
	@go run cmd/poltergeist/main.go watch --verbosity=debug

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "$(GREEN)✅ All checks passed$(NC)"

## deps: List dependencies
deps:
	@echo "$(GREEN)Dependencies:$(NC)"
	@go list -m all

## update-deps: Update all dependencies
update-deps:
	@echo "$(GREEN)Updating dependencies...$(NC)"
	@go get -u ./...
	@go mod tidy
	@echo "$(GREEN)✅ Dependencies updated$(NC)"

## size: Show binary size
size: build
	@echo "$(GREEN)Binary size:$(NC)"
	@ls -lh $(BINARY_NAME) | awk '{print $$5 "\t" $$9}'

## loc: Count lines of code
loc:
	@echo "$(GREEN)Lines of code:$(NC)"
	@find . -name '*.go' -not -path "./vendor/*" | xargs wc -l | tail -1

# Default target
.DEFAULT_GOAL := help