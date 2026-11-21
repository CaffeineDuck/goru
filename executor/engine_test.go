package executor_test

import (
	"context"
	"strings"
	"testing"

	"github.com/caffeineduck/goru/executor"
	"github.com/caffeineduck/goru/language/javascript"
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
