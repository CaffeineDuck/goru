package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/caffeineduck/goru/sandbox"
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
		timeout     = flag.Duration("timeout", sandbox.DefaultOptions().Timeout, "Execution timeout")
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

	opts := sandbox.Options{
		Timeout:      *timeout,
		AllowedHosts: allowedHost,
	}

	result := sandbox.RunPython(source, opts)

	fmt.Print(result.Output)

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", result.Error)
		os.Exit(1)
	}
}
