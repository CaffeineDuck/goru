package hostfunc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PkgConfig configures the package installer.
type PkgConfig struct {
	PackageDir      string   // Directory to install packages into
	AllowedPackages []string // If set, only these packages can be installed
	Enabled         bool     // Whether package installation is enabled
}

// DefaultPkgConfig returns the default package installer configuration.
func DefaultPkgConfig() PkgConfig {
	return PkgConfig{
		PackageDir: ".goru/python/packages",
		Enabled:    false,
	}
}

// NewPkgInstaller returns a host function for installing Python packages via pip.
// Args: name (required), version (optional).
func NewPkgInstaller(cfg PkgConfig) Func {
	return func(ctx context.Context, args map[string]any) (any, error) {
		if !cfg.Enabled {
			return nil, fmt.Errorf("package installation disabled")
		}

		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("package name required")
		}

		if strings.ContainsAny(name, ";|&$`") {
			return nil, fmt.Errorf("invalid package name")
		}

		if len(cfg.AllowedPackages) > 0 {
			allowed := false
			for _, pkg := range cfg.AllowedPackages {
				if pkg == name || strings.HasPrefix(name, pkg+"[") {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, fmt.Errorf("package %q not allowed", name)
			}
		}

		pkgSpec := name
		if version, ok := args["version"].(string); ok && version != "" {
			if strings.ContainsAny(version, ";|&$`") {
				return nil, fmt.Errorf("invalid version specifier")
			}
			pkgSpec = name + version
		}

		if err := os.MkdirAll(cfg.PackageDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create package dir: %w", err)
		}

		absDir, err := filepath.Abs(cfg.PackageDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve package dir: %w", err)
		}

		cmd := exec.CommandContext(ctx, "pip", "install", "--target", absDir, pkgSpec)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return map[string]any{
				"success": false,
				"error":   string(output),
			}, nil
		}

		return map[string]any{
			"success": true,
			"output":  string(output),
		}, nil
	}
}
