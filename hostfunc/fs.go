package hostfunc

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// MountMode defines the permission level for a mount point.
type MountMode int

const (
	// MountReadOnly allows only read operations.
	MountReadOnly MountMode = iota
	// MountReadWrite allows read and write operations to existing files/dirs.
	MountReadWrite
	// MountReadWriteCreate allows read, write, and create operations.
	MountReadWriteCreate
)

const (
	DefaultMaxFileSize   = 10 * 1024 * 1024 // 10MB max file read
	DefaultMaxWriteSize  = 10 * 1024 * 1024 // 10MB max file write
	DefaultMaxPathLength = 4096             // 4KB max path
)

// Mount represents a virtual path mapped to a host path with specific permissions.
type Mount struct {
	VirtualPath string    // Path as seen by sandboxed code (e.g., "/data")
	HostPath    string    // Actual path on host filesystem
	Mode        MountMode // Permission level
}

// FS provides filesystem operations with explicit mount points.
type FS struct {
	mounts       []Mount
	mu           sync.RWMutex
	maxFileSize  int64
	maxWriteSize int64
	maxPathLen   int
}

type FSOption func(*FS)

func WithMaxFileSize(size int64) FSOption {
	return func(f *FS) { f.maxFileSize = size }
}

func WithMaxWriteSize(size int64) FSOption {
	return func(f *FS) { f.maxWriteSize = size }
}

func WithMaxPathLength(length int) FSOption {
	return func(f *FS) { f.maxPathLen = length }
}

// NewFS creates a new filesystem handler with the given mount points.
func NewFS(mounts []Mount, opts ...FSOption) *FS {
	// Normalize and validate mounts
	normalized := make([]Mount, 0, len(mounts))
	for _, m := range mounts {
		// Ensure virtual path starts with / and has no trailing slash
		vp := "/" + strings.Trim(m.VirtualPath, "/")
		// Resolve host path to absolute
		hp, err := filepath.Abs(m.HostPath)
		if err != nil {
			continue
		}
		// Resolve any symlinks in the mount path itself
		// This handles cases like /var -> /private/var on macOS
		if realHP, err := filepath.EvalSymlinks(hp); err == nil {
			hp = realHP
		}
		normalized = append(normalized, Mount{
			VirtualPath: vp,
			HostPath:    hp,
			Mode:        m.Mode,
		})
	}
	f := &FS{
		mounts:       normalized,
		maxFileSize:  DefaultMaxFileSize,
		maxWriteSize: DefaultMaxWriteSize,
		maxPathLen:   DefaultMaxPathLength,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// normalizePath converts a virtual path to a clean, absolute form.
func normalizePath(virtualPath string) string {
	return filepath.Clean("/" + strings.TrimPrefix(virtualPath, "/"))
}

// checkSymlinkEscape verifies a path doesn't escape the mount via symlinks.
// Returns the resolved real path if valid.
func checkSymlinkEscape(absPath, mountBase string) (string, error) {
	realPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		if !strings.HasPrefix(realPath, mountBase) {
			return "", errors.New("permission denied: symlink escape attempt")
		}
		return realPath, nil
	}
	if !os.IsNotExist(err) {
		return "", errors.New("invalid path")
	}

	// Path doesn't exist, check parent for symlink escape
	parentPath := filepath.Dir(absPath)
	if realParent, err := filepath.EvalSymlinks(parentPath); err == nil {
		if !strings.HasPrefix(realParent, mountBase) {
			return "", errors.New("permission denied: symlink escape attempt")
		}
	}
	return absPath, nil
}

// resolve maps a virtual path to a host path, checking permissions.
func (f *FS) resolve(virtualPath string, needWrite bool) (string, error) {
	if len(virtualPath) > f.maxPathLen {
		return "", errors.New("path too long")
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	vp := normalizePath(virtualPath)

	for _, m := range f.mounts {
		if vp != m.VirtualPath && !strings.HasPrefix(vp, m.VirtualPath+"/") {
			continue
		}

		if needWrite && m.Mode == MountReadOnly {
			return "", errors.New("permission denied: read-only mount")
		}

		relPath := strings.TrimPrefix(vp, m.VirtualPath)
		if relPath == "" {
			relPath = "/"
		}

		absHostPath, err := filepath.Abs(filepath.Join(m.HostPath, relPath))
		if err != nil {
			return "", errors.New("invalid path")
		}

		if !strings.HasPrefix(absHostPath, m.HostPath) {
			return "", errors.New("permission denied: path escape attempt")
		}

		return checkSymlinkEscape(absHostPath, m.HostPath)
	}

	return "", errors.New("permission denied: path not in any mount")
}

// Read returns the contents of a file.
func (f *FS) Read(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, errors.New("path required")
	}

	hostPath, err := f.resolve(path, false)
	if err != nil {
		return nil, err
	}

	// Check file size before reading
	info, err := os.Stat(hostPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("stat: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("cannot read directory: %s", path)
	}
	if info.Size() > f.maxFileSize {
		return nil, errors.New("file too large")
	}

	data, err := os.ReadFile(hostPath)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	return string(data), nil
}

// Write writes content to a file.
func (f *FS) Write(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, errors.New("path required")
	}
	content, ok := args["content"].(string)
	if !ok {
		return nil, errors.New("content required")
	}
	if int64(len(content)) > f.maxWriteSize {
		return nil, errors.New("content too large")
	}

	hostPath, err := f.resolve(path, true)
	if err != nil {
		return nil, err
	}

	// Check if file exists for MountReadWrite (can't create new files)
	if _, statErr := os.Stat(hostPath); os.IsNotExist(statErr) {
		// Check if mount allows creation
		mount := f.findMount(path)
		if mount == nil || mount.Mode != MountReadWriteCreate {
			return nil, errors.New("permission denied: cannot create new files")
		}
	}

	if err := os.WriteFile(hostPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return "ok", nil
}

// List returns the contents of a directory.
func (f *FS) List(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, errors.New("path required")
	}

	hostPath, err := f.resolve(path, false)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(hostPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory not found: %s", path)
		}
		return nil, fmt.Errorf("listdir: %w", err)
	}

	result := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		item := map[string]any{
			"name":   entry.Name(),
			"is_dir": entry.IsDir(),
		}
		if info != nil {
			item["size"] = info.Size()
		}
		result = append(result, item)
	}

	return result, nil
}

// Exists checks if a path exists.
func (f *FS) Exists(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, errors.New("path required")
	}

	hostPath, err := f.resolve(path, false)
	if err != nil {
		// Permission denied means it doesn't exist from sandbox perspective
		return false, nil
	}

	_, err = os.Stat(hostPath)
	return err == nil, nil
}

// Mkdir creates a directory.
func (f *FS) Mkdir(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, errors.New("path required")
	}

	hostPath, err := f.resolve(path, true)
	if err != nil {
		return nil, err
	}

	// Check if mount allows creation
	mount := f.findMount(path)
	if mount == nil || mount.Mode != MountReadWriteCreate {
		return nil, errors.New("permission denied: cannot create directories")
	}

	if err := os.MkdirAll(hostPath, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	return "ok", nil
}

// Remove deletes a file or empty directory.
func (f *FS) Remove(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, errors.New("path required")
	}

	hostPath, err := f.resolve(path, true)
	if err != nil {
		return nil, err
	}

	// Check if mount allows write (delete is a write operation)
	mount := f.findMount(path)
	if mount == nil || mount.Mode == MountReadOnly {
		return nil, errors.New("permission denied: read-only mount")
	}

	if err := os.Remove(hostPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		if pathErr, ok := err.(*fs.PathError); ok && strings.Contains(pathErr.Error(), "directory not empty") {
			return nil, fmt.Errorf("directory not empty: %s", path)
		}
		return nil, fmt.Errorf("remove: %w", err)
	}

	return "ok", nil
}

// Stat returns information about a file or directory.
func (f *FS) Stat(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, errors.New("path required")
	}

	hostPath, err := f.resolve(path, false)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(hostPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("stat: %w", err)
	}

	return map[string]any{
		"name":     info.Name(),
		"size":     info.Size(),
		"is_dir":   info.IsDir(),
		"mod_time": info.ModTime().Unix(),
	}, nil
}

// findMount finds the mount for a given virtual path.
func (f *FS) findMount(virtualPath string) *Mount {
	f.mu.RLock()
	defer f.mu.RUnlock()

	vp := normalizePath(virtualPath)
	for i := range f.mounts {
		m := &f.mounts[i]
		if vp == m.VirtualPath || strings.HasPrefix(vp, m.VirtualPath+"/") {
			return m
		}
	}
	return nil
}
