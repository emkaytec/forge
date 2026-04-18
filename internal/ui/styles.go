package ui

import (
	"io"

	"github.com/charmbracelet/lipgloss"
)

var (
	HeadingStyle = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor)
	MutedStyle   = lipgloss.NewStyle().Foreground(MutedColor)
	SuccessStyle = lipgloss.NewStyle().Foreground(SuccessColor)
	WarningStyle = lipgloss.NewStyle().Foreground(WarningColor)
	ErrorStyle   = lipgloss.NewStyle().Foreground(ErrorColor)
)

type styles struct {
	Heading lipgloss.Style
	Muted   lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
}

func stylesFor(w io.Writer) styles {
	renderer := rendererFor(w)

	return styles{
		Heading: renderer.NewStyle().Bold(true).Foreground(PrimaryColor),
		Muted:   renderer.NewStyle().Foreground(MutedColor),
		Success: renderer.NewStyle().Foreground(SuccessColor),
		Warning: renderer.NewStyle().Foreground(WarningColor),
		Error:   renderer.NewStyle().Foreground(ErrorColor),
	}
}
