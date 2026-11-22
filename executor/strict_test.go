package executor_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/javascript"
)

// A guest that catches the error of a rejected call must not keep a strict run
// going, and no host function may run for it.
func TestStrictProtocolEndsTheRunEvenWhenTheGuestCatches(t *testing.T) {
	var calls atomic.Int32
	registry := hostfunc.NewRegistry()
	registry.Register("known", func(context.Context, map[string]any) (any, error) {
		calls.Add(1)
		return "ok", nil
	})
	exec, err := executor.New(registry)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close()
	cases := map[string]string{
		"unknown":   `try { call("not_registered", {}) } catch (e) {}`,
		"malformed": `try { std.err.puts("\x00GORU:{not json\x00"); std.in.getline() } catch (e) {}`,
		"no-name":   `try { std.err.puts("\x00GORU:{\"args\":{}}\x00"); std.in.getline() } catch (e) {}`,
		"flush":     `try { std.err.puts("\x00GORU_FLUSH:x\x00") } catch (e) {}`,
		"async":     `try { std.err.puts("\x00GORU:{\"id\":\"1\",\"fn\":\"nope\",\"args\":{}}\x00") } catch (e) {}`,
	}
	for name, attack := range cases {
		t.Run(name, func(t *testing.T) {
			calls.Store(0)
			code := attack + "\nconsole.log('survived'); call('known', {});"
			result := exec.Run(context.Background(), javascript.New(), code, executor.WithStrictProtocol())
			if !errors.Is(result.Error, executor.ErrProtocolViolation) {
				t.Fatalf("run error = %v, output %q", result.Error, result.Output)
			}
			if calls.Load() != 0 {
				t.Fatalf("a host function ran after the violation")
			}
		})
	}
	// The same guest without the option keeps today's behaviour: an ordinary call error.
	result := exec.Run(context.Background(), javascript.New(), cases["unknown"]+"\nconsole.log(call('known', {}));")
	if result.Error != nil || !strings.Contains(result.Output, "ok") {
		t.Fatalf("lenient run: %v %q", result.Error, result.Output)
	}
	// A well-formed call to a registered function still works in strict mode.
	result = exec.Run(context.Background(), javascript.New(), "console.log(call('known', {}))", executor.WithStrictProtocol())
	if result.Error != nil || !strings.Contains(result.Output, "ok") {
		t.Fatalf("strict valid call: %v %q", result.Error, result.Output)
	}
}
