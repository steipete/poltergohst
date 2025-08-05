# Go-Watchman Bindings Implementation

This package provides file watching capabilities with two implementations:
1. **Watchman Client** - Communicates with Facebook's Watchman service
2. **FSNotify Fallback** - Uses native Go fsnotify when Watchman is unavailable

## Architecture Overview

```
┌─────────────────┐
│  Poltergeist    │
└────────┬────────┘
         │
    ┌────▼────┐
    │ Watchman │
    │ Interface│
    └────┬────┘
         │
    ┌────┴─────────────┐
    ▼                  ▼
┌─────────┐      ┌──────────┐
│ Watchman │      │ FSNotify │
│  Client  │      │ Fallback │
└─────┬────┘      └────┬─────┘
      │                │
      ▼                ▼
┌──────────┐     ┌───────────┐
│ Watchman │     │ OS Native │
│  Daemon  │     │ FS Events │
└──────────┘     └───────────┘
```

## Watchman Protocol

Watchman uses a JSON-based protocol over Unix domain sockets (or named pipes on Windows):

### 1. Connection
```go
// Watchman connects via Unix socket
const watchmanSockPath = "/usr/local/var/run/watchman/{user}-state/sock"

type WatchmanConnection struct {
    conn net.Conn
    encoder *json.Encoder
    decoder *json.Decoder
}

func connectToWatchman() (*WatchmanConnection, error) {
    // Find socket path
    sockPath := getWatchmanSocket()
    
    // Connect to Unix socket
    conn, err := net.Dial("unix", sockPath)
    if err != nil {
        return nil, err
    }
    
    return &WatchmanConnection{
        conn: conn,
        encoder: json.NewEncoder(conn),
        decoder: json.NewDecoder(conn),
    }, nil
}
```

### 2. Protocol Messages

Watchman uses PDU (Protocol Data Unit) format:
```go
// PDU Format: [header][json_data]
// Header contains optional fields and JSON data length

type WatchmanPDU struct {
    Version  int         `json:"version"`
    Capabilities []string `json:"capabilities,omitempty"`
}

type WatchmanCommand struct {
    Command []interface{} `json:"cmd"`
}

type WatchmanResponse struct {
    Version      string                 `json:"version,omitempty"`
    Error        string                 `json:"error,omitempty"`
    Warning      string                 `json:"warning,omitempty"`
    Clock        string                 `json:"clock,omitempty"`
    Files        []FileInfo            `json:"files,omitempty"`
    Root         string                 `json:"root,omitempty"`
    Subscription string                 `json:"subscription,omitempty"`
}
```

### 3. Core Commands

#### Watch Project
```go
func (w *WatchmanConnection) WatchProject(path string) error {
    // Send watch-project command
    cmd := WatchmanCommand{
        Command: []interface{}{"watch-project", path},
    }
    
    if err := w.encoder.Encode(cmd); err != nil {
        return err
    }
    
    // Read response
    var resp WatchmanResponse
    if err := w.decoder.Decode(&resp); err != nil {
        return err
    }
    
    if resp.Error != "" {
        return fmt.Errorf("watchman error: %s", resp.Error)
    }
    
    return nil
}
```

#### Subscribe to Changes
```go
func (w *WatchmanConnection) Subscribe(root, name string, query map[string]interface{}) error {
    // Build subscription command
    cmd := WatchmanCommand{
        Command: []interface{}{
            "subscribe",
            root,
            name,
            query,
        },
    }
    
    // Query example:
    // {
    //   "expression": ["anyof",
    //     ["match", "*.go"],
    //     ["match", "*.mod"]
    //   ],
    //   "fields": ["name", "size", "mtime_ms", "exists", "type"]
    // }
    
    return w.encoder.Encode(cmd)
}
```

### 4. Receiving File Events

```go
func (w *WatchmanConnection) ReadEvents(callback func(FileEvent)) {
    for {
        var resp WatchmanResponse
        if err := w.decoder.Decode(&resp); err != nil {
            log.Printf("Error reading watchman response: %v", err)
            break
        }
        
        // Process subscription updates
        if resp.Subscription != "" {
            for _, file := range resp.Files {
                event := FileEvent{
                    Path:     filepath.Join(resp.Root, file.Name),
                    Type:     determineEventType(file),
                    IsDir:    file.Type == "d",
                    ModTime:  time.Unix(0, file.MTimeMs*int64(time.Millisecond)),
                }
                callback(event)
            }
        }
    }
}
```

## FSNotify Fallback Implementation

When Watchman is not available, we fall back to fsnotify:

```go
type FSNotifyWatcher struct {
    watcher  *fsnotify.Watcher
    patterns []string
    logger   logger.Logger
}

func NewFSNotifyWatcher(patterns []string, log logger.Logger) (*FSNotifyWatcher, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }
    
    return &FSNotifyWatcher{
        watcher:  watcher,
        patterns: patterns,
        logger:   log,
    }, nil
}

func (f *FSNotifyWatcher) Watch(root string, callback func(FileEvent)) error {
    // Add root directory
    if err := f.watcher.Add(root); err != nil {
        return err
    }
    
    // Recursively add subdirectories
    err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() && !f.shouldExclude(path) {
            return f.watcher.Add(path)
        }
        return nil
    })
    
    if err != nil {
        return err
    }
    
    // Process events
    go func() {
        for {
            select {
            case event, ok := <-f.watcher.Events:
                if !ok {
                    return
                }
                if f.matchesPattern(event.Name) {
                    callback(convertFSNotifyEvent(event))
                }
                
            case err, ok := <-f.watcher.Errors:
                if !ok {
                    return
                }
                f.logger.Error("Watch error:", err)
            }
        }
    }()
    
    return nil
}
```

## Query Language

Watchman uses a powerful query expression language:

```go
// Basic expressions
type Expression interface{}

// Match files by pattern
func Match(pattern string) Expression {
    return []interface{}{"match", pattern, "wholename"}
}

// Match by type
func Type(t string) Expression {
    return []interface{}{"type", t} // "f" for file, "d" for directory
}

// Combine expressions
func AllOf(exprs ...Expression) Expression {
    return append([]interface{}{"allof"}, exprs...)
}

func AnyOf(exprs ...Expression) Expression {
    return append([]interface{}{"anyof"}, exprs...)
}

func Not(expr Expression) Expression {
    return []interface{}{"not", expr}
}

// Example: Watch Go files but exclude tests
query := map[string]interface{}{
    "expression": AllOf(
        Match("*.go"),
        Not(Match("*_test.go")),
    ),
    "fields": []string{"name", "size", "mtime_ms", "exists"},
}
```

## Complete Implementation Example

```go
type WatchmanClient struct {
    conn       *WatchmanConnection
    fallback   *FSNotifyWatcher
    useWatchman bool
    logger     logger.Logger
    mu         sync.RWMutex
}

func NewWatchmanClient(log logger.Logger) *WatchmanClient {
    client := &WatchmanClient{
        logger: log,
    }
    
    // Try to connect to Watchman
    if conn, err := connectToWatchman(); err == nil {
        client.conn = conn
        client.useWatchman = true
        log.Info("Connected to Watchman")
    } else {
        log.Info("Watchman not available, using fsnotify fallback")
        client.useWatchman = false
    }
    
    return client
}

func (c *WatchmanClient) Watch(root string, patterns []string, callback func(FileEvent)) error {
    if c.useWatchman {
        // Use Watchman
        if err := c.conn.WatchProject(root); err != nil {
            return err
        }
        
        // Build query from patterns
        var expressions []Expression
        for _, pattern := range patterns {
            expressions = append(expressions, Match(pattern))
        }
        
        query := map[string]interface{}{
            "expression": AnyOf(expressions...),
            "fields": []string{"name", "size", "mtime_ms", "exists", "type"},
        }
        
        // Subscribe
        if err := c.conn.Subscribe(root, "poltergeist", query); err != nil {
            return err
        }
        
        // Read events in background
        go c.conn.ReadEvents(callback)
        
    } else {
        // Use fsnotify fallback
        watcher, err := NewFSNotifyWatcher(patterns, c.logger)
        if err != nil {
            return err
        }
        c.fallback = watcher
        return c.fallback.Watch(root, callback)
    }
    
    return nil
}
```

## Performance Optimizations

### 1. Clock Synchronization
Watchman uses clock tokens to avoid missing events:
```go
type ClockSpec struct {
    Clock string `json:"clock,omitempty"`
}

func (w *WatchmanConnection) QuerySince(root string, clock string) ([]FileInfo, string, error) {
    cmd := WatchmanCommand{
        Command: []interface{}{
            "query",
            root,
            map[string]interface{}{
                "since": clock,
                "fields": []string{"name", "size", "mtime_ms"},
            },
        },
    }
    // ... send and receive
}
```

### 2. Settling Period
Avoid triggering on rapid changes:
```go
type SettlingWatcher struct {
    base     WatchmanClient
    delay    time.Duration
    pending  map[string]time.Time
    mu       sync.Mutex
}

func (s *SettlingWatcher) handleEvent(event FileEvent) {
    s.mu.Lock()
    s.pending[event.Path] = time.Now()
    s.mu.Unlock()
    
    time.AfterFunc(s.delay, func() {
        s.mu.Lock()
        if lastTime, exists := s.pending[event.Path]; exists {
            if time.Since(lastTime) >= s.delay {
                delete(s.pending, event.Path)
                s.mu.Unlock()
                s.callback(event)
                return
            }
        }
        s.mu.Unlock()
    })
}
```

## Installation & Setup

### macOS
```bash
brew install watchman
```

### Linux
```bash
# Ubuntu/Debian
sudo apt-get install watchman

# From source
git clone https://github.com/facebook/watchman.git
cd watchman
./autogen.sh
./configure
make
sudo make install
```

### Configuration
```json
// .watchmanconfig
{
  "ignore_dirs": ["node_modules", ".git", "vendor"],
  "settle": 20,
  "recrawl_on_startup": true
}
```

## Testing

The implementation includes both unit tests and integration tests that work with or without Watchman:

```go
func TestWatchmanClient(t *testing.T) {
    client := NewWatchmanClient(logger.New())
    
    if client.useWatchman {
        t.Log("Testing with Watchman")
    } else {
        t.Log("Testing with fsnotify fallback")
    }
    
    // Test watching
    err := client.Watch("/tmp/test", []string{"*.txt"}, func(event FileEvent) {
        t.Logf("Event: %+v", event)
    })
    
    if err != nil {
        t.Fatalf("Watch failed: %v", err)
    }
}
```

## Advantages of Watchman

1. **Efficiency**: Watchman maintains a persistent view of the filesystem
2. **Reliability**: Handles edge cases like atomic saves, renames
3. **Performance**: Coalesces events, reduces syscalls
4. **Cross-platform**: Consistent behavior across OS
5. **Query Language**: Powerful filtering and expression system
6. **Clock Tokens**: Never miss events between queries

## Fallback Strategy

The implementation gracefully degrades:
1. Try to connect to Watchman
2. If unavailable, use fsnotify
3. Abstract differences behind common interface
4. Log which backend is being used

This ensures the build system works whether or not Watchman is installed, while taking advantage of Watchman's superior capabilities when available.