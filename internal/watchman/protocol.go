// Package watchman provides Watchman protocol implementation
package watchman

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Protocol constants
const (
	// Unix socket paths
	unixSockPathTemplate = "%s/%s-state/sock"
	
	// Windows named pipe
	windowsPipeTemplate = "\\\\.\\pipe\\watchman-%s"
)

// WatchmanPDU represents the protocol data unit header
type WatchmanPDU struct {
	Version      int      `json:"version"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// WatchmanCommand represents a command to send to Watchman
type WatchmanCommand []interface{}

// WatchmanResponse represents a response from Watchman
type WatchmanResponse struct {
	Version         string                 `json:"version,omitempty"`
	Error           string                 `json:"error,omitempty"`
	Warning         string                 `json:"warning,omitempty"`
	Clock           string                 `json:"clock,omitempty"`
	IsFreshInstance bool                   `json:"is_fresh_instance,omitempty"`
	Files           []WatchmanFile         `json:"-"` // Custom unmarshal
	FilesRaw        json.RawMessage        `json:"files,omitempty"` // Raw for flexible parsing
	Root            string                 `json:"root,omitempty"`
	Subscription    string                 `json:"subscription,omitempty"`
	Unilateral      bool                   `json:"unilateral,omitempty"`
	Log             string                 `json:"log,omitempty"`
	Watch           string                 `json:"watch,omitempty"`
	RelativeRoot    string                 `json:"relative_path,omitempty"`
}

// UnmarshalJSON custom unmarshaler for WatchmanResponse
func (wr *WatchmanResponse) UnmarshalJSON(data []byte) error {
	type Alias WatchmanResponse
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(wr),
	}
	
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	
	// Parse files if present
	if len(wr.FilesRaw) > 0 {
		// Try to parse as array of file objects
		var files []WatchmanFile
		if err := json.Unmarshal(wr.FilesRaw, &files); err == nil {
			wr.Files = files
		} else {
			// Try to parse as array of strings (name-only query)
			var names []string
			if err := json.Unmarshal(wr.FilesRaw, &names); err == nil {
				wr.Files = make([]WatchmanFile, len(names))
				for i, name := range names {
					wr.Files[i] = WatchmanFile{Name: name}
				}
			}
		}
	}
	
	return nil
}

// WatchmanFile represents file information from Watchman
type WatchmanFile struct {
	Name     string      `json:"name"`
	Size     int64       `json:"size,omitempty"`
	Mode     int32       `json:"mode,omitempty"`
	UID      int         `json:"uid,omitempty"`
	GID      int         `json:"gid,omitempty"`
	MTimeMs  int64       `json:"mtime_ms,omitempty"`
	CTimeMs  int64       `json:"ctime_ms,omitempty"`
	Exists   bool        `json:"exists"`
	Type     string      `json:"type,omitempty"` // 'f' for file, 'd' for directory, 'l' for symlink
	New      bool        `json:"new,omitempty"`
}

// Expression types for Watchman queries
type Expression interface{}

// MatchExpression matches files by pattern
func MatchExpression(pattern string, wholename bool) Expression {
	if wholename {
		return []interface{}{"match", pattern, "wholename"}
	}
	return []interface{}{"match", pattern}
}

// TypeExpression matches by file type
func TypeExpression(fileType string) Expression {
	return []interface{}{"type", fileType}
}

// AllOfExpression combines expressions with AND
func AllOfExpression(exprs ...Expression) Expression {
	result := []interface{}{"allof"}
	for _, expr := range exprs {
		result = append(result, expr)
	}
	return result
}

// AnyOfExpression combines expressions with OR
func AnyOfExpression(exprs ...Expression) Expression {
	result := []interface{}{"anyof"}
	for _, expr := range exprs {
		result = append(result, expr)
	}
	return result
}

// NotExpression negates an expression
func NotExpression(expr Expression) Expression {
	return []interface{}{"not", expr}
}

// SinceExpression matches files changed since a clock value
func SinceExpression(clock string) Expression {
	return []interface{}{"since", clock}
}

// WatchmanConnection represents a connection to Watchman
type WatchmanConnection struct {
	conn     net.Conn
	reader   *bufio.Reader
	writer   *bufio.Writer
	sockPath string
}

// Connect establishes a connection to Watchman
func Connect() (*WatchmanConnection, error) {
	sockPath, err := getWatchmanSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to find watchman socket: %w", err)
	}
	
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to watchman: %w", err)
	}
	
	wc := &WatchmanConnection{
		conn:     conn,
		reader:   bufio.NewReader(conn),
		writer:   bufio.NewWriter(conn),
		sockPath: sockPath,
	}
	
	// Watchman doesn't send anything initially - it waits for commands
	// We'll just return the connection ready to use
	
	return wc, nil
}

// Close closes the connection
func (wc *WatchmanConnection) Close() error {
	return wc.conn.Close()
}

// Send sends a command to Watchman
func (wc *WatchmanConnection) Send(cmd WatchmanCommand) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	
	// Write the JSON data followed by newline
	if _, err := wc.writer.Write(data); err != nil {
		return err
	}
	if err := wc.writer.WriteByte('\n'); err != nil {
		return err
	}
	
	return wc.writer.Flush()
}

// Receive receives a response from Watchman
func (wc *WatchmanConnection) Receive() (*WatchmanResponse, error) {
	line, err := wc.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	
	var resp WatchmanResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, err
	}
	
	if resp.Error != "" {
		return &resp, fmt.Errorf("watchman error: %s", resp.Error)
	}
	
	return &resp, nil
}

// SendReceive sends a command and waits for response
func (wc *WatchmanConnection) SendReceive(cmd WatchmanCommand) (*WatchmanResponse, error) {
	if err := wc.Send(cmd); err != nil {
		return nil, err
	}
	return wc.Receive()
}

// readInitialPDU reads the initial PDU header from Watchman
func (wc *WatchmanConnection) readInitialPDU() error {
	// Try to read PDU v2 header
	var header [16]byte
	if _, err := io.ReadFull(wc.reader, header[:]); err != nil {
		// Might be using JSON protocol, not binary PDU
		// Reset reader and continue
		wc.reader = bufio.NewReader(io.MultiReader(bytes.NewReader(header[:]), wc.conn))
		return nil
	}
	
	// Check for PDU magic
	if bytes.Equal(header[:4], []byte{0x00, 0x01, 0x05, 0x00}) {
		// Binary PDU protocol
		// Read capabilities length
		capLen := binary.LittleEndian.Uint32(header[12:16])
		if capLen > 0 {
			capData := make([]byte, capLen)
			if _, err := io.ReadFull(wc.reader, capData); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// getWatchmanSocket finds the Watchman socket path
func getWatchmanSocket() (string, error) {
	// Try to get socket path from watchman
	cmd := exec.Command("watchman", "get-sockname")
	output, err := cmd.Output()
	if err == nil {
		var result struct {
			Sockname string `json:"sockname"`
		}
		if err := json.Unmarshal(output, &result); err == nil && result.Sockname != "" {
			return result.Sockname, nil
		}
	}
	
	// Fall back to default locations
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(windowsPipeTemplate, os.Getenv("USERNAME")), nil
	}
	
	// Unix-like systems
	stateDir := os.Getenv("WATCHMAN_STATE_DIR")
	if stateDir == "" {
		stateDir = "/usr/local/var/run/watchman"
		if _, err := os.Stat(stateDir); os.IsNotExist(err) {
			// Try tmp directory
			stateDir = filepath.Join(os.TempDir(), ".watchman")
		}
	}
	
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}
	
	sockPath := fmt.Sprintf(unixSockPathTemplate, stateDir, user)
	return sockPath, nil
}

// Query represents a Watchman query
type Query struct {
	Expression Expression     `json:"expression,omitempty"`
	Fields     []string       `json:"fields,omitempty"`
	Since      string         `json:"since,omitempty"`
	Suffix     []string       `json:"suffix,omitempty"`
	RelativeRoot string       `json:"relative_root,omitempty"`
}

// SubscriptionQuery represents a subscription configuration
type SubscriptionQuery struct {
	Expression   Expression     `json:"expression,omitempty"`
	Fields       []string       `json:"fields,omitempty"`
	Since        string         `json:"since,omitempty"`
	DeferVCS     bool          `json:"defer_vcs,omitempty"`
	Drop         []string       `json:"drop,omitempty"`
	RelativeRoot string         `json:"relative_root,omitempty"`
	Empty        bool          `json:"empty_on_fresh_instance,omitempty"`
}

// WatchProject watches a project directory
func (wc *WatchmanConnection) WatchProject(path string) (*WatchmanResponse, error) {
	return wc.SendReceive(WatchmanCommand{"watch-project", path})
}

// Subscribe creates a subscription for file changes
func (wc *WatchmanConnection) Subscribe(root, name string, query SubscriptionQuery) (*WatchmanResponse, error) {
	return wc.SendReceive(WatchmanCommand{"subscribe", root, name, query})
}

// Unsubscribe removes a subscription
func (wc *WatchmanConnection) Unsubscribe(root, name string) error {
	_, err := wc.SendReceive(WatchmanCommand{"unsubscribe", root, name})
	return err
}

// Query performs a one-time query
func (wc *WatchmanConnection) Query(root string, query Query) (*WatchmanResponse, error) {
	return wc.SendReceive(WatchmanCommand{"query", root, query})
}

// Clock gets the current clock value
func (wc *WatchmanConnection) Clock(root string) (string, error) {
	resp, err := wc.SendReceive(WatchmanCommand{"clock", root})
	if err != nil {
		return "", err
	}
	return resp.Clock, nil
}

// Version gets the Watchman version
func (wc *WatchmanConnection) Version() (string, error) {
	resp, err := wc.SendReceive(WatchmanCommand{"version"})
	if err != nil {
		return "", err
	}
	return resp.Version, nil
}

// Trigger creates a trigger
func (wc *WatchmanConnection) Trigger(root, name string, query Query, command []string) error {
	_, err := wc.SendReceive(WatchmanCommand{
		"trigger",
		root,
		map[string]interface{}{
			"name":       name,
			"expression": query.Expression,
			"command":    command,
		},
	})
	return err
}

// TriggerDel deletes a trigger
func (wc *WatchmanConnection) TriggerDel(root, name string) error {
	_, err := wc.SendReceive(WatchmanCommand{"trigger-del", root, name})
	return err
}

// TriggerList lists all triggers
func (wc *WatchmanConnection) TriggerList(root string) (*WatchmanResponse, error) {
	return wc.SendReceive(WatchmanCommand{"trigger-list", root})
}

// GetConfig gets Watchman configuration
func (wc *WatchmanConnection) GetConfig(root string) (map[string]interface{}, error) {
	_, err := wc.SendReceive(WatchmanCommand{"get-config", root})
	if err != nil {
		return nil, err
	}
	
	// Parse config from response
	// This would need proper unmarshaling based on actual response structure
	config := make(map[string]interface{})
	return config, nil
}

// SetConfig sets Watchman configuration
func (wc *WatchmanConnection) SetConfig(root string, config map[string]interface{}) error {
	_, err := wc.SendReceive(WatchmanCommand{"set-config", root, config})
	return err
}

// Shutdown requests Watchman to shutdown
func (wc *WatchmanConnection) Shutdown() error {
	_, err := wc.SendReceive(WatchmanCommand{"shutdown-server"})
	return err
}

// FileEvent represents a file change event
type FileEvent struct {
	Path      string
	Type      EventType
	IsDir     bool
	Size      int64
	Mode      os.FileMode
	ModTime   time.Time
}

// EventType represents the type of file event
type EventType int

const (
	FileCreated EventType = iota
	FileModified
	FileDeleted
	FileRenamed
)

// ConvertWatchmanFile converts a WatchmanFile to FileEvent
func ConvertWatchmanFile(root string, wf WatchmanFile) FileEvent {
	event := FileEvent{
		Path:    filepath.Join(root, wf.Name),
		IsDir:   wf.Type == "d",
		Size:    wf.Size,
		ModTime: time.Unix(0, wf.MTimeMs*int64(time.Millisecond)),
	}
	
	// Determine event type
	if !wf.Exists {
		event.Type = FileDeleted
	} else if wf.New {
		event.Type = FileCreated
	} else {
		event.Type = FileModified
	}
	
	// Convert mode
	if wf.Mode != 0 {
		event.Mode = os.FileMode(wf.Mode)
	}
	
	return event
}