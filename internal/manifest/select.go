package manifest

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
)

type selectOption struct {
	Label string
	Value string
}

type selectorModel struct {
	title       string
	options     []selectOption
	cursor      int
	multi       bool
	selected    map[int]struct{}
	confirmed   bool
	canceled    bool
	cancelError error
}

func selectOnePrompt(p *promptSession, label string, options []selectOption, defaultIndex int) (selectOption, error) {
	if isInteractiveTerminal(p.in, p.out) {
		return runTerminalSelector(p.out, label, options, defaultIndex)
	}

	return p.selectOneByNumber(label, options, defaultIndex)
}

func selectManyPrompt(p *promptSession, label string, options []selectOption, defaultIndices []int) ([]selectOption, error) {
	if isInteractiveTerminal(p.in, p.out) {
		return runTerminalMultiSelector(p.out, label, options, defaultIndices)
	}

	return p.selectManyByNumber(label, options, defaultIndices)
}

func runTerminalSelector(out io.Writer, label string, options []selectOption, defaultIndex int) (selectOption, error) {
	model := selectorModel{
		title:    label,
		options:  options,
		cursor:   clampSelection(defaultIndex, len(options)),
		selected: map[int]struct{}{},
	}

	finalModel, err := tea.NewProgram(model, tea.WithOutput(out)).Run()
	if err != nil {
		return selectOption{}, err
	}

	selector := finalModel.(selectorModel)
	if selector.canceled {
		return selectOption{}, selector.cancelError
	}

	return selector.options[selector.cursor], nil
}

func runTerminalMultiSelector(out io.Writer, label string, options []selectOption, defaultIndices []int) ([]selectOption, error) {
	selected := make(map[int]struct{}, len(defaultIndices))
	for _, index := range defaultIndices {
		if index >= 0 && index < len(options) {
			selected[index] = struct{}{}
		}
	}

	model := selectorModel{
		title:    label,
		options:  options,
		cursor:   0,
		multi:    true,
		selected: selected,
	}

	finalModel, err := tea.NewProgram(model, tea.WithOutput(out)).Run()
	if err != nil {
		return nil, err
	}

	selector := finalModel.(selectorModel)
	if selector.canceled {
		return nil, selector.cancelError
	}

	values := make([]selectOption, 0, len(selector.selected))
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
	var builder strings.Builder
	builder.WriteString(m.title + "\n")
	if m.multi {
		builder.WriteString("Use ↑/↓ to move, space to toggle, Enter to confirm.\n\n")
	} else {
		builder.WriteString("Use ↑/↓ to move and Enter to confirm.\n\n")
	}

	for i, option := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		marker := "  "
		if m.multi {
			if _, ok := m.selected[i]; ok {
				marker = "[x]"
			} else {
				marker = "[ ]"
			}
			builder.WriteString(fmt.Sprintf("%s%s %s\n", cursor, marker, option.Label))
			continue
		}

		builder.WriteString(fmt.Sprintf("%s%s\n", cursor, option.Label))
	}

	return strings.TrimRight(builder.String(), "\n")
}

func isInteractiveTerminal(in io.Reader, out io.Writer) bool {
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

func (p *promptSession) selectOneByNumber(label string, options []selectOption, defaultIndex int) (selectOption, error) {
	fmt.Fprintf(p.out, "%s:\n", label)
	for i, option := range options {
		fmt.Fprintf(p.out, "  %d. %s\n", i+1, option.Label)
	}

	for {
		choice, err := p.line("Choose an option", strconv.Itoa(defaultIndex+1))
		if err != nil {
			return selectOption{}, err
		}

		index, err := strconv.Atoi(choice)
		if err != nil || index < 1 || index > len(options) {
			fmt.Fprintf(p.out, "Enter a number between 1 and %d.\n", len(options))
			continue
		}

		return options[index-1], nil
	}
}

func (p *promptSession) selectManyByNumber(label string, options []selectOption, defaultIndices []int) ([]selectOption, error) {
	fmt.Fprintf(p.out, "%s:\n", label)
	for i, option := range options {
		fmt.Fprintf(p.out, "  %d. %s\n", i+1, option.Label)
	}

	defaultChoice := make([]string, 0, len(defaultIndices))
	for _, index := range defaultIndices {
		defaultChoice = append(defaultChoice, strconv.Itoa(index+1))
	}

	for {
		answer, err := p.line("Choose one or more options (comma-separated)", strings.Join(defaultChoice, ","))
		if err != nil {
			return nil, err
		}

		parts := strings.Split(answer, ",")
		selected := make([]selectOption, 0, len(parts))
		seen := map[int]struct{}{}
		valid := true
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			index, err := strconv.Atoi(part)
			if err != nil || index < 1 || index > len(options) {
				valid = false
				break
			}
			if _, ok := seen[index-1]; ok {
				continue
			}
			seen[index-1] = struct{}{}
			selected = append(selected, options[index-1])
		}

		if valid && len(selected) > 0 {
			return selected, nil
		}

		fmt.Fprintf(p.out, "Enter one or more numbers between 1 and %d.\n", len(options))
	}
}
