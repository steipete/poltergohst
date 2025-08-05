# Poltergeist Build System (Go Edition)

<div align="center">

👻 **The Haunted Build System** 👻

*Your helpful ghost that watches your files and rebuilds your projects*

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-macOS%20|%20Linux%20|%20Windows-lightgrey?style=for-the-badge)](https://github.com/steipete/poltergeist)

</div>

Poltergeist is an intelligent file watcher and build automation system, designed for the age of AI-assisted development. It watches your files, understands your project structure, and automatically rebuilds when changes are detected. 

Originally written in TypeScript, this is the Go implementation offering improved performance, better resource usage, and native compilation.

Requires Watchman to be installed:
  - **macOS**: `brew install watchman`
  - **Linux**: [Installation guide](https://facebook.github.io/watchman/docs/install#linux)
  - **Windows**: [Chocolatey package](https://facebook.github.io/watchman/docs/install#windows) or manual install

## Features

- **Universal Target System**: Support for anything you can build - executables, app bundles, libraries, frameworks, tests, Docker containers
- **Smart Execution Wrapper**: `polter` command that waits for a build to complete, then starts it
- **Efficient File Watching**: Powered by Facebook's Watchman with smart exclusions and performance optimization (with fsnotify fallback)
- **Intelligent Build Prioritization**: Multiple projects that share code? Poltergeist will compile the right one first
- **Automatic Project Configuration**: Just type `poltergeist init` and it'll parse your folder and set up the config
- **Native Notifications**: System notifications with customizable sounds and icons for build status
- **Concurrent Build Protection**: Intelligent locking prevents overlapping builds
- **Advanced State Management**: Process tracking, build history, and heartbeat monitoring
- **Automatic Configuration Reloading**: Changes to `poltergeist.config.json` are applied without restart

## Designed for Humans and Agents

Poltergeist has been designed with an agentic workflow in mind. As soon as your agent starts editing files, we'll start a background compile process. Since agents are relatively slow, there's a good chance your project already finished compiling before the agent tries to run it.

Benefits:
- Agents don't have to call build manually anymore
- They call your executable directly with `polter` as prefix, which waits until the build is complete
- Faster loops, fewer wasted tokens
- Build time is tracked, so agents can set their timeout correctly

Commands have been designed with the least surprises:
- `haunt` starts the daemon, but `start` is also a valid alias
- Commands executed in non-tty environments have helpful messages for agents
- Fuzzy matching finds targets even if misspelled
- Token-conservative output by default

## Quick Start

### Installation

#### From Source (Go 1.22+)

```bash
# Clone the repository
git clone https://github.com/steipete/poltergeist.git
cd poltergeist

# Build and install
go build -o poltergeist cmd/poltergeist/main.go
go build -o polter cmd/polter/main.go

# Move to PATH
sudo mv poltergeist polter /usr/local/bin/
```

#### From Release

Download the latest binary from [releases](https://github.com/steipete/poltergeist/releases) for your platform.

### Basic Usage

1. **Automatic Configuration** - Let Poltergeist analyze your project:

```bash
poltergeist init
```

This automatically detects your project type (Go, Swift, Node.js, Rust, Python, CMake, etc.) and creates an optimized configuration.

2. **Start Watching** - Begin auto-building on file changes:

```bash
poltergeist haunt        # Runs as background daemon (default)
poltergeist status       # Check what's running
```

3. **Execute Fresh Builds** - Use `polter` to ensure you never run stale code:

```bash
polter my-app            # Waits for build, then runs fresh binary
polter my-app --help     # All arguments passed through
```

That's it! Poltergeist now watches your files and rebuilds automatically.

## Command Line Interface

### Core Commands (poltergeist)

#### Starting and Managing the Daemon

```bash
# Start watching (runs as background daemon by default)
poltergeist haunt
poltergeist start         # Alias for haunt

# Check what's running
poltergeist status        # Shows all active projects and their build status

# View build logs
poltergeist logs          # Recent logs
poltergeist logs -f       # Follow logs in real-time

# Stop watching
poltergeist stop          # Stop all targets
poltergeist stop --target my-app  # Stop specific target
```

#### Project Management

```bash
# Initialize configuration
poltergeist init          # Auto-detect and create config
poltergeist init --cmake  # Specialized CMake detection

# List configured targets
poltergeist list          # Shows all targets and their status

# Clean up old state files
poltergeist clean         # Remove stale state files
poltergeist clean --all   # Remove all state files
```

### Smart Execution with polter

The `polter` command ensures you always run fresh builds:

```bash
# Basic usage
polter <target-name> [arguments...]

# Examples
polter my-app                    # Run after build completes
polter my-app --port 8080       # All arguments passed through
polter backend serve --watch    # Complex commands work too

# Options
polter my-app --timeout 60000   # Wait up to 60 seconds
polter my-app --force           # Run even if build failed
polter my-app --verbose         # Show build progress
```

## Configuration

### Configuration Schema

Create `poltergeist.config.json` in your project root:

```json
{
  "projectType": "go",
  "targets": [
    {
      "name": "api",
      "type": "executable",
      "main": "cmd/api/main.go",
      "output": "bin/api",
      "command": "go build -o bin/api cmd/api/main.go",
      "watchPaths": ["cmd/api/**/*.go", "pkg/**/*.go"],
      "excludePaths": ["**/*_test.go"],
      "env": {
        "CGO_ENABLED": "0"
      }
    }
  ],
  "watchmanConfig": {
    "settlingDelay": 200,
    "excludeDirs": [".git", "vendor", "node_modules"]
  }
}
```

### Target Types

| Type | Description | Use Case |
|------|-------------|----------|
| `executable` | Command-line programs | CLIs, servers, tools |
| `library` | Shared/static libraries | .so, .a, .dylib files |
| `test` | Test suites | Unit/integration tests |
| `app-bundle` | macOS/iOS applications | .app bundles |
| `docker` | Container images | Dockerfile builds |
| `custom` | Any command | Scripts, generators |

### Project Types

Poltergeist auto-detects these project types:

- `go` - Go modules (go.mod)
- `rust` - Cargo projects (Cargo.toml)
- `node` - Node.js/TypeScript (package.json)
- `python` - Python projects (setup.py, pyproject.toml)
- `swift` - Swift packages (Package.swift)
- `cmake` - CMake projects (CMakeLists.txt)
- `make` - Makefile projects
- `mixed` - Multi-language projects

## Advanced Features

### Intelligent Build Prioritization

Poltergeist tracks which files you edit and prioritizes builds accordingly:

```json
{
  "priorityConfig": {
    "baseScore": 100,
    "recentEditBoost": 50,
    "dependencyMultiplier": 1.5,
    "maxHistorySize": 100
  }
}
```

### Performance Profiles

Choose a profile based on your system:

- **`aggressive`** - Minimum delays, maximum responsiveness
- **`balanced`** - Good performance with reasonable CPU usage (default)
- **`conservative`** - Lower CPU usage, longer delays

### Watchman Integration

The Go implementation includes a complete Watchman client with automatic fallback to fsnotify:

```go
// Automatic detection and fallback
client := watchman.NewClient(logger)
err := client.Connect(ctx)

// Subscribe to changes
client.Subscribe(root, name, config, callback, exclusions)
```

## Architecture

```
poltergeist/
├── cmd/
│   ├── poltergeist/    # Main daemon
│   └── polter/          # Execution wrapper
├── pkg/
│   ├── builders/        # Build target implementations
│   ├── config/          # Configuration management
│   ├── daemon/          # Background daemon
│   ├── notifier/        # System notifications
│   ├── priority/        # Build prioritization
│   ├── state/           # State management
│   ├── types/           # Shared types
│   └── watchman/        # File watching (Watchman + fsnotify)
└── examples/            # Example configurations
```

## Examples

See the `examples/` directory for complete configurations:

- **Go API Server**: REST API with hot reload
- **React + Go**: Full-stack application
- **Rust CLI**: Command-line tool with tests
- **Swift Package**: iOS/macOS library
- **Python ML**: Machine learning project
- **Docker Compose**: Multi-container setup

## macOS Monitor App

A native macOS menu bar application is available for monitoring all Poltergeist instances. See [apps/mac/README.md](apps/mac/README.md) for details.

## Development

### Requirements

- Go 1.22 or later
- Watchman (optional, falls back to fsnotify)
- macOS 14+ for the monitor app

### Building

```bash
# Build everything
make build

# Run tests
make test

# Install locally
make install

# Clean artifacts
make clean
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/watchman

# Run benchmarks
go test -bench=. ./...
```

## Troubleshooting

### Common Issues

**Watchman not found**
- Install Watchman: `brew install watchman` (macOS)
- Poltergeist will automatically fall back to fsnotify

**Builds not triggering**
- Check `poltergeist status` for errors
- Verify watch patterns in config
- Check exclusions aren't too broad

**Permission denied**
- Ensure output directories exist
- Check file permissions
- On macOS, grant Terminal full disk access

**High CPU usage**
- Switch to `conservative` performance profile
- Increase `settlingDelay` in config
- Add more exclusions

### Debug Mode

```bash
# Enable debug logging
POLTERGEIST_DEBUG=1 poltergeist haunt

# Verbose output
poltergeist haunt --verbose

# Dry run for init
poltergeist init --dry-run
```

## Migration from TypeScript Version

The Go version maintains full compatibility with existing configurations:

1. Configuration files are identical
2. State files in `/tmp/poltergeist/` are compatible
3. Command-line interface is the same
4. The `polter` wrapper works identically

Key improvements in the Go version:
- 5-10x faster startup time
- 50% less memory usage
- Native binary (no Node.js required)
- Better Watchman integration with fallback
- Improved build prioritization algorithm

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Original TypeScript implementation by [@steipete](https://github.com/steipete)
- [Watchman](https://facebook.github.io/watchman/) by Meta
- [fsnotify](https://github.com/fsnotify/fsnotify) for fallback file watching
- The Go community for excellent libraries and tools

---

<div align="center">

Made with 👻 by the Poltergeist team

[Report Bug](https://github.com/steipete/poltergeist/issues) • [Request Feature](https://github.com/steipete/poltergeist/issues) • [Documentation](https://github.com/steipete/poltergeist/wiki)

</div>