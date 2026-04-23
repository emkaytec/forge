package initcmd

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/emkaytec/forge/internal/aws/accounts"
	"github.com/emkaytec/forge/internal/ui"
)

type awsAccountTarget struct {
	ProfileName string
	AccountID   string
}

func (t awsAccountTarget) display() string {
	if t.ProfileName != "" && t.AccountID != "" {
		return fmt.Sprintf("%s (%s)", t.ProfileName, t.AccountID)
	}
	if t.ProfileName != "" {
		return t.ProfileName
	}
	return t.AccountID
}

type awsTargetPrompt struct {
	in     io.Reader
	out    io.Writer
	reader *bufio.Reader
}

func newAWSTargetPrompt(in io.Reader, out io.Writer) *awsTargetPrompt {
	return &awsTargetPrompt{
		in:     in,
		out:    out,
		reader: bufio.NewReader(in),
	}
}

func resolveAWSAccountTargets(in io.Reader, out io.Writer, accountProfiles, accountIDs []string) ([]awsAccountTarget, error) {
	accountProfiles = normalizeList(accountProfiles)
	accountIDs = normalizeList(accountIDs)

	profiles, err := accounts.LoadProfiles()
	if err != nil {
		return nil, err
	}

	targets := make([]awsAccountTarget, 0, len(accountProfiles)+len(accountIDs))
	for _, profileName := range accountProfiles {
		profile, ok := accounts.FindProfile(profiles, profileName)
		if !ok {
			return nil, fmt.Errorf("AWS profile %q was not found in local AWS config", profileName)
		}
		targets = append(targets, awsAccountTarget{
			ProfileName: profile.Name,
			AccountID:   profile.AccountID,
		})
	}

	for _, accountID := range accountIDs {
		targets = append(targets, awsAccountTarget{AccountID: accountID})
	}

	if len(targets) > 0 {
		return dedupeTargets(targets), nil
	}

	prompt := newAWSTargetPrompt(in, out)
	if len(profiles) == 0 {
		accountID, err := prompt.required("AWS account ID", "")
		if err != nil {
			return nil, err
		}
		return []awsAccountTarget{{AccountID: accountID}}, nil
	}

	orderedProfiles, defaultIndex := accounts.PrioritizeProfiles(profiles, "")
	options := make([]ui.SelectOption, 0, len(orderedProfiles)+1)
	for _, profile := range orderedProfiles {
		options = append(options, ui.SelectOption{
			Label: accounts.Label(profile),
			Value: profile.Name,
		})
	}
	options = append(options, ui.SelectOption{Label: "Enter an account ID manually", Value: "manual"})

	selected, err := prompt.selectMany("AWS accounts", options, []int{defaultIndex})
	if err != nil {
		return nil, err
	}

	for _, option := range selected {
		if option.Value == "manual" {
			accountID, err := prompt.required("AWS account ID", "")
			if err != nil {
				return nil, err
			}
			targets = append(targets, awsAccountTarget{AccountID: accountID})
			continue
		}

		profile, _ := accounts.FindProfile(orderedProfiles, option.Value)
		targets = append(targets, awsAccountTarget{
			ProfileName: profile.Name,
			AccountID:   profile.AccountID,
		})
	}

	return dedupeTargets(targets), nil
}

func normalizeList(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				normalized = append(normalized, trimmed)
			}
		}
	}
	return normalized
}

func dedupeTargets(targets []awsAccountTarget) []awsAccountTarget {
	seen := map[string]struct{}{}
	deduped := make([]awsAccountTarget, 0, len(targets))
	for _, target := range targets {
		key := target.ProfileName
		if key == "" {
			key = target.AccountID
		}
		if strings.TrimSpace(key) == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, target)
	}
	return deduped
}

func (p *awsTargetPrompt) selectMany(label string, options []ui.SelectOption, defaultIndices []int) ([]ui.SelectOption, error) {
	if ui.IsInteractiveTerminal(p.in, p.out) {
		return ui.RunTerminalMultiSelector(p.out, label, ui.ChipLabelWidth(label), options, defaultIndices)
	}

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
		selected := make([]ui.SelectOption, 0, len(parts))
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

func (p *awsTargetPrompt) required(label, defaultValue string) (string, error) {
	for {
		value, eof, err := p.line(label, defaultValue)
		if err != nil {
			return "", err
		}
		if value != "" {
			return value, nil
		}
		if eof {
			return "", fmt.Errorf("prompt canceled before %s was provided", strings.ToLower(label))
		}
		fmt.Fprintln(p.out, "Value is required.")
	}
}

func (p *awsTargetPrompt) line(label, defaultValue string) (string, bool, error) {
	if defaultValue != "" {
		fmt.Fprintf(p.out, "%s [%s]: ", label, defaultValue)
	} else {
		fmt.Fprintf(p.out, "%s: ", label)
	}

	line, err := p.reader.ReadString('\n')
	trimmed := strings.TrimSpace(line)
	if err != nil {
		if err == io.EOF {
			if trimmed != "" {
				return trimmed, true, nil
			}
			return defaultValue, true, nil
		}
		return "", false, err
	}

	if trimmed == "" {
		return defaultValue, false, nil
	}
	return trimmed, false, nil
}
