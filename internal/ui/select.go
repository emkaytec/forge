package ui

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

// SelectOption is one value rendered by the shared selector widget.
type SelectOption struct {
	Label string
	Value string
}

type selectorModel struct {
	title       string
	labelWidth  int
	options     []SelectOption
	cursor      int
	multi       bool
	selected    map[int]struct{}
	confirmed   bool
	canceled    bool
	cancelError error
}

// RunTerminalSelector renders an interactive single-select prompt.
func RunTerminalSelector(out io.Writer, label string, labelWidth int, options []SelectOption, defaultIndex int) (SelectOption, error) {
	model := selectorModel{
		title:      label,
		labelWidth: labelWidth,
		options:    options,
		cursor:     clampSelection(defaultIndex, len(options)),
		selected:   map[int]struct{}{},
	}

	finalModel, err := tea.NewProgram(model, tea.WithOutput(out)).Run()
	if err != nil {
		return SelectOption{}, err
	}

	selector := finalModel.(selectorModel)
	if selector.canceled {
		return SelectOption{}, selector.cancelError
	}

	return selector.options[selector.cursor], nil
}

// RunTerminalMultiSelector renders an interactive multi-select prompt.
func RunTerminalMultiSelector(out io.Writer, label string, labelWidth int, options []SelectOption, defaultIndices []int) ([]SelectOption, error) {
	selected := make(map[int]struct{}, len(defaultIndices))
	for _, index := range defaultIndices {
		if index >= 0 && index < len(options) {
			selected[index] = struct{}{}
		}
	}

	model := selectorModel{
		title:      label,
		labelWidth: labelWidth,
		options:    options,
		cursor:     0,
		multi:      true,
		selected:   selected,
	}

	finalModel, err := tea.NewProgram(model, tea.WithOutput(out)).Run()
	if err != nil {
		return nil, err
	}

	selector := finalModel.(selectorModel)
	if selector.canceled {
		return nil, selector.cancelError
	}

	values := make([]SelectOption, 0, len(selector.selected))
	for i, option := range selector.options {
		if _, ok := selector.selected[i]; ok {
			values = append(values, option)
		}
	}

	return values, nil
}

func (m selectorModel) Init() tea.Cmd { return nil }

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.canceled = true
			m.cancelError = fmt.Errorf("selection canceled")
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case " ":
			if m.multi {
				if _, ok := m.selected[m.cursor]; ok {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = struct{}{}
				}
			}
		case "enter":
			if m.multi && len(m.selected) == 0 {
				m.selected[m.cursor] = struct{}{}
			}
			m.confirmed = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m selectorModel) View() string {
	if m.confirmed {
		return RenderChip(m.title, m.confirmedSummary(), m.labelWidth) + "\n"
	}

	var builder strings.Builder
	builder.WriteString(m.title + "\n")
	if m.multi {
		builder.WriteString(MutedStyle.Render("Use ↑/↓ to move, space to toggle, Enter to confirm.") + "\n\n")
	} else {
		builder.WriteString(MutedStyle.Render("Use ↑/↓ to move and Enter to confirm.") + "\n\n")
	}

	for i, option := range m.options {
		focused := i == m.cursor
		cursor := "  "
		if focused {
			cursor = HeadingStyle.Render("❯ ")
		}

		label := option.Label
		if focused {
			label = BoldStyle.Render(option.Label)
		}

		if m.multi {
			marker := MutedStyle.Render("[ ]")
			if _, ok := m.selected[i]; ok {
				marker = PrimaryStyle.Render("[✓]")
			}
			builder.WriteString(fmt.Sprintf("%s%s %s\n", cursor, marker, label))
			continue
		}

		builder.WriteString(fmt.Sprintf("%s%s\n", cursor, label))
	}

	return strings.TrimRight(builder.String(), "\n")
}

func (m selectorModel) confirmedSummary() string {
	if !m.multi {
		if m.cursor >= 0 && m.cursor < len(m.options) {
			return m.options[m.cursor].Label
		}
		return ""
	}

	labels := make([]string, 0, len(m.selected))
	for i, option := range m.options {
		if _, ok := m.selected[i]; ok {
			labels = append(labels, option.Label)
		}
	}
	return strings.Join(labels, ", ")
}

// IsInteractiveTerminal reports whether both sides of a prompt are TTYs.
func IsInteractiveTerminal(in io.Reader, out io.Writer) bool {
	inFD, ok := in.(interface{ Fd() uintptr })
	if !ok {
		return false
	}
	outFD, ok := out.(interface{ Fd() uintptr })
	if !ok {
		return false
	}

	return (isatty.IsTerminal(inFD.Fd()) || isatty.IsCygwinTerminal(inFD.Fd())) &&
		(isatty.IsTerminal(outFD.Fd()) || isatty.IsCygwinTerminal(outFD.Fd()))
}

func clampSelection(index, count int) int {
	if count == 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= count {
		return count - 1
	}
	return index
}
