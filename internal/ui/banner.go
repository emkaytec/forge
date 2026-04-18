package ui

import (
	"fmt"
	"io"
	"strings"

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

const (
	bannerLeftPad  = "  "
	bannerFlameGap = "  "
)

func Banner(w io.Writer, profile termenv.Profile) {
	renderer := lipgloss.NewRenderer(w)
	renderer.SetColorProfile(profile)

	wordmark := renderer.NewStyle().Bold(true).Foreground(PrimaryColor)
	warning := renderer.NewStyle().Foreground(WarningColor)
	errorStyle := renderer.NewStyle().Foreground(ErrorColor)

	flameWidth := 0
	for _, line := range bannerFlame {
		if w := lipgloss.Width(line); w > flameWidth {
			flameWidth = w
		}
	}

	fmt.Fprintln(w)
	for idx := range bannerWordmark {
		flameStyle := warning
		if idx >= len(bannerWordmark)/2 {
			flameStyle = errorStyle
		}

		flame := bannerFlame[idx]
		pad := strings.Repeat(" ", flameWidth-lipgloss.Width(flame))
		fmt.Fprintln(
			w,
			bannerLeftPad+flameStyle.Render(flame)+pad+bannerFlameGap+wordmark.Render(bannerWordmark[idx]),
		)
	}
	fmt.Fprintln(w)
}
