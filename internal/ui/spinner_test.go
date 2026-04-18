package ui

import (
	"bytes"
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
