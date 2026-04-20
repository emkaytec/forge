package ui

import (
	"io"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

type Spinner struct {
	message string
}

type stopSpinnerMsg struct{}

type spinnerModel struct {
	spinner  spinner.Model
	message  string
	duration time.Duration
	done     <-chan struct{}
}

func NewSpinner(message string) *Spinner {
	return &Spinner{message: message}
}

func (s *Spinner) Run(w io.Writer, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}

	if Profile() == termenv.Ascii || !isTerminalWriter(w) {
		time.Sleep(duration)
		return nil
	}

	model := spinnerModel{
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		message:  s.message,
		duration: duration,
	}

	program := tea.NewProgram(model, tea.WithOutput(w))
	_, err := program.Run()
	return err
}

// RunWhile renders the spinner while fn runs and returns fn's error as soon as
// fn returns. When the writer is not a TTY or the profile is ASCII, fn runs
// synchronously without a spinner so non-interactive output stays clean.
func (s *Spinner) RunWhile(w io.Writer, fn func() error) error {
	if fn == nil {
		return nil
	}

	if Profile() == termenv.Ascii || !isTerminalWriter(w) {
		return fn()
	}

	var fnErr error
	done := make(chan struct{})
	go func() {
		fnErr = fn()
		close(done)
	}()

	model := spinnerModel{
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		message: s.message,
		done:    done,
	}

	program := tea.NewProgram(model, tea.WithOutput(w))
	if _, err := program.Run(); err != nil {
		<-done
		return err
	}
	return fnErr
}

func (m spinnerModel) Init() tea.Cmd {
	if m.done != nil {
		return tea.Batch(m.spinner.Tick, waitForDone(m.done))
	}
	return tea.Batch(m.spinner.Tick, stopSpinnerAfter(m.duration))
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case stopSpinnerMsg:
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m spinnerModel) View() string {
	return m.spinner.View() + " " + m.message
}

func stopSpinnerAfter(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(time.Time) tea.Msg {
		return stopSpinnerMsg{}
	})
}

func waitForDone(done <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-done
		return stopSpinnerMsg{}
	}
}

func isTerminalWriter(w io.Writer) bool {
	file, ok := w.(interface{ Fd() uintptr })
	if !ok {
		return false
	}

	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}
