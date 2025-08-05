# Contributing to Poltergeist

Thank you for your interest in contributing to Poltergeist! This guide will help you get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Code Style](#code-style)
- [Documentation](#documentation)

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct:

- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on constructive criticism
- Accept feedback gracefully
- Prioritize the project's best interests

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR-USERNAME/poltergeist.git
   cd poltergeist
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/steipete/poltergeist.git
   ```

## Development Setup

### Prerequisites

- Go 1.22 or later
- Watchman (optional, for testing)
- golangci-lint (for linting)
- Make (optional)

### Building

```bash
# Build the main binary
go build -o poltergeist cmd/poltergeist/main.go

# Build the polter wrapper
go build -o polter cmd/polter/main.go

# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./...
```

### Development Workflow

1. Create a feature branch:
   ```bash
   git checkout -b feature/my-feature
   ```

2. Make your changes

3. Run tests:
   ```bash
   go test ./...
   ```

4. Run linter:
   ```bash
   golangci-lint run
   ```

5. Commit your changes (see [Commit Guidelines](#commit-guidelines))

## Making Changes

### What to Work On

- Check [open issues](https://github.com/steipete/poltergeist/issues)
- Look for issues labeled `good first issue`
- Review the [roadmap](https://github.com/steipete/poltergeist/projects)
- Propose new features via [discussions](https://github.com/steipete/poltergeist/discussions)

### Types of Contributions

- **Bug fixes**: Fix reported issues
- **Features**: Add new functionality
- **Documentation**: Improve docs, add examples
- **Tests**: Increase test coverage
- **Performance**: Optimize code
- **Refactoring**: Improve code quality

## Testing

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./pkg/watchman

# With coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Writing Tests

- Place tests in `*_test.go` files
- Use table-driven tests when appropriate
- Mock external dependencies
- Test both success and error cases
- Aim for >80% coverage

Example test:

```go
func TestWatchmanConnect(t *testing.T) {
    tests := []struct {
        name    string
        setup   func()
        wantErr bool
    }{
        {
            name:    "successful connection",
            setup:   func() { /* mock setup */ },
            wantErr: false,
        },
        {
            name:    "connection failure",
            setup:   func() { /* mock setup */ },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.setup()
            err := Connect()
            if (err != nil) != tt.wantErr {
                t.Errorf("Connect() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Submitting Changes

### Commit Guidelines

Follow conventional commits format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Code style changes
- `refactor`: Refactoring
- `test`: Tests
- `chore`: Maintenance

Examples:
```
feat(watchman): add fsnotify fallback support

fix(builder): handle spaces in file paths

docs(readme): update installation instructions
```

### Pull Request Process

1. **Update your fork**:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Push to your fork**:
   ```bash
   git push origin feature/my-feature
   ```

3. **Create Pull Request**:
   - Clear title and description
   - Reference related issues
   - Include test results
   - Add screenshots if relevant

4. **PR Checklist**:
   - [ ] Tests pass
   - [ ] Code follows style guidelines
   - [ ] Documentation updated
   - [ ] Commits are clean
   - [ ] PR description is complete

## Code Style

### Go Style

Follow standard Go conventions:

- Run `gofmt` on all code
- Use `golangci-lint` for additional checks
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use meaningful variable names
- Add comments for exported functions

### Project Conventions

- **Package names**: Lowercase, no underscores
- **Interface names**: End with `-er` suffix when appropriate
- **Error handling**: Always check errors
- **Logging**: Use structured logging
- **Configuration**: Use JSON for configs

### Code Organization

```
pkg/
├── <package>/
│   ├── <package>.go        # Main implementation
│   ├── <package>_test.go   # Tests
│   ├── types.go            # Type definitions
│   └── doc.go              # Package documentation
```

## Documentation

### Code Documentation

- Document all exported types and functions
- Use complete sentences
- Include examples for complex functions
- Update README for user-facing changes

Example:

```go
// Client represents a Watchman client connection.
// It provides methods for watching files and receiving change notifications.
type Client struct {
    // ...
}

// Connect establishes a connection to the Watchman service.
// It returns an error if Watchman is not available or the connection fails.
//
// Example:
//
//	client := watchman.NewClient(logger)
//	if err := client.Connect(ctx); err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) Connect(ctx context.Context) error {
    // ...
}
```

### User Documentation

Update relevant documentation:

- `README.md` - Overview and quick start
- `docs/CONFIGURATION.md` - Configuration options
- `docs/API.md` - API documentation
- `examples/` - Example configurations

## Review Process

### What to Expect

1. **Automated checks**: CI runs tests and linting
2. **Code review**: Maintainers review code
3. **Feedback**: Address review comments
4. **Approval**: Two approvals required
5. **Merge**: Squash and merge to main

### Review Criteria

- Code quality and style
- Test coverage
- Documentation
- Performance impact
- Breaking changes
- Security considerations

## Getting Help

- **Discord**: Join our [Discord server](https://discord.gg/poltergeist)
- **Discussions**: Use [GitHub Discussions](https://github.com/steipete/poltergeist/discussions)
- **Issues**: Report bugs via [Issues](https://github.com/steipete/poltergeist/issues)

## Recognition

Contributors are recognized in:
- [CONTRIBUTORS.md](CONTRIBUTORS.md)
- Release notes
- Project README

Thank you for contributing to Poltergeist! 👻