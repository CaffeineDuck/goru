package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Manage Python packages for sandboxed code",
	Long: `Install and manage Python packages that can be used in sandboxed code.

Packages are downloaded directly from PyPI (no pip required).
Only pure Python wheels are supported - packages with C extensions won't work.

Note: JavaScript packages are not supported. Use bundling (esbuild/webpack) for JS.`,
}

var depsInstallCmd = &cobra.Command{
	Use:   "install [packages...]",
	Short: "Install packages from PyPI",
	Args:  cobra.MinimumNArgs(1),
	Run:   runDepsInstall,
}

var depsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages",
	Run:   runDepsList,
}

var depsRemoveCmd = &cobra.Command{
	Use:   "remove [packages...]",
	Short: "Remove packages",
	Args:  cobra.MinimumNArgs(1),
	Run:   runDepsRemove,
}

var depsCacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Cache management commands",
}

var depsCacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear download cache",
	Run:   runDepsCacheClear,
}

var depsPkgDir string

func init() {
	depsCmd.PersistentFlags().StringVar(&depsPkgDir, "dir", ".goru/python/packages", "Package directory")

	depsCacheCmd.AddCommand(depsCacheClearCmd)
	depsCmd.AddCommand(depsInstallCmd, depsListCmd, depsRemoveCmd, depsCacheCmd)
	rootCmd.AddCommand(depsCmd)
}

type pypiURL struct {
	PackageType string `json:"packagetype"`
	Filename    string `json:"filename"`
	URL         string `json:"url"`
}

type pypiResponse struct {
	Info struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"info"`
	Urls []pypiURL `json:"urls"`
}

// Packages that won't work in WASM (require C extensions, sockets, etc.)
var blockedPackages = map[string]string{
	// C extensions
	"numpy":        "requires C extensions",
	"pandas":       "requires C extensions (numpy)",
	"scipy":        "requires C extensions",
	"tensorflow":   "requires C extensions",
	"torch":        "requires C extensions",
	"pytorch":      "requires C extensions",
	"scikit-learn": "requires C extensions",
	"sklearn":      "requires C extensions",
	"matplotlib":   "requires C extensions",
	"pillow":       "requires C extensions",
	"pil":          "requires C extensions",
	"opencv-python": "requires C extensions",
	"cv2":          "requires C extensions",
	"psycopg2":     "requires C extensions",
	"mysqlclient":  "requires C extensions",
	"cryptography": "requires C extensions",
	"bcrypt":       "requires C extensions",
	"lxml":         "requires C extensions",
	"grpcio":       "requires C extensions",
	// Socket-based (use goru's http module instead)
	"requests":    "uses sockets (use goru's http module instead)",
	"httpx":       "uses sockets (use goru's http module instead)",
	"urllib3":     "uses sockets (use goru's http module instead)",
	"aiohttp":     "uses async sockets (use goru's http module instead)",
	"flask":       "requires sockets (web framework not supported)",
	"django":      "requires sockets (web framework not supported)",
	"fastapi":     "requires sockets (web framework not supported)",
	"uvicorn":     "requires sockets (ASGI server not supported)",
	"gunicorn":    "requires sockets (WSGI server not supported)",
}

func runDepsInstall(cmd *cobra.Command, args []string) {
	if err := os.MkdirAll(depsPkgDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create package dir: %v\n", err)
		os.Exit(1)
	}

	for _, pkg := range args {
		name, version := parsePackageSpec(pkg)

		// Check blocklist
		if reason, blocked := blockedPackages[strings.ToLower(name)]; blocked {
			fmt.Fprintf(os.Stderr, "Error: %s is not supported in WASM (%s)\n", name, reason)
			fmt.Fprintf(os.Stderr, "See docs/python.md for compatible packages\n")
			os.Exit(1)
		}

		if err := installPackage(name, version); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing %s: %v\n", name, err)
			os.Exit(1)
		}
	}
	fmt.Println("Done.")
}

func parsePackageSpec(spec string) (name, version string) {
	// Handle specs like "requests>=2.32" or "pydantic==2.0"
	for _, op := range []string{">=", "<=", "==", "~=", "!="} {
		if idx := strings.Index(spec, op); idx != -1 {
			return spec[:idx], ""
		}
	}
	return spec, ""
}

func installPackage(name, version string) error {
	fmt.Printf("Installing %s...\n", name)

	// Fetch package info from PyPI
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", name)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch package info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("package not found on PyPI")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("PyPI returned status %d", resp.StatusCode)
	}

	var pypi pypiResponse
	if err := json.NewDecoder(resp.Body).Decode(&pypi); err != nil {
		return fmt.Errorf("failed to parse PyPI response: %w", err)
	}

	// Find a suitable wheel
	wheelURL := findWheel(pypi.Urls)
	if wheelURL == "" {
		return fmt.Errorf("no compatible wheel found (pure Python wheel required)")
	}

	// Download the wheel
	fmt.Printf("  Downloading %s-%s...\n", pypi.Info.Name, pypi.Info.Version)
	wheelResp, err := http.Get(wheelURL)
	if err != nil {
		return fmt.Errorf("failed to download wheel: %w", err)
	}
	defer wheelResp.Body.Close()

	// Save to temp file
	tmpFile, err := os.CreateTemp("", "goru-*.whl")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, wheelResp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download wheel: %w", err)
	}
	tmpFile.Close()

	// Extract the wheel
	fmt.Printf("  Extracting...\n")
	if err := extractWheel(tmpPath, depsPkgDir); err != nil {
		return fmt.Errorf("failed to extract wheel: %w", err)
	}

	return nil
}

func findWheel(urls []pypiURL) string {
	// Only accept pure Python wheels - no C extensions work in WASM
	for _, u := range urls {
		if u.PackageType != "bdist_wheel" {
			continue
		}

		filename := strings.ToLower(u.Filename)

		// Pure Python 3 wheel
		if strings.Contains(filename, "-py3-none-any") {
			return u.URL
		}

		// Universal wheel (Python 2 & 3)
		if strings.Contains(filename, "-py2.py3-none-any") {
			return u.URL
		}
	}

	return ""
}

func extractWheel(wheelPath, destDir string) error {
	r, err := zip.OpenReader(wheelPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// First pass: check for C extensions
	for _, f := range r.File {
		name := strings.ToLower(f.Name)
		if strings.HasSuffix(name, ".so") || strings.HasSuffix(name, ".pyd") || strings.HasSuffix(name, ".dylib") {
			return fmt.Errorf("package contains C extensions (%s) which won't work in WASM", filepath.Base(f.Name))
		}
	}

	// Second pass: extract files
	for _, f := range r.File {
		// Skip .dist-info directories (metadata)
		if strings.Contains(f.Name, ".dist-info/") {
			continue
		}

		destPath := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(destPath)
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

func runDepsList(cmd *cobra.Command, args []string) {
	entries, err := os.ReadDir(depsPkgDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No packages installed.")
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Println("No packages installed.")
		return
	}

	fmt.Printf("Packages in %s:\n", depsPkgDir)
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasSuffix(entry.Name(), ".dist-info") && !strings.HasPrefix(entry.Name(), "__") {
			fmt.Printf("  %s\n", entry.Name())
		}
	}
}

func runDepsRemove(cmd *cobra.Command, args []string) {
	for _, pkg := range args {
		pkgPath := filepath.Join(depsPkgDir, pkg)
		if err := os.RemoveAll(pkgPath); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", pkg, err)
			continue
		}

		entries, _ := os.ReadDir(depsPkgDir)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), pkg) && strings.HasSuffix(entry.Name(), ".dist-info") {
				distInfoPath := filepath.Join(depsPkgDir, entry.Name())
				os.RemoveAll(distInfoPath)
			}
		}

		fmt.Printf("Removed %s\n", pkg)
	}
}

func runDepsCacheClear(cmd *cobra.Command, args []string) {
	cacheDir := filepath.Join(".goru", "cache")
	if err := os.RemoveAll(cacheDir); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: failed to clear cache: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Cache cleared.")
}
