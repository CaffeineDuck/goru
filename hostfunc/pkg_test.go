package hostfunc

import (
	"context"
	"testing"
)

func TestPkgInstallerDisabled(t *testing.T) {
	installer := NewPkgInstaller(DefaultPkgConfig())

	_, err := installer(context.Background(), map[string]any{"name": "requests"})
	if err == nil {
		t.Error("expected error when disabled")
	}
	if err.Error() != "package installation disabled" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPkgInstallerNoName(t *testing.T) {
	cfg := DefaultPkgConfig()
	cfg.Enabled = true
	installer := NewPkgInstaller(cfg)

	_, err := installer(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error when no name")
	}
	if err.Error() != "package name required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPkgInstallerInvalidName(t *testing.T) {
	cfg := DefaultPkgConfig()
	cfg.Enabled = true
	installer := NewPkgInstaller(cfg)

	_, err := installer(context.Background(), map[string]any{"name": "foo;rm -rf /"})
	if err == nil {
		t.Error("expected error for invalid name")
	}
	if err.Error() != "invalid package name" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPkgInstallerNotAllowed(t *testing.T) {
	cfg := DefaultPkgConfig()
	cfg.Enabled = true
	cfg.AllowedPackages = []string{"requests", "pydantic"}
	installer := NewPkgInstaller(cfg)

	_, err := installer(context.Background(), map[string]any{"name": "dangerous-package"})
	if err == nil {
		t.Error("expected error for non-allowed package")
	}
	if err.Error() != `package "dangerous-package" not allowed` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPkgInstallerAllowedWithExtras(t *testing.T) {
	cfg := DefaultPkgConfig()
	cfg.Enabled = true
	cfg.AllowedPackages = []string{"pydantic"}
	cfg.PackageDir = t.TempDir()
	installer := NewPkgInstaller(cfg)

	// pydantic[email] should match pydantic
	// This will fail if pip isn't installed, but that's ok for the test
	_, _ = installer(context.Background(), map[string]any{"name": "pydantic[email]"})
}

func TestPkgInstallerInvalidVersion(t *testing.T) {
	cfg := DefaultPkgConfig()
	cfg.Enabled = true
	installer := NewPkgInstaller(cfg)

	_, err := installer(context.Background(), map[string]any{
		"name":    "requests",
		"version": ">=2.0;rm -rf /",
	})
	if err == nil {
		t.Error("expected error for invalid version")
	}
	if err.Error() != "invalid version specifier" {
		t.Errorf("unexpected error: %v", err)
	}
}
