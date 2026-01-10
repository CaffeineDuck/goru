//go:build wasip1

// Mock language for testing executor logic without real Python/JS.
// Build with: GOOS=wasip1 GOARCH=wasm go build -o mock.wasm mock.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	// Signal ready for session mode
	fmt.Fprint(os.Stderr, "\x00GORU_READY\x00")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var cmd struct {
			Type string `json:"type"`
			Code string `json:"code"`
			Repl bool   `json:"repl"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &cmd); err != nil {
			continue
		}

		if cmd.Type == "exit" {
			break
		}

		if cmd.Type == "check" {
			fmt.Fprint(os.Stderr, "\x00GORU_COMPLETE\x00")
			continue
		}

		if cmd.Type == "exec" {
			// Echo the code as output (simple mock behavior)
			fmt.Print(cmd.Code)
			fmt.Fprint(os.Stderr, "\x00GORU_DONE\x00")
		}
	}
}
