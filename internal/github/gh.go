package github

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// lookupGHToken returns the GitHub token the `gh` CLI would emit for the
// default host, or "" if gh is not installed, not authenticated, or the
// lookup otherwise fails. It is overridden in tests so the suite never
// shells out to the real gh binary.
var lookupGHToken = defaultLookupGHToken

func defaultLookupGHToken() string {
	if _, err := exec.LookPath("gh"); err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

// SetLookupGHTokenForTesting replaces the gh-token lookup for the lifetime
// of the returned restore func. It is intended for tests that exercise code
// paths where a real `gh` binary on PATH could leak into the test process.
func SetLookupGHTokenForTesting(fn func() string) (restore func()) {
	previous := lookupGHToken
	lookupGHToken = fn
	return func() { lookupGHToken = previous }
}
