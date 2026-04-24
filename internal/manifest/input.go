package manifest

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/emkaytec/forge/internal/ui"
)

type inputModel struct {
	label        string
	labelWidth   int
	defaultValue string
	required     bool
	value        []rune
	confirmed    bool
	canceled     bool
	cancelError  error
	errMsg       string
}

func inputPrompt(p *promptSession, label, defaultValue string, required bool) (string, error) {
	p.runPrelude()

	if ui.IsInteractiveTerminal(p.in, p.out) {
		return runTerminalInput(p.out, label, p.labelWidth, defaultValue, required)
	}

	if required {
		return p.required(label, defaultValue)
	}
	return p.optional(label, defaultValue)
}

func runTerminalInput(out io.Writer, label string, labelWidth int, defaultValue string, required bool) (string, error) {
	model := inputModel{
		label:        label,
		labelWidth:   labelWidth,
		defaultValue: defaultValue,
		required:     required,
	}

	finalModel, err := tea.NewProgram(model, tea.WithOutput(out)).Run()
	if err != nil {
		return "", err
	}

	final := finalModel.(inputModel)
	if final.canceled {
		return "", final.cancelError
	}

	return string(final.value), nil
}

func (m inputModel) Init() tea.Cmd { return nil }

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.canceled = true
			m.cancelError = fmt.Errorf("prompt canceled")
			return m, tea.Quit
		case tea.KeyEnter:
			text := strings.TrimSpace(string(m.value))
			if text == "" {
				text = m.defaultValue
			}
			if m.required && text == "" {
				m.errMsg = "Value is required."
				return m, nil
			}
			m.value = []rune(text)
			m.confirmed = true
			return m, tea.Quit
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.value) > 0 {
				m.value = m.value[:len(m.value)-1]
				m.errMsg = ""
			}
		case tea.KeySpace:
			m.value = append(m.value, ' ')
			m.errMsg = ""
		case tea.KeyRunes:
			m.value = append(m.value, msg.Runes...)
			m.errMsg = ""
		}
	}
	return m, nil
}

func (m inputModel) View() string {
	if m.confirmed {
		return ui.RenderChip(m.label, string(m.value), m.labelWidth) + "\n"
	}

	var builder strings.Builder
	if m.defaultValue != "" {
		builder.WriteString(fmt.Sprintf("%s [%s]: %s", m.label, m.defaultValue, string(m.value)))
	} else {
		builder.WriteString(fmt.Sprintf("%s: %s", m.label, string(m.value)))
	}
	if m.errMsg != "" {
		builder.WriteString("\n" + ui.ErrorStyle.Render(m.errMsg))
	}
	return builder.String()
}
