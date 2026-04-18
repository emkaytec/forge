package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestBannerColorCapableRenderContainsANSIAndSilhouette(t *testing.T) {
	var buf bytes.Buffer
	Banner(&buf, termenv.TrueColor)

	output := buf.String()
	if !strings.Contains(output, "\x1b[") {
		t.Fatalf("expected ANSI escape sequences in color-capable render, got %q", output)
	}

	if !strings.Contains(output, bannerWordmark[0]) {
		t.Fatalf("expected banner silhouette in output, got %q", output)
	}
}

func TestBannerColorDisabledRenderContainsSilhouetteWithoutANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	var buf bytes.Buffer
	Banner(&buf, Profile())

	output := buf.String()
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("expected no ANSI escape sequences, got %q", output)
	}

	if !strings.Contains(output, bannerWordmark[0]) {
		t.Fatalf("expected banner silhouette in output, got %q", output)
	}
}

func TestBannerWidthBudget(t *testing.T) {
	var buf bytes.Buffer
	Banner(&buf, termenv.Ascii)

	for _, line := range strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n") {
		if lipgloss.Width(line) > 60 {
			t.Fatalf("expected banner width <= 60 columns, got %d for %q", lipgloss.Width(line), line)
		}
	}
}
