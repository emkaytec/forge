package ui

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var forcedProfile *termenv.Profile

func Profile() termenv.Profile {
	if os.Getenv("NO_COLOR") != "" {
		return termenv.Ascii
	}

	if forcedProfile != nil {
		return *forcedProfile
	}

	return termenv.EnvColorProfile()
}

func SetProfileForTesting(profile termenv.Profile) func() {
	previous := forcedProfile
	forcedProfile = &profile

	return func() {
		forcedProfile = previous
	}
}

func rendererFor(w io.Writer) *lipgloss.Renderer {
	renderer := lipgloss.NewRenderer(w)
	renderer.SetColorProfile(Profile())
	return renderer
}
