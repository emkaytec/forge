package launchagent

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// launchctl abstracts the launchctl(1) interface so tests can fake it.
type launchctl interface {
	// Bootstrap loads label from plistPath into the user's GUI domain.
	Bootstrap(ctx context.Context, domainTarget, plistPath string) error
	// Bootout unloads label from the user's GUI domain. Returning
	// nil on "service not loaded" is the implementation's responsibility.
	Bootout(ctx context.Context, serviceTarget string) error
}

// execLaunchctl is the production launchctl implementation, calling
// out to the `launchctl` binary. Tests inject fakes.
type execLaunchctl struct{}

func (execLaunchctl) Bootstrap(ctx context.Context, domainTarget, plistPath string) error {
	cmd := exec.CommandContext(ctx, "launchctl", "bootstrap", domainTarget, plistPath)
	return runCommand(cmd)
}

func (execLaunchctl) Bootout(ctx context.Context, serviceTarget string) error {
	cmd := exec.CommandContext(ctx, "launchctl", "bootout", serviceTarget)
	err := runCommand(cmd)
	if err == nil {
		return nil
	}

	// launchctl bootout returns non-zero when the service is not
	// loaded. Treat that as success so bootout → bootstrap stays
	// idempotent.
	if isServiceNotLoaded(err) {
		return nil
	}
	return err
}

func runCommand(cmd *exec.Cmd) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		trimmed := strings.TrimSpace(stderr.String())
		if trimmed != "" {
			return fmt.Errorf("%s: %w: %s", strings.Join(cmd.Args, " "), err, trimmed)
		}
		return fmt.Errorf("%s: %w", strings.Join(cmd.Args, " "), err)
	}

	return nil
}

// isServiceNotLoaded detects the launchctl exit conditions that mean
// "nothing to unload" so bootout can be idempotent.
func isServiceNotLoaded(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	if strings.Contains(msg, "No such process") {
		return true
	}
	if strings.Contains(msg, "Could not find specified service") {
		return true
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		// 113 = "Could not find specified service" in recent macOS.
		if exitErr.ExitCode() == 113 {
			return true
		}
	}
	return false
}
