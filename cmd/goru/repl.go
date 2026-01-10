package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
)

var replCmd = &cobra.Command{
	Use:   "repl",
	Short: "Interactive REPL with persistent state",
	Long: `Start an interactive REPL (Read-Eval-Print Loop) session.

Features:
  - Command history (up/down arrows)
  - Line editing (left/right, backspace, delete)
  - History search (Ctrl+R)
  - Multi-line input (end line with \)

Type 'exit' or 'quit' to end the session, or press Ctrl+D.`,
	Run: runRepl,
}

func init() {
	replCmd.Flags().StringP("lang", "l", "", "Language: python, js (required)")
	replCmd.Flags().String("packages", "", "Path to packages directory (Python only)")
	replCmd.Flags().Bool("kv", false, "Enable key-value store")
	replCmd.Flags().String("history", "", "History file path (default: ~/.goru_history)")
	rootCmd.AddCommand(replCmd)
}

func runRepl(cmd *cobra.Command, args []string) {
	lang, _ := cmd.Flags().GetString("lang")
	packages, _ := cmd.Flags().GetString("packages")
	noCache, _ := cmd.Root().PersistentFlags().GetBool("no-cache")
	enableKV, _ := cmd.Flags().GetBool("kv")
	historyFile, _ := cmd.Flags().GetString("history")

	if historyFile == "" {
		home, _ := os.UserHomeDir()
		historyFile = filepath.Join(home, ".goru_history")
	}

	language, langErr := getLanguage(lang, "")
	if langErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", langErr)
		os.Exit(1)
	}
	registry := hostfunc.NewRegistry()

	var execOpts []executor.ExecutorOption
	if !noCache {
		execOpts = append(execOpts, executor.WithDiskCache())
	}
	execOpts = append(execOpts, executor.WithPrecompile(language))

	exec, err := executor.New(registry, execOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer exec.Close()

	var sessionOpts []executor.SessionOption
	if packages != "" {
		sessionOpts = append(sessionOpts, executor.WithPackages(packages))
	}
	if enableKV {
		sessionOpts = append(sessionOpts, executor.WithSessionKV())
	}

	session, err := exec.NewSession(language, sessionOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting session: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	langName := language.Name()

	rl, err := readline.NewEx(&readline.Config{
		Prompt:            ">>> ",
		HistoryFile:       historyFile,
		HistoryLimit:      1000,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing readline: %v\n", err)
		os.Exit(1)
	}
	defer rl.Close()

	fmt.Fprintf(os.Stderr, "goru %s REPL (type 'exit' to quit, Ctrl+D to exit)\n", langName)

	var codeBuffer strings.Builder

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if codeBuffer.Len() > 0 {
					codeBuffer.Reset()
					rl.SetPrompt(">>> ")
					continue
				}
				continue
			}
			if err == io.EOF {
				fmt.Println()
				break
			}
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			break
		}

		// Handle explicit line continuation
		if strings.HasSuffix(line, "\\") {
			codeBuffer.WriteString(strings.TrimSuffix(line, "\\"))
			codeBuffer.WriteString("\n")
			rl.SetPrompt("... ")
			continue
		}

		// Build up the code
		if codeBuffer.Len() > 0 {
			codeBuffer.WriteString(line)
			codeBuffer.WriteString("\n")
		} else {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if trimmed == "exit" || trimmed == "quit" {
				break
			}
			codeBuffer.WriteString(line)
			codeBuffer.WriteString("\n")
		}

		code := codeBuffer.String()

		// Check if code is complete (handles def, for, if blocks)
		complete, err := session.CheckComplete(context.Background(), code)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			codeBuffer.Reset()
			rl.SetPrompt(">>> ")
			continue
		}

		if !complete {
			rl.SetPrompt("... ")
			continue
		}

		// Code is complete, execute it
		codeBuffer.Reset()
		rl.SetPrompt(">>> ")

		result := session.RunRepl(context.Background(), strings.TrimRight(code, "\n"))
		if result.Output != "" {
			fmt.Print(result.Output)
			if !strings.HasSuffix(result.Output, "\n") {
				fmt.Println()
			}
		}
		if result.Error != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
		}
	}
}
