package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: download <url> <output>")
		os.Exit(1)
	}

	url, output := os.Args[1], os.Args[2]

	if _, err := os.Stat(output); err == nil {
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "download failed: %s\n", resp.Status)
		os.Exit(1)
	}

	f, err := os.Create(output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
