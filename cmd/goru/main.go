package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/python"
)

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	var (
		code        = flag.String("c", "", "Python code to execute")
		timeout     = flag.Duration("timeout", 30*time.Second, "Execution timeout")
		noCache     = flag.Bool("no-cache", false, "Disable disk compilation cache")
		allowedHost stringSlice
	)
	flag.Var(&allowedHost, "allow-host", "Allowed HTTP host (can be repeated)")
	flag.Parse()

	var source string

	switch {
	case *code != "":
		source = *code
	case flag.NArg() > 0:
		data, err := os.ReadFile(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
			os.Exit(1)
		}
		source = string(data)
	default:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
			os.Exit(1)
		}
		source = string(data)
	}

	if source == "" {
		fmt.Fprintln(os.Stderr, "usage: goru -c 'code' | goru file.py | echo 'code' | goru")
		os.Exit(1)
	}

	// Set up host function registry
	registry := hostfunc.NewRegistry()

	// Create executor with disk cache for faster repeated CLI calls
	var execOpts []executor.ExecutorOption
	if !*noCache {
		execOpts = append(execOpts, executor.WithDiskCache())
	}

	exec, err := executor.New(registry, execOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating executor: %v\n", err)
		os.Exit(1)
	}
	defer exec.Close()

	// Run Python code
	var runOpts []executor.Option
	runOpts = append(runOpts, executor.WithTimeout(*timeout))
	if len(allowedHost) > 0 {
		runOpts = append(runOpts, executor.WithAllowedHosts(allowedHost))
	}

	result := exec.Run(context.Background(), python.New(), source, runOpts...)

	fmt.Print(result.Output)

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", result.Error)
		os.Exit(1)
	}
}
