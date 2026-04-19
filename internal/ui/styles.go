package ui

import (
	"io"

	"github.com/charmbracelet/lipgloss"
)

var (
	HeadingStyle = lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor)
	PrimaryStyle = lipgloss.NewStyle().Foreground(PrimaryColor)
	BoldStyle    = lipgloss.NewStyle().Bold(true)
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

func RenderHeading(w io.Writer, text string) string {
	return stylesFor(w).Heading.Render(text)
}

func RenderMuted(w io.Writer, text string) string {
	return stylesFor(w).Muted.Render(text)
}

func RenderCommand(w io.Writer, text string) string {
	return rendererFor(w).NewStyle().Bold(true).Render(text)
}
