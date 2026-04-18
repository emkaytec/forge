package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var bannerWordmark = []string{
	"███████╗ ██████╗ ██████╗  ██████╗ ███████╗",
	"██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝",
	"█████╗  ██║   ██║██████╔╝██║  ███╗█████╗",
	"██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝",
	"██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗",
	"╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝",
}

var bannerFlame = []string{
	"   (",
	"  ) )",
	" ( ( (",
	"  \\ | /",
	"   \\|/",
	"   \\_|_/",
}

func Banner(w io.Writer, profile termenv.Profile) {
	renderer := lipgloss.NewRenderer(w)
	renderer.SetColorProfile(profile)

	wordmark := renderer.NewStyle().Bold(true).Foreground(PrimaryColor)
	warning := renderer.NewStyle().Foreground(WarningColor)
	errorStyle := renderer.NewStyle().Foreground(ErrorColor)

	for idx := range bannerWordmark {
		flameStyle := warning
		if idx >= len(bannerWordmark)/2 {
			flameStyle = errorStyle
		}

		fmt.Fprintln(w, flameStyle.Render(bannerFlame[idx])+"  "+wordmark.Render(bannerWordmark[idx]))
	}
}
