package hostfunc

import (
	"context"
	"errors"
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

// Mount represents a virtual path mapped to a host path with specific permissions.
type Mount struct {
	VirtualPath string    // Path as seen by sandboxed code (e.g., "/data")
	HostPath    string    // Actual path on host filesystem
	Mode        MountMode // Permission level
}

// FS provides filesystem operations with explicit mount points.
type FS struct {
	mounts []Mount
	mu     sync.RWMutex
}

// NewFS creates a new filesystem handler with the given mount points.
func NewFS(mounts ...Mount) *FS {
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
		normalized = append(normalized, Mount{
			VirtualPath: vp,
			HostPath:    hp,
			Mode:        m.Mode,
		})
	}
	return &FS{mounts: normalized}
}

// resolve maps a virtual path to a host path, checking permissions.
// Returns the host path and whether write access is allowed.
func (f *FS) resolve(virtualPath string, needWrite bool) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Normalize the virtual path
	vp := filepath.Clean("/" + strings.TrimPrefix(virtualPath, "/"))

	// Find matching mount
	for _, m := range f.mounts {
		if vp == m.VirtualPath || strings.HasPrefix(vp, m.VirtualPath+"/") {
			// Check permissions
			if needWrite && m.Mode == MountReadOnly {
				return "", errors.New("permission denied: read-only mount")
			}

			// Calculate relative path within mount
			relPath := strings.TrimPrefix(vp, m.VirtualPath)
			if relPath == "" {
				relPath = "/"
			}

			// Join with host path
			hostPath := filepath.Join(m.HostPath, relPath)

			// Security: ensure we haven't escaped the mount via ..
			absHostPath, err := filepath.Abs(hostPath)
			if err != nil {
				return "", errors.New("invalid path")
			}

			// Check that resolved path is still under mount's host path
			if !strings.HasPrefix(absHostPath, m.HostPath) {
				return "", errors.New("permission denied: path escape attempt")
			}

			return absHostPath, nil
		}
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

	data, err := os.ReadFile(hostPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("file not found: " + path)
		}
		return nil, errors.New("read error: " + err.Error())
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
		return nil, errors.New("write error: " + err.Error())
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
			return nil, errors.New("directory not found: " + path)
		}
		return nil, errors.New("list error: " + err.Error())
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
		return nil, errors.New("mkdir error: " + err.Error())
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
			return nil, errors.New("file not found: " + path)
		}
		if pathErr, ok := err.(*fs.PathError); ok && strings.Contains(pathErr.Error(), "directory not empty") {
			return nil, errors.New("directory not empty: " + path)
		}
		return nil, errors.New("remove error: " + err.Error())
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
			return nil, errors.New("file not found: " + path)
		}
		return nil, errors.New("stat error: " + err.Error())
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

	vp := filepath.Clean("/" + strings.TrimPrefix(virtualPath, "/"))

	for i := range f.mounts {
		m := &f.mounts[i]
		if vp == m.VirtualPath || strings.HasPrefix(vp, m.VirtualPath+"/") {
			return m
		}
	}
	return nil
}
