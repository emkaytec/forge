package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSpinnerRunsCleanlyWithoutANSIOnNonTTYWriter(t *testing.T) {
	var buf bytes.Buffer

	spinner := NewSpinner("Testing spinner")
	if err := spinner.Run(&buf, 5*time.Millisecond); err != nil {
		t.Fatalf("spinner returned error: %v", err)
	}

	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("expected no ANSI escape sequences, got %q", buf.String())
	}
}

func TestSpinnerRunWhileReturnsFunctionResult(t *testing.T) {
	wantErr := errors.New("boom")

	cases := []struct {
		name string
		fn   func() error
		want error
	}{
		{name: "nil error", fn: func() error { return nil }, want: nil},
		{name: "propagates error", fn: func() error { return wantErr }, want: wantErr},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer

			spinner := NewSpinner("Working...")
			got := spinner.RunWhile(&buf, tc.fn)
			if !errors.Is(got, tc.want) {
				t.Fatalf("RunWhile returned %v, want %v", got, tc.want)
			}

			if strings.Contains(buf.String(), "\x1b[") {
				t.Fatalf("expected no ANSI escape sequences on non-TTY writer, got %q", buf.String())
			}
		})
	}
}

func TestSpinnerRunWhileInvokesFunctionOnNonTTYWriter(t *testing.T) {
	var buf bytes.Buffer
	called := false

	spinner := NewSpinner("Working...")
	if err := spinner.RunWhile(&buf, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("RunWhile returned error: %v", err)
	}

	if !called {
		t.Fatal("expected RunWhile to invoke fn on non-TTY writer")
	}
}

func TestSpinnerRunWhileNilFunctionIsNoOp(t *testing.T) {
	var buf bytes.Buffer

	spinner := NewSpinner("Working...")
	if err := spinner.RunWhile(&buf, nil); err != nil {
		t.Fatalf("RunWhile returned error: %v", err)
	}
}
