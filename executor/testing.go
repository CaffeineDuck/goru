package executor

import (
	"sync"

	"github.com/caffeineduck/goru/hostfunc"
)

// TestExecutor provides a shared executor for tests to avoid repeated cold starts.
// Use GetTestExecutor() to get a shared instance that's reused across tests.
var (
	testExecutor     *Executor
	testExecutorOnce sync.Once
	testExecutorErr  error
)

// GetTestExecutor returns a shared executor for testing.
// This avoids the 1.5s cold start on each test.
// The executor is created once and reused.
func GetTestExecutor() (*Executor, error) {
	testExecutorOnce.Do(func() {
		registry := hostfunc.NewRegistry()
		testExecutor, testExecutorErr = New(registry)
	})
	return testExecutor, testExecutorErr
}

// CloseTestExecutor closes the shared test executor.
// Call this in TestMain if needed, but typically not necessary.
func CloseTestExecutor() {
	if testExecutor != nil {
		testExecutor.Close()
		testExecutor = nil
		testExecutorOnce = sync.Once{} // Reset for next test run
	}
}
