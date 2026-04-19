package ui

import (
	"fmt"
	"strings"
)

// ChipLabel trims a trailing parenthetical hint from a prompt label so the
// "saved" chip form stays compact. "GitHub repository (owner/repo)" becomes
// "GitHub repository".
func ChipLabel(label string) string {
	i := strings.LastIndex(label, " (")
	if i < 0 || !strings.HasSuffix(label, ")") {
		return label
	}
	return label[:i]
}

// ChipLabelWidth returns the widest ChipLabel across the provided prompt
// labels — pre-compute once per interactive flow to column-align the chips.
func ChipLabelWidth(labels ...string) int {
	max := 0
	for _, label := range labels {
		if w := len(ChipLabel(label)); w > max {
			max = w
		}
	}
	return max
}

// RenderChip formats a label/value pair for a confirmed prompt. When
// labelWidth > 0 the label is padded to that column width and the colon is
// dropped, producing filled-form output. With labelWidth = 0 the fallback is
// "Label: value".
func RenderChip(label, value string, labelWidth int) string {
	short := ChipLabel(label)
	if labelWidth > 0 {
		if pad := labelWidth - len(short); pad > 0 {
			short += strings.Repeat(" ", pad)
		}
		return fmt.Sprintf("%s  %s", short, PrimaryStyle.Render(value))
	}
	return fmt.Sprintf("%s: %s", short, PrimaryStyle.Render(value))
}

// RenderSectionHeader renders a one-line heading for interactive wizards,
// e.g. "━━ Generate aws-iam-provisioner ━━".
func RenderSectionHeader(title string) string {
	return HeadingStyle.Render(fmt.Sprintf("━━ %s ━━", title))
}
