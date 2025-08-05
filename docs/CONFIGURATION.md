# Poltergeist Configuration Guide

This guide covers all configuration options for Poltergeist, from automatic detection to advanced customization.

## Table of Contents

- [Quick Start](#quick-start)
- [Configuration File Structure](#configuration-file-structure)
- [Project Types](#project-types)
- [Target Configuration](#target-configuration)
- [Watch Patterns](#watch-patterns)
- [Performance Profiles](#performance-profiles)
- [Build Prioritization](#build-prioritization)
- [Notifications](#notifications)
- [Advanced Options](#advanced-options)

## Quick Start

### Automatic Configuration

The easiest way to get started is to let Poltergeist analyze your project:

```bash
poltergeist init
```

This command:
1. Detects your project type (Go, Swift, Rust, Node.js, etc.)
2. Identifies build targets
3. Sets up optimal watch patterns
4. Configures appropriate exclusions
5. Creates `poltergeist.config.json`

### Manual Configuration

Create `poltergeist.config.json` in your project root:

```json
{
  "version": "1.0",
  "projectType": "go",
  "targets": [
    {
      "name": "main",
      "type": "executable",
      "buildCommand": "go build -o main cmd/main.go",
      "watchPaths": ["**/*.go"],
      "outputPath": "main"
    }
  ]
}
```

## Configuration File Structure

### Root Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `version` | string | Yes | Configuration version (currently "1.0") |
| `projectType` | string | Yes | Project type (go, rust, node, swift, etc.) |
| `projectName` | string | No | Human-readable project name |
| `performanceProfile` | string | No | Performance profile (aggressive, balanced, conservative) |
| `targets` | array | Yes | Build target definitions |
| `watchmanConfig` | object | No | Watchman-specific settings |
| `buildScheduling` | object | No | Build queue configuration |
| `notifications` | object | No | Notification preferences |

### Complete Example

```json
{
  "version": "1.0",
  "projectType": "go",
  "projectName": "My API Server",
  "performanceProfile": "balanced",
  "targets": [...],
  "watchmanConfig": {...},
  "buildScheduling": {...},
  "notifications": {...}
}
```

## Project Types

Poltergeist supports these project types with smart defaults:

### go
- **Detection**: `go.mod` file
- **Default targets**: main executable, tests
- **Watch patterns**: `**/*.go`, `go.mod`, `go.sum`
- **Exclusions**: `vendor/`, `.git/`

### rust
- **Detection**: `Cargo.toml` file
- **Default targets**: debug/release builds, tests
- **Watch patterns**: `src/**/*.rs`, `Cargo.toml`
- **Exclusions**: `target/`, `.git/`

### node
- **Detection**: `package.json` file
- **Default targets**: build, test, start scripts
- **Watch patterns**: `src/**/*.{js,jsx,ts,tsx}`, `package.json`
- **Exclusions**: `node_modules/`, `dist/`, `build/`

### swift
- **Detection**: `Package.swift` file
- **Default targets**: debug/release builds, tests
- **Watch patterns**: `Sources/**/*.swift`, `Package.swift`
- **Exclusions**: `.build/`, `.swiftpm/`

### python
- **Detection**: `setup.py`, `pyproject.toml`
- **Default targets**: main script, tests
- **Watch patterns**: `**/*.py`, `requirements.txt`
- **Exclusions**: `__pycache__/`, `.venv/`, `venv/`

### cmake
- **Detection**: `CMakeLists.txt`
- **Default targets**: Auto-detected from CMake
- **Watch patterns**: `**/*.{c,cpp,h,hpp}`, `CMakeLists.txt`
- **Exclusions**: `build/`, `cmake-build-*/`

### mixed
- **Detection**: Multiple project markers
- **Default targets**: Combined from detected types
- **Watch patterns**: Union of all types
- **Exclusions**: Union of all types

## Target Configuration

Each target in the `targets` array has these properties:

### Required Properties

```json
{
  "name": "api",
  "type": "executable",
  "buildCommand": "go build -o bin/api ./cmd/api"
}
```

### All Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `name` | string | Yes | Unique target identifier |
| `displayName` | string | No | Human-readable name |
| `type` | string | Yes | Target type (see below) |
| `buildCommand` | string | Yes | Command to build the target |
| `runCommand` | string | No | Command to run after build |
| `outputPath` | string | No | Path to built artifact |
| `watchPaths` | array | No | Paths to watch (glob patterns) |
| `excludePaths` | array | No | Paths to exclude (glob patterns) |
| `env` | object | No | Environment variables |
| `priority` | number | No | Build priority (0-1000) |
| `dependencies` | array | No | Target dependencies |
| `continueOnError` | boolean | No | Continue if build fails |
| `buildOptions` | object | No | Additional build options |

### Target Types

- **executable**: Command-line programs
- **library**: Static/dynamic libraries
- **test**: Test suites
- **app-bundle**: macOS/iOS applications
- **docker**: Container images
- **custom**: Any custom command

### Build Options

```json
{
  "buildOptions": {
    "cleanBeforeBuild": true,
    "timeoutMs": 30000,
    "workingDirectory": "./backend",
    "shell": "/bin/bash",
    "inheritEnv": true
  }
}
```

## Watch Patterns

### Glob Pattern Support

Poltergeist supports standard glob patterns:

- `*` - Match any characters except `/`
- `**` - Match any characters including `/`
- `?` - Match single character
- `[abc]` - Match character set
- `{a,b}` - Match alternatives

### Examples

```json
{
  "watchPaths": [
    "src/**/*.go",           // All Go files under src/
    "cmd/**/main.go",        // All main.go files under cmd/
    "**/*.{go,mod,sum}",     // Go source and module files
    "config/*.yaml",         // YAML configs in config/
    "!**/*_test.go"         // Exclude test files
  ]
}
```

### Performance Tips

1. Be specific with paths to reduce file events
2. Use `excludePaths` for large directories
3. Avoid watching binary/build output directories
4. Use `settlingDelay` to batch rapid changes

## Performance Profiles

### aggressive
```json
{
  "performanceProfile": "aggressive",
  "watchmanConfig": {
    "settlingDelay": 50,
    "maxFileEvents": 5000
  }
}
```
- Minimal delays
- Maximum responsiveness
- Higher CPU usage

### balanced (default)
```json
{
  "performanceProfile": "balanced",
  "watchmanConfig": {
    "settlingDelay": 200,
    "maxFileEvents": 1000
  }
}
```
- Moderate delays
- Good responsiveness
- Reasonable CPU usage

### conservative
```json
{
  "performanceProfile": "conservative",
  "watchmanConfig": {
    "settlingDelay": 500,
    "maxFileEvents": 500
  }
}
```
- Longer delays
- Lower CPU usage
- Better for large projects

## Build Prioritization

### Basic Configuration

```json
{
  "buildScheduling": {
    "parallelization": 2,
    "prioritization": {
      "enabled": true
    }
  }
}
```

### Advanced Configuration

```json
{
  "buildScheduling": {
    "parallelization": 4,
    "prioritization": {
      "enabled": true,
      "dependencyAware": true,
      "defaultPriority": 100,
      "recentEditBoost": 50,
      "dependencyMultiplier": 1.5,
      "maxHistorySize": 100
    }
  }
}
```

### Priority Scores

Targets are prioritized based on:
1. Base priority (0-1000, higher = more important)
2. Recent edit history
3. Dependency relationships
4. Build failure history

## Notifications

### Basic Setup

```json
{
  "notifications": {
    "enabled": true,
    "sound": true,
    "buildSuccess": true,
    "buildFailure": true
  }
}
```

### Advanced Options

```json
{
  "notifications": {
    "enabled": true,
    "sound": true,
    "buildStart": false,
    "buildSuccess": true,
    "buildFailure": true,
    "soundSuccess": "Glass",
    "soundFailure": "Basso",
    "groupBy": "target",
    "rateLimit": {
      "maxPerMinute": 5,
      "cooldownMs": 5000
    }
  }
}
```

## Advanced Options

### Watchman Configuration

```json
{
  "watchmanConfig": {
    "useDefaultExclusions": true,
    "excludeDirs": [
      ".git",
      "node_modules",
      "vendor",
      ".build",
      "target"
    ],
    "settlingDelay": 200,
    "maxFileEvents": 1000,
    "expression": ["allof",
      ["type", "f"],
      ["not", ["match", "*.tmp", "basename"]]
    ]
  }
}
```

### Environment Variables

Use environment variables in configurations:

```json
{
  "env": {
    "API_KEY": "${API_KEY}",
    "DATABASE_URL": "${DATABASE_URL:-postgresql://localhost/dev}",
    "PORT": "${PORT:-3000}"
  }
}
```

### Build Dependencies

Define target dependencies for ordered builds:

```json
{
  "targets": [
    {
      "name": "shared",
      "type": "library",
      "buildCommand": "make libshared.a"
    },
    {
      "name": "app",
      "type": "executable",
      "buildCommand": "make app",
      "dependencies": ["shared"]
    }
  ]
}
```

### Custom Build Scripts

For complex builds, use external scripts:

```json
{
  "targets": [
    {
      "name": "complex-build",
      "type": "custom",
      "buildCommand": "./scripts/build.sh",
      "env": {
        "BUILD_MODE": "production"
      }
    }
  ]
}
```

## Platform-Specific Configuration

### macOS/iOS

```json
{
  "xcode": {
    "scheme": "MyApp",
    "workspace": "MyApp.xcworkspace",
    "configuration": "Release",
    "sdk": "iphoneos",
    "derivedDataPath": "./DerivedData"
  }
}
```

### Docker

```json
{
  "docker": {
    "context": ".",
    "dockerfile": "Dockerfile",
    "tag": "myapp:latest",
    "buildArgs": {
      "VERSION": "${VERSION}"
    }
  }
}
```

## Configuration Validation

Validate your configuration:

```bash
poltergeist validate
```

This checks for:
- Valid JSON syntax
- Required fields
- Valid target types
- Glob pattern syntax
- Circular dependencies

## Best Practices

1. **Start Simple**: Begin with minimal config and add as needed
2. **Use Exclusions**: Exclude large directories to improve performance
3. **Set Priorities**: Higher priority for frequently edited targets
4. **Test Patterns**: Use `poltergeist list` to verify watch patterns
5. **Version Control**: Commit `poltergeist.config.json` to your repo

## Troubleshooting

### Builds Not Triggering

1. Check watch patterns match your files
2. Verify exclusions aren't too broad
3. Run with `--verbose` to see file events
4. Check `poltergeist status` for errors

### High CPU Usage

1. Switch to `conservative` performance profile
2. Increase `settlingDelay`
3. Add more exclusions
4. Reduce `maxFileEvents`

### Notification Issues

1. Check system notification permissions
2. Verify notification settings in config
3. Test with `poltergeist test-notify`

## Migration from TypeScript Version

The Go version uses the same configuration format. Simply copy your existing `poltergeist.config.json` file.

## Examples

See the `examples/` directory for configurations for:
- Go API servers
- Swift packages
- Rust CLI tools
- Node.js applications
- Mixed-language projects
- Docker-based development

## Getting Help

- Run `poltergeist init --help` for initialization options
- Run `poltergeist config --help` for configuration commands
- Check [GitHub Issues](https://github.com/steipete/poltergeist/issues) for known issues
- Join discussions at [GitHub Discussions](https://github.com/steipete/poltergeist/discussions)