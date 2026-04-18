package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestWritersStripANSIWhenNoColorIsSet(t *testing.T) {
	t.Setenv("NO_COLOR", "true")

	tests := []struct {
		name     string
		write    func(ioWriter *bytes.Buffer)
		expected string
	}{
		{
			name: "success",
			write: func(ioWriter *bytes.Buffer) {
				Success(ioWriter, "done")
			},
			expected: IconSuccess,
		},
		{
			name: "warning",
			write: func(ioWriter *bytes.Buffer) {
				Warn(ioWriter, "careful")
			},
			expected: IconWarning,
		},
		{
			name: "error",
			write: func(ioWriter *bytes.Buffer) {
				Error(ioWriter, "broken")
			},
			expected: IconError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.write(&buf)

			if strings.Contains(buf.String(), "\x1b[") {
				t.Fatalf("expected no ANSI escape sequences, got %q", buf.String())
			}

			if !strings.Contains(buf.String(), tt.expected) {
				t.Fatalf("expected icon %q in output %q", tt.expected, buf.String())
			}
		})
	}
}
