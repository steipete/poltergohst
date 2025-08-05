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
