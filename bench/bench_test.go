package bench

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/python"
)

func BenchmarkGoru_ColdStart(b *testing.B) {
	registry := hostfunc.NewRegistry()
	for b.Loop() {
		exec, _ := executor.New(registry)
		exec.Run(context.Background(), python.New(), "x=1")
		exec.Close()
	}
}

func BenchmarkGoru_WarmStart(b *testing.B) {
	registry := hostfunc.NewRegistry()
	exec, _ := executor.New(registry)
	defer exec.Close()
	lang := python.New()

	exec.Run(context.Background(), lang, "x=1") // warmup

	b.ResetTimer()
	for b.Loop() {
		exec.Run(context.Background(), lang, "x=1")
	}
}

func BenchmarkGoru_DiskCache(b *testing.B) {
	cacheDir, _ := os.MkdirTemp("", "goru-bench-cache")
	defer os.RemoveAll(cacheDir)

	registry := hostfunc.NewRegistry()
	lang := python.New()

	// First run to warm cache
	exec, _ := executor.New(registry, executor.WithDiskCache(cacheDir))
	exec.Run(context.Background(), lang, "x=1")
	exec.Close()

	b.ResetTimer()
	for b.Loop() {
		exec, _ := executor.New(registry, executor.WithDiskCache(cacheDir))
		exec.Run(context.Background(), lang, "x=1")
		exec.Close()
	}
}

func BenchmarkNative_Python(b *testing.B) {
	for b.Loop() {
		exec.Command("python3", "-c", "x=1").Run()
	}
}

func BenchmarkDocker_Python(b *testing.B) {
	if _, err := exec.LookPath("docker"); err != nil {
		b.Skip("docker not available")
	}
	exec.Command("docker", "pull", "-q", "python:3.12-slim").Run()

	for b.Loop() {
		exec.Command("docker", "run", "--rm", "python:3.12-slim", "python", "-c", "x=1").Run()
	}
}
