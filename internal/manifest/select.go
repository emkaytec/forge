package manifest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/emkaytec/forge/internal/ui"
)

type selectOption = ui.SelectOption

func selectOnePrompt(p *promptSession, label string, options []selectOption, defaultIndex int) (selectOption, error) {
	p.runPrelude()

	if ui.IsInteractiveTerminal(p.in, p.out) {
		return ui.RunTerminalSelector(p.out, label, p.labelWidth, options, defaultIndex)
	}

	return p.selectOneByNumber(label, options, defaultIndex)
}

func selectManyPrompt(p *promptSession, label string, options []selectOption, defaultIndices []int) ([]selectOption, error) {
	p.runPrelude()

	if ui.IsInteractiveTerminal(p.in, p.out) {
		return ui.RunTerminalMultiSelector(p.out, label, p.labelWidth, options, defaultIndices)
	}

	return p.selectManyByNumber(label, options, defaultIndices)
}

func (p *promptSession) selectOneByNumber(label string, options []selectOption, defaultIndex int) (selectOption, error) {
	fmt.Fprintf(p.out, "%s:\n", label)
	for i, option := range options {
		fmt.Fprintf(p.out, "  %d. %s\n", i+1, option.Label)
	}

	for {
		choice, eof, err := p.line("Choose an option", strconv.Itoa(defaultIndex+1))
		if err != nil {
			return selectOption{}, err
		}

		index, err := strconv.Atoi(choice)
		if err != nil || index < 1 || index > len(options) {
			if eof {
				return selectOption{}, fmt.Errorf("selection canceled before option was chosen")
			}
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
		answer, eof, err := p.line("Choose one or more options (comma-separated)", strings.Join(defaultChoice, ","))
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

		if eof {
			return nil, fmt.Errorf("selection canceled before options were chosen")
		}
		fmt.Fprintf(p.out, "Enter one or more numbers between 1 and %d.\n", len(options))
	}
}
