# Tasks: build-cli-shell

Ordered so each item lands a verifiable, user-visible slice. Items within the same numeric group may be parallelized.

## 1. UI foundation (no behavior change yet)

- [x] 1.1 Add `internal/ui/palette.go` defining the primary, success, warning, error, and muted `lipgloss.Color` values.
- [x] 1.2 Add `internal/ui/styles.go` exporting `lipgloss.Style` values built from the palette (heading, muted, success, warning, error).
- [x] 1.3 Add `internal/ui/icons.go` exporting `IconSuccess` (`✓`), `IconError` (`✗`), `IconWarning` (`⚠`).
- [x] 1.4 Add `internal/ui/profile.go` that resolves the termenv profile from env and exposes a test hook to force `termenv.Ascii`.
- [x] 1.5 Add `internal/ui/writer.go` exporting `Success`, `Warn`, and `Error` functions that take an `io.Writer` and render prefixed, styled output using (1.2)–(1.4).
- [x] 1.6 Unit tests in `internal/ui` asserting that any non-empty `NO_COLOR` value strips ANSI from all writer outputs and that icons appear in the rendered bytes.
- [x] 1.7 Add `internal/ui/banner.go` that renders the `FORGE` wordmark plus flame motif to an `io.Writer` using the resolved termenv profile; primary color for the wordmark and warning→error gradient for the flame.
- [x] 1.8 Unit tests for the banner: color-capable render contains ANSI codes and the `FORGE` silhouette; color-disabled render contains the silhouette but no ANSI; total width ≤ 60 columns.

## 2. Cobra root command

- [x] 2.1 Add cobra, lipgloss, termenv, bubbletea, bubbles to `go.mod`; run `go mod tidy`.
- [x] 2.2 Create `internal/cli/root.go` defining the root cobra command, use `internal/ui` for help templates (command names bold, descriptions muted) and group the help output by `GroupID`.
- [x] 2.2.1 In the root command's default run (no args), render the banner from (1.7) followed by the styled help body to stdout.
- [x] 2.2.2 Ensure explicit help flows (`forge --help`, `forge help`) render styled help without the banner.
- [x] 2.3 Refactor `internal/cli/run.go` so `Run(args, stdout, stderr, version)` constructs the root command, sets `SetOut`/`SetErr`/`SetArgs`, and executes it.
- [x] 2.4 Update `cmd/forge/main.go` to pass a package-level `version` variable (default `"dev"`) into `cli.Run` and exit non-zero on error.

## 3. Flags and dispatch behavior

- [x] 3.1 Add a root-level `--version` / `-v` flag that prints the injected version string to stdout and returns nil (short-circuiting subcommand dispatch).
- [x] 3.2 Add persistent `--verbose` and `--debug` boolean flags on the root command (no consumers yet; surface only).
- [x] 3.3 Ensure unknown commands write styled help to **stderr** without the banner, emit cobra's suggestion message, and return an error whose message includes the unknown command token.

## 4. Async feedback primitive and demo commands

- [x] 4.1 Add `internal/ui/spinner.go` exposing a `Spinner` wrapper around `bubbles/spinner` + `bubbletea` that can be started and stopped from non-interactive code paths.
- [x] 4.2 Unit test that verifies the spinner wrapper runs and terminates cleanly under a non-TTY `io.Writer` without emitting ANSI escape sequences.
- [x] 4.3 Add `internal/cli/demo.go` registering a `demo` command group on the root with `GroupID: "demo"`.
- [x] 4.4 Implement `forge demo banner` that prints the brand banner once and returns nil.
- [x] 4.5 Implement `forge demo spinner` that runs the spinner for a short bounded duration (≈2s by default so the operator can see it, adjustable via a hidden `--duration` flag for tests) and then prints a styled success line.
- [x] 4.6 Behavior tests for the demo group: `forge demo banner` output contains the `FORGE` silhouette; `forge demo spinner` with a short duration terminates without error and ends with a success-icon line.

## 5. Behavior tests for the shell

- [x] 5.1 Update `internal/cli/run_test.go` to cover:
  - `forge` with no args prints the banner followed by the styled welcome/help screen to stdout and returns nil.
  - `forge --help` prints help to stdout and returns nil.
  - `forge --help` does not render the banner.
  - `forge --version` and `forge -v` print the injected version string and return nil.
  - `forge bogus` writes help to stderr without the banner, includes a suggestion when one exists, and returns an error containing `"bogus"`.
  - With any non-empty `NO_COLOR` value set, help output and banner bytes contain no ANSI escape sequences.
  - `forge --help` lists the `demo` command group.

## 6. Validation

- [x] 6.1 `go build ./...` succeeds.
- [x] 6.2 `go test ./...` passes.
- [x] 6.3 `openspec validate build-cli-shell --strict` passes.
