package hostfunc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFSReadOnly(t *testing.T) {
	// Create temp directory with test file
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	fs := NewFS(Mount{
		VirtualPath: "/data",
		HostPath:    dir,
		Mode:        MountReadOnly,
	})

	ctx := context.Background()

	// Should be able to read
	content, err := fs.Read(ctx, map[string]any{"path": "/data/test.txt"})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if content != "hello world" {
		t.Errorf("expected 'hello world', got %q", content)
	}

	// Should NOT be able to write
	_, err = fs.Write(ctx, map[string]any{"path": "/data/test.txt", "content": "modified"})
	if err == nil {
		t.Error("expected write to fail on read-only mount")
	}
}

func TestFSReadWrite(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("original"), 0644)

	fs := NewFS(Mount{
		VirtualPath: "/output",
		HostPath:    dir,
		Mode:        MountReadWrite,
	})

	ctx := context.Background()

	// Should be able to write to existing file
	_, err := fs.Write(ctx, map[string]any{"path": "/output/test.txt", "content": "modified"})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Verify content changed
	content, _ := os.ReadFile(testFile)
	if string(content) != "modified" {
		t.Errorf("expected 'modified', got %q", content)
	}

	// Should NOT be able to create new file
	_, err = fs.Write(ctx, map[string]any{"path": "/output/new.txt", "content": "new"})
	if err == nil {
		t.Error("expected creating new file to fail on MountReadWrite")
	}
}

func TestFSReadWriteCreate(t *testing.T) {
	dir := t.TempDir()

	fs := NewFS(Mount{
		VirtualPath: "/workspace",
		HostPath:    dir,
		Mode:        MountReadWriteCreate,
	})

	ctx := context.Background()

	// Should be able to create new file
	_, err := fs.Write(ctx, map[string]any{"path": "/workspace/new.txt", "content": "created"})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Verify file exists
	content, _ := os.ReadFile(filepath.Join(dir, "new.txt"))
	if string(content) != "created" {
		t.Errorf("expected 'created', got %q", content)
	}

	// Should be able to create directory
	_, err = fs.Mkdir(ctx, map[string]any{"path": "/workspace/subdir"})
	if err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Verify directory exists
	info, _ := os.Stat(filepath.Join(dir, "subdir"))
	if !info.IsDir() {
		t.Error("expected directory to be created")
	}
}

func TestFSList(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("1"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("22"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	fs := NewFS(Mount{
		VirtualPath: "/data",
		HostPath:    dir,
		Mode:        MountReadOnly,
	})

	ctx := context.Background()

	result, err := fs.List(ctx, map[string]any{"path": "/data"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	entries := result.([]map[string]any)
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// Check that we have expected files
	names := make(map[string]bool)
	for _, e := range entries {
		names[e["name"].(string)] = true
	}
	if !names["file1.txt"] || !names["file2.txt"] || !names["subdir"] {
		t.Errorf("unexpected entries: %v", names)
	}
}

func TestFSPathTraversalBlocked(t *testing.T) {
	dir := t.TempDir()
	// Create a file outside the mount
	parentFile := filepath.Join(filepath.Dir(dir), "secret.txt")
	os.WriteFile(parentFile, []byte("secret"), 0644)
	defer os.Remove(parentFile)

	fs := NewFS(Mount{
		VirtualPath: "/data",
		HostPath:    dir,
		Mode:        MountReadOnly,
	})

	ctx := context.Background()

	// Try to escape via ..
	_, err := fs.Read(ctx, map[string]any{"path": "/data/../secret.txt"})
	if err == nil {
		t.Error("expected path traversal to be blocked")
	}
}

func TestFSPathNotInMount(t *testing.T) {
	dir := t.TempDir()

	fs := NewFS(Mount{
		VirtualPath: "/data",
		HostPath:    dir,
		Mode:        MountReadOnly,
	})

	ctx := context.Background()

	// Try to access path not in any mount
	_, err := fs.Read(ctx, map[string]any{"path": "/etc/passwd"})
	if err == nil {
		t.Error("expected access outside mount to fail")
	}
}

func TestFSExists(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "exists.txt"), []byte(""), 0644)

	fs := NewFS(Mount{
		VirtualPath: "/data",
		HostPath:    dir,
		Mode:        MountReadOnly,
	})

	ctx := context.Background()

	// File exists
	exists, _ := fs.Exists(ctx, map[string]any{"path": "/data/exists.txt"})
	if exists != true {
		t.Error("expected file to exist")
	}

	// File doesn't exist
	exists, _ = fs.Exists(ctx, map[string]any{"path": "/data/nope.txt"})
	if exists != false {
		t.Error("expected file to not exist")
	}

	// Path outside mount
	exists, _ = fs.Exists(ctx, map[string]any{"path": "/etc/passwd"})
	if exists != false {
		t.Error("expected path outside mount to return false")
	}
}

func TestFSRemove(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "delete-me.txt")
	os.WriteFile(testFile, []byte("bye"), 0644)

	fs := NewFS(Mount{
		VirtualPath: "/output",
		HostPath:    dir,
		Mode:        MountReadWrite,
	})

	ctx := context.Background()

	// Remove file
	_, err := fs.Remove(ctx, map[string]any{"path": "/output/delete-me.txt"})
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestFSStat(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644)

	fs := NewFS(Mount{
		VirtualPath: "/data",
		HostPath:    dir,
		Mode:        MountReadOnly,
	})

	ctx := context.Background()

	result, err := fs.Stat(ctx, map[string]any{"path": "/data/file.txt"})
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}

	stat := result.(map[string]any)
	if stat["name"] != "file.txt" {
		t.Errorf("expected name 'file.txt', got %v", stat["name"])
	}
	if stat["size"].(int64) != 5 {
		t.Errorf("expected size 5, got %v", stat["size"])
	}
	if stat["is_dir"].(bool) != false {
		t.Error("expected is_dir to be false")
	}
}
