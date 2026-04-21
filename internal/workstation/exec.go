package workstation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type commandRunner interface {
	LookPath(file string) (string, error)
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	Run(ctx context.Context, workdir, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type execRunner struct{}

func (execRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, &commandError{
			Name:   name,
			Args:   args,
			Err:    err,
			Stderr: strings.TrimSpace(stderr.String()),
		}
	}

	return output, nil
}

func (execRunner) Run(ctx context.Context, workdir, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workdir
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return &commandError{Name: name, Args: args, Err: err}
	}

	return nil
}

type commandError struct {
	Name   string
	Args   []string
	Err    error
	Stderr string
}

func (e *commandError) Error() string {
	base := fmt.Sprintf("%s %s failed", e.Name, strings.Join(e.Args, " "))
	if e.Stderr != "" {
		return base + ": " + e.Stderr
	}
	if e.Err != nil {
		return base + ": " + e.Err.Error()
	}
	return base
}

func (e *commandError) Unwrap() error {
	return e.Err
}

type binaryUnavailableError struct {
	provider ProviderKind
	binary   string
	err      error
}

func (e *binaryUnavailableError) Error() string {
	return fmt.Sprintf("%s workstation support requires %s on PATH", e.provider, e.binary)
}

func (e *binaryUnavailableError) Unwrap() error {
	return e.err
}

func isBinaryUnavailable(err error) bool {
	var unavailable *binaryUnavailableError
	if errors.As(err, &unavailable) {
		return true
	}

	return errors.Is(err, exec.ErrNotFound)
}
