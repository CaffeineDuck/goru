package executor_test

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/hostfunc"
	"github.com/caffeineduck/goru/language/javascript"
	"github.com/tetratelabs/wazero/experimental"
)

func TestExplicitEngines(t *testing.T) {
	for _, engine := range []executor.Engine{executor.EngineCompiler, executor.EngineInterpreter} {
		t.Run(string(engine), func(t *testing.T) {
			e, err := executor.New(nil, executor.WithEngine(engine))
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			r := e.Run(context.Background(), javascript.New(), "console.log(42)")
			if r.Error != nil || strings.TrimSpace(r.Output) != "42" {
				t.Fatalf("execution: %+v", r)
			}
		})
	}
	if _, err := executor.New(nil, executor.WithEngine("invalid")); err == nil {
		t.Fatal("accepted an unknown engine")
	}
}

func TestClockOptIn(t *testing.T) {
	ctx := context.Background()
	e, err := executor.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	lang := javascript.New()
	code := `try { console.log(time_now() > 1e9) } catch (_) { console.log("disabled") }`
	check := func(r executor.Result, want string) {
		t.Helper()
		if r.Error != nil || strings.TrimSpace(r.Output) != want {
			t.Fatalf("want %q, got %+v", want, r)
		}
	}
	check(e.Run(ctx, lang, code), "disabled")
	check(e.Run(ctx, lang, code, executor.WithClock()), "true")
	check(e.Run(ctx, lang, code), "disabled")

	for _, tc := range []struct {
		options []executor.SessionOption
		want    string
	}{
		{nil, "disabled"},
		{[]executor.SessionOption{executor.WithSessionClock()}, "true"},
	} {
		s, err := e.NewSession(lang, tc.options...)
		if err != nil {
			t.Fatal(err)
		}
		check(s.Run(ctx, code), tc.want)
		s.Close()
	}
}

func TestRunClosesSuccessfulInstances(t *testing.T) {
	e, err := executor.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	var closes atomic.Int32
	ctx := experimental.WithCloseNotifier(context.Background(), experimental.CloseNotifyFunc(func(context.Context, uint32) {
		closes.Add(1)
	}))
	for i := int32(1); i <= 3; i++ {
		r := e.Run(ctx, javascript.New(), "console.log(42)", executor.WithTimeout(0))
		if r.Error != nil {
			t.Fatal(r.Error)
		}
		if closes.Load() != i {
			t.Fatalf("completed guest instances closed: %d, want %d", closes.Load(), i)
		}
	}
}

func TestLargeAsyncBatch(t *testing.T) {
	registry := hostfunc.NewRegistry()
	registry.Register("echo", func(_ context.Context, args map[string]any) (any, error) {
		return args["value"], nil
	})
	e, err := executor.New(registry)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	result := e.Run(context.Background(), javascript.New(), `
Promise.all(Array.from({length: 40}, (_, value) => asyncCall("echo", {value})))
    .then(values => console.log(values.length, values.every((value, index) => value === index)));
`)
	if result.Error != nil || strings.TrimSpace(result.Output) != "40 true" {
		t.Fatalf("execution: %+v", result)
	}
}
