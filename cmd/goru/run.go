package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [file]",
	Short: "Run code (stateless execution)",
	Long: `Execute Python or JavaScript code in a sandboxed environment.

Code can be provided via:
  - File argument: goru run script.py
  - Inline flag: goru run -c 'print(1+1)'
  - Stdin: echo 'print(1+1)' | goru run`,
	Args: cobra.MaximumNArgs(1),
	Run:  runRun,
}

func init() {
	addRunFlags(runCmd)
	rootCmd.AddCommand(runCmd)
}

func addRunFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("code", "c", "", "Code to execute")
	cmd.Flags().Duration("timeout", 30*time.Second, "Execution timeout")
	cmd.Flags().Bool("kv", false, "Enable key-value store")
	cmd.Flags().StringSlice("allow-host", nil, "Allow HTTP to host (repeatable)")
	cmd.Flags().StringSlice("mount", nil, "Mount filesystem virtual:host:mode (repeatable)")
	cmd.Flags().String("memory", "256mb", "Memory limit: 1mb, 16mb, 64mb, 256mb, 1gb")

	// Security limits
	cmd.Flags().Int("http-max-url", 8192, "Max HTTP URL length")
	cmd.Flags().Int64("http-max-body", 1024*1024, "Max HTTP response body size")
	cmd.Flags().Int64("fs-max-file", 10*1024*1024, "Max file read size")
	cmd.Flags().Int64("fs-max-write", 10*1024*1024, "Max file write size")
	cmd.Flags().Int("fs-max-path", 4096, "Max path length")
}

func runRun(cmd *cobra.Command, args []string) {
	code, _ := cmd.Flags().GetString("code")
	lang, _ := cmd.Flags().GetString("lang")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	noCache, _ := cmd.Flags().GetBool("no-cache")
	enableKV, _ := cmd.Flags().GetBool("kv")
	allowedHosts, _ := cmd.Flags().GetStringSlice("allow-host")
	mounts, _ := cmd.Flags().GetStringSlice("mount")

	httpMaxURL, _ := cmd.Flags().GetInt("http-max-url")
	httpMaxBody, _ := cmd.Flags().GetInt64("http-max-body")
	fsMaxFile, _ := cmd.Flags().GetInt64("fs-max-file")
	fsMaxWrite, _ := cmd.Flags().GetInt64("fs-max-write")
	fsMaxPath, _ := cmd.Flags().GetInt("fs-max-path")
	memoryLimit, _ := cmd.Flags().GetString("memory")

	var source string
	var filename string

	switch {
	case code != "":
		source = code
	case len(args) > 0:
		filename = args[0]
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		source = string(data)
	default:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		source = string(data)
	}

	if source == "" {
		cmd.Help()
		os.Exit(1)
	}

	language, langErr := getLanguage(lang, filename)
	if langErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", langErr)
		os.Exit(1)
	}
	registry := hostfunc.NewRegistry()

	var execOpts []executor.ExecutorOption
	if !noCache {
		execOpts = append(execOpts, executor.WithDiskCache())
	}
	if pages := parseMemoryLimit(memoryLimit); pages > 0 {
		execOpts = append(execOpts, executor.WithMemoryLimit(pages))
	}

	exec, err := executor.New(registry, execOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer exec.Close()

	var runOpts []executor.Option
	runOpts = append(runOpts, executor.WithTimeout(timeout))

	runOpts = append(runOpts,
		executor.WithHTTPMaxURLLength(httpMaxURL),
		executor.WithHTTPMaxBodySize(httpMaxBody),
		executor.WithFSMaxFileSize(fsMaxFile),
		executor.WithFSMaxWriteSize(fsMaxWrite),
		executor.WithFSMaxPathLength(fsMaxPath),
	)

	if len(allowedHosts) > 0 {
		runOpts = append(runOpts, executor.WithAllowedHosts(allowedHosts))
	}

	if enableKV {
		runOpts = append(runOpts, executor.WithKV())
	}

	for _, spec := range mounts {
		m, err := parseMount(spec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		runOpts = append(runOpts, executor.WithMount(m.VirtualPath, m.HostPath, m.Mode))
	}

	result := exec.Run(context.Background(), language, source, runOpts...)
	fmt.Print(result.Output)

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
		os.Exit(1)
	}
}
