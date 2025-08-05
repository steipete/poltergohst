#!/bin/bash
# Package consolidation script for Poltergeist
# Following Go's "start simple" philosophy

set -e

echo "🔧 Starting package consolidation..."

# Create internal directory structure
echo "📁 Creating internal directory structure..."
mkdir -p internal/engine
mkdir -p internal/builders  
mkdir -p internal/watchman
mkdir -p internal/state

# Phase 1: Move and consolidate engine packages
echo "📦 Consolidating engine packages..."

# Create consolidated engine package
cat > internal/engine/engine.go << 'EOF'
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
EOF

# Copy core engine files
cp pkg/poltergeist/poltergeist.go internal/engine/poltergeist.go 2>/dev/null || true
cp pkg/poltergeist/factory.go internal/engine/factory.go 2>/dev/null || true
cp pkg/poltergeist/safegroup.go internal/engine/safegroup.go 2>/dev/null || true
cp pkg/poltergeist/poltergeist_test.go internal/engine/poltergeist_test.go 2>/dev/null || true

# Merge queue files
echo "// Build queue implementation (merged from pkg/queue)" > internal/engine/queue_impl.go
echo "package engine" >> internal/engine/queue_impl.go
echo "" >> internal/engine/queue_impl.go
tail -n +3 pkg/queue/queue.go >> internal/engine/queue_impl.go 2>/dev/null || true

# Merge priority engine
echo "// Priority engine implementation (merged from pkg/priority)" > internal/engine/priority.go
echo "package engine" >> internal/engine/priority.go
echo "" >> internal/engine/priority.go
if [ -f pkg/priority/engine.go ]; then
    tail -n +3 pkg/priority/engine.go >> internal/engine/priority.go
elif [ -f pkg/queue/priority.go ]; then
    tail -n +3 pkg/queue/priority.go >> internal/engine/priority.go
fi

# Process manager
echo "// Process lifecycle management (merged from pkg/process)" > internal/engine/process_manager.go
echo "package engine" >> internal/engine/process_manager.go
echo "" >> internal/engine/process_manager.go
tail -n +3 pkg/process/manager.go >> internal/engine/process_manager.go 2>/dev/null || true

# Context utilities
if [ -f pkg/context/context.go ]; then
    echo "// Context enrichment utilities (merged from pkg/context)" > internal/engine/context_utils.go
    echo "package engine" >> internal/engine/context_utils.go
    echo "" >> internal/engine/context_utils.go
    tail -n +3 pkg/context/context.go >> internal/engine/context_utils.go
fi

# Phase 2: Move builders
echo "📦 Moving builders..."
cp -r pkg/builders/* internal/builders/ 2>/dev/null || true

# Phase 3: Move watchman
echo "📦 Moving watchman..."
cp -r pkg/watchman/* internal/watchman/ 2>/dev/null || true

# Phase 4: Move state management
echo "📦 Moving state management..."
cp -r pkg/state/* internal/state/ 2>/dev/null || true

# Phase 5: Create public config package
echo "📦 Creating public config package..."
mkdir -p pkg/config
cp pkg/types/types.go pkg/config/types.go 2>/dev/null || true
cp pkg/types/target_*.go pkg/config/ 2>/dev/null || true
if [ -f pkg/config/config.go ]; then
    cp pkg/config/config.go pkg/config/config.go.bak
fi

# Phase 6: Update package declarations
echo "📝 Updating package declarations..."

# Update all files in internal/engine to use package engine
for file in internal/engine/*.go; do
    if [ -f "$file" ] && [ "$(basename $file)" != "engine.go" ]; then
        sed -i.bak 's/^package poltergeist$/package engine/' "$file"
        sed -i.bak 's/^package queue$/package engine/' "$file"
        sed -i.bak 's/^package process$/package engine/' "$file"
        sed -i.bak 's/^package context$/package engine/' "$file"
        sed -i.bak 's/^package priority$/package engine/' "$file"
        rm "${file}.bak" 2>/dev/null || true
    fi
done

# Update imports (this is complex and would need more sophisticated tooling)
echo "⚠️  Import updates need to be done manually or with goimports"

# Phase 7: Remove utils package (distribute functions)
echo "🗑️  Utils package needs manual distribution to usage points"

# Create migration guide
cat > PACKAGE_MIGRATION.md << 'EOF'
# Package Migration Guide

## Import Changes Required

### Old → New Mappings

| Old Import | New Import |
|------------|------------|
| `pkg/poltergeist` | `internal/engine` |
| `pkg/queue` | `internal/engine` |
| `pkg/process` | `internal/engine` |
| `pkg/context` | `internal/engine` |
| `pkg/priority` | `internal/engine` |
| `pkg/builders` | `internal/builders` |
| `pkg/watchman` | `internal/watchman` |
| `pkg/state` | `internal/state` |
| `pkg/types` | `pkg/config` |

## Manual Steps Required

1. **Update all imports** in existing code
2. **Distribute utils functions** to their usage points
3. **Remove single-implementation interfaces**
4. **Run tests** to ensure everything still works
5. **Delete old packages** once migration is complete

## Testing

```bash
# Update imports
goimports -w .

# Run tests
go test ./...

# Build
go build ./cmd/poltergeist
```
EOF

echo "✅ Package consolidation structure created!"
echo "📋 See PACKAGE_MIGRATION.md for next steps"
echo ""
echo "⚠️  Manual steps required:"
echo "   1. Update imports throughout the codebase"
echo "   2. Distribute utils package functions"
echo "   3. Remove single-implementation interfaces"
echo "   4. Run tests to verify everything works"