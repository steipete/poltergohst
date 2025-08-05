// Package utils provides utility functions
package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileSystemUtils provides file system operations
type FileSystemUtils struct{}

// NewFileSystemUtils creates a new filesystem utils instance
func NewFileSystemUtils() *FileSystemUtils {
	return &FileSystemUtils{}
}

// Exists checks if a path exists
func (f *FileSystemUtils) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDirectory checks if a path is a directory
func (f *FileSystemUtils) IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// CreateDirectory creates a directory with all parents
func (f *FileSystemUtils) CreateDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// RemoveDirectory removes a directory and all contents
func (f *FileSystemUtils) RemoveDirectory(path string) error {
	return os.RemoveAll(path)
}

// CopyFile copies a file from src to dst
func (f *FileSystemUtils) CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination directory if needed
	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy contents
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	// Copy permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

// MoveFile moves a file from src to dst
func (f *FileSystemUtils) MoveFile(src, dst string) error {
	// Try rename first (fastest if on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy and delete
	if err := f.CopyFile(src, dst); err != nil {
		return err
	}

	return os.Remove(src)
}

// ReadFile reads the entire file
func (f *FileSystemUtils) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes data to a file
func (f *FileSystemUtils) WriteFile(path string, data []byte) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write atomically using temp file
	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tempFile, path)
}

// WalkDirectory walks a directory tree
func (f *FileSystemUtils) WalkDirectory(path string, callback func(path string, isDir bool) error) error {
	return filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return callback(path, info.IsDir())
	})
}

// GetFileHash returns MD5 hash of a file
func (f *FileSystemUtils) GetFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// FindFiles finds files matching a pattern
func (f *FileSystemUtils) FindFiles(root string, pattern string) ([]string, error) {
	var matches []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return err
		}

		if matched {
			matches = append(matches, path)
		}

		return nil
	})

	return matches, err
}

// GetRelativePath returns the relative path from base to target
func (f *FileSystemUtils) GetRelativePath(base, target string) (string, error) {
	return filepath.Rel(base, target)
}

// NormalizePath normalizes a file path
func (f *FileSystemUtils) NormalizePath(path string) string {
	// Clean the path
	path = filepath.Clean(path)

	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Make absolute if relative
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}

	return path
}

// EnsureDirectory ensures a directory exists
func EnsureDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirectoryExists checks if a directory exists
func DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// CleanPath cleans and normalizes a path
func CleanPath(path string) string {
	return filepath.Clean(path)
}

// JoinPaths joins path segments
func JoinPaths(segments ...string) string {
	return filepath.Join(segments...)
}

// GetFileSize returns the size of a file
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// GetFileModTime returns the modification time of a file
func GetFileModTime(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// TouchFile creates an empty file or updates its timestamp
func TouchFile(path string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create or update file
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

// ListDirectory lists files in a directory
func ListDirectory(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

// GetWorkingDirectory returns the current working directory
func GetWorkingDirectory() (string, error) {
	return os.Getwd()
}

// ChangeDirectory changes the current working directory
func ChangeDirectory(path string) error {
	return os.Chdir(path)
}

// GetTempDir returns the system temp directory
func GetTempDir() string {
	return os.TempDir()
}

// CreateTempFile creates a temporary file
func CreateTempFile(pattern string) (*os.File, error) {
	return os.CreateTemp("", pattern)
}

// CreateTempDir creates a temporary directory
func CreateTempDir(pattern string) (string, error) {
	return os.MkdirTemp("", pattern)
}

// RemoveFile removes a file
func RemoveFile(path string) error {
	return os.Remove(path)
}

// RemoveAll removes a path and all its contents
func RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// IsSymlink checks if a path is a symbolic link
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// CreateSymlink creates a symbolic link
func CreateSymlink(target, link string) error {
	return os.Symlink(target, link)
}

// ReadSymlink reads the target of a symbolic link
func ReadSymlink(path string) (string, error) {
	return os.Readlink(path)
}

// GetExecutablePath returns the path of the current executable
func GetExecutablePath() (string, error) {
	return os.Executable()
}

// SetFilePermissions sets file permissions
func SetFilePermissions(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

// CopyDirectory copies a directory recursively
func CopyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Construct destination path
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			// Create directory
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		utils := &FileSystemUtils{}
		return utils.CopyFile(path, dstPath)
	})
}

// GetDirectorySize calculates the total size of a directory
func GetDirectorySize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// FormatBytes formats bytes into human-readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
