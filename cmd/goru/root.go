package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/javascript"
	"github.com/caffeineduck/goru/language/python"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "goru [file]",
	Short: "WASM code sandbox for Python and JavaScript",
	Long: `goru - Run untrusted Python and JavaScript safely using WebAssembly.

Run code from files, inline strings, or stdin. By default, sandboxed code
has no access to filesystem, network, or other system resources. Enable
capabilities explicitly with flags.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runRun, // Default to run command behavior
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Add persistent flags that apply to multiple commands
	rootCmd.PersistentFlags().StringP("lang", "l", "", "Language: python, js (default: auto-detect)")
	rootCmd.PersistentFlags().Bool("no-cache", false, "Disable compilation cache")

	// Add run-specific flags to root (for default command)
	addRunFlags(rootCmd)
}

type stringSliceValue []string

func (s *stringSliceValue) String() string { return strings.Join(*s, ",") }
func (s *stringSliceValue) Set(v string) error {
	*s = append(*s, v)
	return nil
}
func (s *stringSliceValue) Type() string { return "string" }

func parseMount(spec string) (hostfunc.Mount, error) {
	parts := strings.Split(spec, ":")
	if len(parts) != 3 {
		return hostfunc.Mount{}, fmt.Errorf("invalid mount spec %q (expected virtual:host:mode)", spec)
	}

	var mode hostfunc.MountMode
	switch parts[2] {
	case "ro":
		mode = hostfunc.MountReadOnly
	case "rw":
		mode = hostfunc.MountReadWrite
	case "rwc":
		mode = hostfunc.MountReadWriteCreate
	default:
		return hostfunc.Mount{}, fmt.Errorf("invalid mount mode %q (expected ro, rw, or rwc)", parts[2])
	}

	return hostfunc.Mount{
		VirtualPath: parts[0],
		HostPath:    parts[1],
		Mode:        mode,
	}, nil
}

func getLanguage(langFlag string, filename string) (executor.Language, error) {
	lang := langFlag

	if lang == "" && filename != "" {
		switch strings.ToLower(filepath.Ext(filename)) {
		case ".py":
			lang = "python"
		case ".js", ".mjs":
			lang = "js"
		}
	}

	if lang == "" {
		return nil, fmt.Errorf("language required: use --lang python or --lang js")
	}

	switch lang {
	case "js", "javascript":
		return javascript.New(), nil
	case "python", "py":
		return python.New(), nil
	default:
		return nil, fmt.Errorf("unknown language %q: use python or js", lang)
	}
}

func parseMemoryLimit(s string) uint32 {
	switch strings.ToLower(s) {
	case "1mb":
		return executor.MemoryLimit1MB
	case "16mb":
		return executor.MemoryLimit16MB
	case "64mb":
		return executor.MemoryLimit64MB
	case "256mb":
		return executor.MemoryLimit256MB
	case "1gb":
		return executor.MemoryLimit1GB
	default:
		return 0 // use default
	}
}
